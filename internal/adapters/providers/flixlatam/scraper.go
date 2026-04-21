package flixlatam

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

func New() ports.Provider { return &Provider{} }

func (p *Provider) Name() string     { return "FlixLatam" }
func (p *Provider) Language() string { return "Latino" }

func (p *Provider) Search(ctx context.Context, query string) ([]domain.Movie, error) {
	var movies []domain.Movie
	baseURL := "https://flixlatam.com"
	searchURL := fmt.Sprintf("%s/search?s=%s", baseURL, url.QueryEscape(query))

	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/115.0.0.0 Safari/537.36"),
	)
	c.SetRequestTimeout(15 * time.Second)
	ch := make(chan error, 1)

	c.OnHTML("article.item, div.result-item article, .items article", func(e *colly.HTMLElement) {
		a := e.DOM.Find("a").First()
		href, exists := a.Attr("href")
		if !exists {
			return
		}

		title := e.DOM.Find("h3").First().Text()
		if title == "" {
			title = e.DOM.Find(".title").First().Text()
		}
		title = strings.TrimSpace(title)

		if title != "" && (strings.Contains(href, "/pelicula/") || strings.Contains(href, "/peliculas/")) {
			movieURL := href
			if !strings.HasPrefix(movieURL, "http") {
				movieURL = baseURL + movieURL
			}

			// Evitar duplicados
			duplicate := false
			for _, m := range movies {
				if m.URL == movieURL {
					duplicate = true
					break
				}
			}

			if !duplicate {
				movies = append(movies, domain.Movie{
					Title:    title,
					Year:     "N/A",
					Language: p.Language(),
					URL:      movieURL,
					Provider: p.Name(),
				})
			}
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
			return nil, fmt.Errorf("error buscando en flixlatam: %w", err)
		}
	}

	return movies, nil
}

func (p *Provider) ExtractStreamURL(ctx context.Context, movie domain.Movie) ([]domain.StreamOption, error) {
	var options []domain.StreamOption
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/115.0.0.0 Safari/537.36"),
	)
	c.SetRequestTimeout(15 * time.Second)

	// Array auxiliar para guardar las iframes de la pelicula
	var iframes []string

	c.OnHTML("div.pframe iframe", func(e *colly.HTMLElement) {
		src := e.Attr("src")
		if src != "" {
			iframes = append(iframes, src)
		}
	})

	err := c.Visit(movie.URL)
	if err != nil {
		return nil, fmt.Errorf("error accediendo a la pelicula en flixlatam: %w", err)
	}

	if len(iframes) == 0 {
		return nil, fmt.Errorf("no embeds found inside flixlatam player")
	}

	// Segundo scraper para resolver los src iframes embed
	c2 := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/115.0.0.0 Safari/537.36"),
	)
	c2.SetRequestTimeout(10 * time.Second)

	for _, embedURL := range iframes {
		c2.OnHTML("script", func(e *colly.HTMLElement) {
			text := e.Text
			if strings.Contains(text, "dataLink") {
				re := regexp.MustCompile(`dataLink\s*=\s*(\[.+?\]);`)
				matches := re.FindStringSubmatch(text)
				if len(matches) > 1 {
					jsonPayload := matches[1]
					
					var items []struct {
						VideoLanguage string `json:"video_language"`
						SortedEmbeds  []struct {
							Servername string `json:"servername"`
							Link       string `json:"link"`
						} `json:"sortedEmbeds"`
					}
					
					if err := json.Unmarshal([]byte(jsonPayload), &items); err == nil {
						for _, item := range items {
							for _, embed := range item.SortedEmbeds {
								if strings.EqualFold(embed.Servername, "download") {
									continue
								}
								urlParsed := decodeFlixLatamBase64(embed.Link)
								if urlParsed != "" {
									options = append(options, domain.StreamOption{
										Server:  fmt.Sprintf("%s [%s]", strings.Title(embed.Servername), item.VideoLanguage),
										Quality: "HD",
										URL:     urlParsed,
									})
								}
							}
						}
					}
				}
			}
		})

		// Caso go_to_playerVast
		c2.OnHTML(".ODDIV .OD_1 li", func(e *colly.HTMLElement) {
			onclick := e.Attr("onclick")
			if onclick != "" && strings.Contains(onclick, "go_to_playerVast") {
				re := regexp.MustCompile(`go_to_playerVast\(\s*'([^']+)'`)
				matches := re.FindStringSubmatch(onclick)
				if len(matches) > 1 {
					finalUrl := strings.TrimSpace(matches[1])
					serverName := strings.TrimSpace(e.DOM.Find("span").Text())
					if !strings.Contains(strings.ToLower(serverName), "download") {
						options = append(options, domain.StreamOption{
							Server:  serverName,
							Quality: "HD",
							URL:     finalUrl,
						})
					}
				}
			}
		})

		_ = c2.Visit(embedURL)
	}

	if len(options) == 0 {
		return nil, fmt.Errorf("no stream options found in flixlatam")
	}

	return options, nil
}

// decodeFlixLatamBase64 decodes JWT style base64 string
func decodeFlixLatamBase64(encryptedLink string) string {
	parts := strings.Split(encryptedLink, ".")
	if len(parts) != 3 {
		return ""
	}
	
	payloadB64 := parts[1]
	
	// Add padding
	missingPadding := len(payloadB64) % 4
	if missingPadding != 0 {
		payloadB64 += strings.Repeat("=", 4-missingPadding)
	}
	
	decoded, err := base64.URLEncoding.DecodeString(payloadB64)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(payloadB64)
		if err != nil {
			return ""
		}
	}
	
	payloadJson := string(decoded)
	
	// Fast extraction
	linkStart := strings.Index(payloadJson, `"link":"`)
	if linkStart == -1 {
		return ""
	}
	
	valueStart := linkStart + 8
	valueEnd := strings.Index(payloadJson[valueStart:], `"`)
	if valueEnd == -1 {
		return ""
	}
	
	return payloadJson[valueStart : valueStart+valueEnd]
}
