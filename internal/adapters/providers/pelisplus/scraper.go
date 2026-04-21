package pelisplus

import (
	"context"
	"encoding/base64"
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

func New() ports.Provider { return &Provider{} }

func (p *Provider) Name() string     { return "PelisPlus" }
func (p *Provider) Language() string  { return "Latino" }

func (p *Provider) Search(ctx context.Context, query string) ([]domain.Movie, error) {
	var movies []domain.Movie

	baseURL := "https://tioplus.app"
	searchURL := fmt.Sprintf("%s/search/%s", baseURL, url.PathEscape(query))

	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36"),
	)
	c.SetRequestTimeout(15 * time.Second)

	ch := make(chan error, 1)

	c.OnHTML("article.item a.itemA", func(e *colly.HTMLElement) {
		href := e.Attr("href")

		// Solo peliculas
		if !strings.Contains(href, "/pelicula/") {
			return
		}

		title := strings.TrimSpace(e.DOM.Find("h2").Text())
		if title == "" {
			return
		}
		// Limpiar año del titulo (formato "Title (2024)")
		title = strings.TrimSpace(strings.Split(title, " (")[0])

		movie := domain.Movie{
			Title:    title,
			Year:     "",
			Quality:  "HD",
			URL:      href,
			Language: p.Language(),
			Provider: p.Name(),
		}

		// Hacer la URL absoluta
		if !strings.HasPrefix(movie.URL, "http") {
			movie.URL = baseURL + movie.URL
		}

		movies = append(movies, movie)
	})

	go func() {
		ch <- c.Visit(searchURL)
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-ch:
		if err != nil {
			return nil, fmt.Errorf("error buscando en pelisplus: %w", err)
		}
	}

	if len(movies) > 30 {
		movies = movies[:30]
	}

	return movies, nil
}

func (p *Provider) ExtractStreamURL(ctx context.Context, movie domain.Movie) ([]domain.StreamOption, error) {
	var options []domain.StreamOption

	baseURL := "https://tioplus.app"

	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/115.0.0.0 Safari/537.36"),
	)
	c.SetRequestTimeout(15 * time.Second)

	// PelisPlus: servidores en .bg-tabs ul li con data-server (Base64 encoded)
	c.OnHTML(".bg-tabs ul li", func(e *colly.HTMLElement) {
		serverName := strings.TrimSpace(e.Text)
		serverName = strings.ReplaceAll(serverName, " Reproducir", "")
		dataServer := e.Attr("data-server")
		if dataServer == "" {
			return
		}

		// Intentar decodificar el Base64
		decoded, err := base64.StdEncoding.DecodeString(dataServer)
		if err != nil {
			return
		}

		decodedStr := string(decoded)

		var finalURL string
		if strings.Contains(decodedStr, "https://") {
			finalURL = decodedStr
		} else {
			// Si no tiene https, necesitamos ir al player intermediario
			playerURL := fmt.Sprintf("%s/player/%s", baseURL, dataServer)
			finalURL = resolvePlayerRedirect(playerURL)
		}

		if finalURL != "" {
			options = append(options, domain.StreamOption{
				Server:  serverName,
				Quality: "HD",
				URL:     finalURL,
			})
		}
	})

	err := c.Visit(movie.URL)
	if err != nil {
		return nil, fmt.Errorf("error extrayendo servers de pelisplus: %w", err)
	}

	if len(options) == 0 {
		return nil, fmt.Errorf("no servers found in pelisplus")
	}

	return options, nil
}

// resolvePlayerRedirect sigue la página intermedia del player de PelisPlus
// para extraer la URL de embed real.
func resolvePlayerRedirect(playerURL string) string {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/115.0.0.0 Safari/537.36"),
	)
	c.SetRequestTimeout(10 * time.Second)

	var result string
	c.OnResponse(func(r *colly.Response) {
		body := string(r.Body)
		// Buscar URL en el script window.onload
		re := regexp.MustCompile(`(https?://[^\s'"]+)`)
		matches := re.FindStringSubmatch(body)
		if len(matches) > 1 {
			result = matches[1]
		}
	})

	_ = c.Visit(playerURL)
	return result
}
