package cinecalidad

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"dabura/internal/core/domain"
	"dabura/internal/core/ports"

	"github.com/gocolly/colly/v2"
)

type Provider struct{}

func New() ports.Provider { return &Provider{} }

func (p *Provider) Name() string     { return "CineCalidad" }
func (p *Provider) Language() string  { return "Latino" }

func (p *Provider) Search(ctx context.Context, query string) ([]domain.Movie, error) {
	var movies []domain.Movie

	baseURL := "https://www.cinecalidad.ec"
	searchURL := fmt.Sprintf("%s/?s=%s", baseURL, url.QueryEscape(query))

	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/124.0.0.0 Safari/537.36"),
	)
	c.SetRequestTimeout(15 * time.Second)

	ch := make(chan error, 1)

	c.OnHTML("article.item", func(e *colly.HTMLElement) {
		a := e.DOM.Find("a").First()
		href, _ := a.Attr("href")

		// Solo peliculas: la URL debe contener /ver-pelicula/
		if !strings.Contains(href, "/ver-pelicula/") {
			return
		}

		img := e.DOM.Find("div.poster img")
		title, _ := img.Attr("alt")
		title = strings.TrimSpace(title)
		if title == "" {
			return
		}

		movie := domain.Movie{
			Title:    title,
			Year:     "",
			Quality:  "HD",
			URL:      href,
			Language: p.Language(),
			Provider: p.Name(),
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
			return nil, fmt.Errorf("error buscando en cinecalidad: %w", err)
		}
	}

	if len(movies) > 30 {
		movies = movies[:30]
	}

	return movies, nil
}

func (p *Provider) ExtractStreamURL(ctx context.Context, movie domain.Movie) ([]domain.StreamOption, error) {
	var options []domain.StreamOption

	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/124.0.0.0 Safari/537.36"),
	)
	c.SetRequestTimeout(15 * time.Second)

	// CineCalidad tiene los servidores en #playeroptionsul li[data-option]
	c.OnHTML("#playeroptionsul li[data-option]", func(e *colly.HTMLElement) {
		dataOption := e.Attr("data-option")
		serverName := strings.TrimSpace(e.Text)
		serverName = strings.ReplaceAll(serverName, " Reproducir", "")

		// Skip trailers
		classes, _ := e.DOM.Attr("class")
		if strings.Contains(classes, "trailer") {
			return
		}
		if strings.Contains(strings.ToLower(serverName), "trailer") {
			return
		}

		if dataOption != "" {
			options = append(options, domain.StreamOption{
				Server:  serverName,
				Quality: "HD",
				URL:     dataOption,
			})
		}
	})

	err := c.Visit(movie.URL)
	if err != nil {
		return nil, fmt.Errorf("error extrayendo servers de cinecalidad: %w", err)
	}

	if len(options) == 0 {
		return nil, fmt.Errorf("no servers found in cinecalidad")
	}

	return options, nil
}
