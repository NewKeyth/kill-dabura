package sololatino

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"dabura/internal/core/domain"
	"dabura/internal/core/ports"

	"github.com/gocolly/colly/v2"
)

type Provider struct{}

func New() ports.Provider {
	return &Provider{}
}

func (s *Provider) Name() string {
	return "SoloLatino"
}

func (s *Provider) Language() string {
	return "Latino"
}

func (s *Provider) Search(ctx context.Context, query string) ([]domain.Movie, error) {
	var movies []domain.Movie

	searchURL := fmt.Sprintf("https://sololatino.net/buscar?q=%s", url.QueryEscape(query))
	queryLower := strings.ToLower(query)

	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/122.0.0.0 Safari/537.36"),
	)
	c.SetRequestTimeout(15 * time.Second)

	ch := make(chan error, 1)

	c.OnHTML("div.card", func(e *colly.HTMLElement) {
		title := strings.TrimSpace(e.DOM.Find(".card__title").Text())
		if title == "" {
			return
		}

		match := false
		tLower := strings.ToLower(title)
		for _, word := range strings.Fields(queryLower) {
			if len(word) > 2 && strings.Contains(tLower, word) {
				match = true
				break
			}
		}
		if !match && len(queryLower) > 2 {
			return
		}

		quality := strings.TrimSpace(e.DOM.Find(".badge").Text())
		ql := strings.ToLower(quality)
		if strings.Contains(ql, "serie") || strings.Contains(ql, "anime") || strings.Contains(ql, "tv") || strings.Contains(ql, "capitulo") {
			return // Omitimos series a petición del usuario
		}

		movie := domain.Movie{
			Title:    title,
			Year:     strings.TrimSpace(e.DOM.Find(".card__year").Text()),
			Rating:   strings.TrimSpace(strings.ReplaceAll(e.DOM.Find(".card__rating").Text(), "★", "")),
			Quality:  "HD", // Por petición del usuario, normalizamos o quitamos lo que diga pelicula/anime
			URL:      e.DOM.Find("a").AttrOr("href", ""),
			Language: s.Language(),
			Provider: s.Name(),
		}

		if movie.URL != "" && !strings.Contains(movie.URL, "javascript") {
			movies = append(movies, movie)
		}
	})

	go func() {
		ch <- c.Visit(searchURL)
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-ch:
		if err != nil {
			return nil, fmt.Errorf("error buscando en sololatino: %w", err)
		}
	}

	if len(movies) > 30 {
		movies = movies[:30]
	}

	return movies, nil
}

func (s *Provider) ExtractStreamURL(ctx context.Context, movie domain.Movie) ([]domain.StreamOption, error) {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/122.0.0.0 Safari/537.36"),
	)
	c.SetRequestTimeout(15 * time.Second)

	var htmlContent string
	c.OnResponse(func(r *colly.Response) {
		htmlContent = string(r.Body)
	})

	err := c.Visit(movie.URL)
	if err != nil || htmlContent == "" {
		return nil, err // Fallback is handled by orchestration
	}

	// 1. Encontrar iframe src (embed69) ya sea en un tag iframe o en un data-server-url
	reIframe := regexp.MustCompile(`(?i)(?:<iframe[^>]+src="|data-server-(?:url|iframe|src)=")([^"]+)`)
	matches := reIframe.FindAllStringSubmatch(htmlContent, -1)

	iframeURL := ""
	for _, m := range matches {
		if strings.Contains(m[1], "embed69.") || strings.Contains(m[1], "xupalace.") {
			iframeURL = m[1]
			break
		}
	}

	if iframeURL == "" {
		return nil, fmt.Errorf("no iframe found")
	}

	// 2. Navegar al iframe para obtener el dataLink de embed69
	embedHtml := ""
	c2 := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/122.0.0.0 Safari/537.36"),
	)
	c2.SetRequestTimeout(10 * time.Second)
	c2.OnResponse(func(r *colly.Response) {
		embedHtml = string(r.Body)
	})

	err = c2.Visit(iframeURL)
	if err != nil || embedHtml == "" {
		return nil, fmt.Errorf("could not load iframe")
	}

	// 3. Extraer el array JSON de `let dataLink = [...];`
	reData := regexp.MustCompile(`let dataLink\s*=\s*(\[.*?\]);`)
	matchData := reData.FindStringSubmatch(embedHtml)
	if len(matchData) < 2 {
		return nil, fmt.Errorf("no datalink found")
	}

	jsonStr := matchData[1]

	type JWTLink struct {
		ServerName string `json:"servername"`
		Link       string `json:"link"` // JWT Base64
	}
	type FileData struct {
		Language     string    `json:"video_language"`
		SortedEmbeds []JWTLink `json:"sortedEmbeds"`
	}

	var parsed []FileData
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return nil, err
	}

	var bestEmbeds []JWTLink
	for _, f := range parsed {
		if f.Language == "LAT" {
			bestEmbeds = f.SortedEmbeds
			break
		}
	}
	if len(bestEmbeds) == 0 && len(parsed) > 0 {
		bestEmbeds = parsed[0].SortedEmbeds
	}

	var options []domain.StreamOption

	// 4. Mapear TODOS los embeds en opciones reales
	for _, e := range bestEmbeds {
		// Decodificar Base64 del JWT
		parts := strings.Split(e.Link, ".")
		if len(parts) != 3 {
			continue
		}

		payloadRaw := parts[1]
		if m := len(payloadRaw) % 4; m != 0 {
			payloadRaw += strings.Repeat("=", 4-m)
		}

		payloadBytes, err := base64.URLEncoding.DecodeString(payloadRaw)
		if err != nil {
			continue
		}

		var payload struct {
			Link string `json:"link"`
		}
		if err := json.Unmarshal(payloadBytes, &payload); err == nil && payload.Link != "" {
			opts := domain.StreamOption{
				Server:  strings.ToUpper(e.ServerName),
				Quality: "HD",
				URL:     payload.Link,
			}
			options = append(options, opts)
		}
	}

	if len(options) == 0 {
		return nil, fmt.Errorf("no streams decoded")
	}

	return options, nil
}


