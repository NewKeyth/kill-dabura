package libreflix

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dabura/internal/core/domain"
	"dabura/internal/core/ports"

	"github.com/gocolly/colly/v2"
)

const BaseURL = "https://libreflix.org"

type Provider struct{}

func New() ports.Provider { return &Provider{} }

func (p *Provider) Name() string { return "Libreflix" }

func (p *Provider) Language() string { return "Portugués" }

func (p *Provider) Search(ctx context.Context, query string) ([]domain.Movie, error) {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0"),
	)
	c.SetRequestTimeout(10 * time.Second)

	var movies []domain.Movie
	searchURL := fmt.Sprintf("%s/busca/?q=%s", BaseURL, strings.ReplaceAll(query, " ", "+"))

	c.OnHTML("a", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		// In Libreflix, movies usually have /assistir/
		if strings.Contains(link, "/assistir/") {
			title := strings.TrimSpace(e.Text)
			if title != "" {
				movies = append(movies, domain.Movie{
					Title:    title,
					Year:     "N/A",
					Language: p.Language(),
					URL:      BaseURL + link,
					Provider: p.Name(),
				})
			}
		}
	})

	err := c.Visit(searchURL)
	return movies, err
}

func (p *Provider) ExtractStreamURL(ctx context.Context, movie domain.Movie) ([]domain.StreamOption, error) {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0"),
	)
	var options []domain.StreamOption

	c.OnHTML("video source", func(e *colly.HTMLElement) {
		src := e.Attr("src")
		if src != "" {
			if strings.HasPrefix(src, "/") {
				src = BaseURL + src
			}
			options = append(options, domain.StreamOption{
				Server:  "Libreflix CDN",
				Quality: "Auto",
				URL:     src,
			})
		}
	})

	c.OnHTML("iframe", func(e *colly.HTMLElement) {
		src := e.Attr("src")
		if src != "" && strings.Contains(src, "youtube") {
			options = append(options, domain.StreamOption{
				Server:  "YouTube",
				Quality: "Auto",
				URL:     src,
			})
		}
	})

	err := c.Visit(movie.URL)
	return options, err
}
