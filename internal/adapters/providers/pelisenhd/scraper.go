package pelisenhd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dabura/internal/core/domain"
	"dabura/internal/core/ports"

	"github.com/gocolly/colly/v2"
)

const BaseURL = "https://pelisenhd.org"

type Provider struct{}

func New() ports.Provider { return &Provider{} }

func (p *Provider) Name() string { return "PelisEnHD" }

func (p *Provider) Language() string { return "España" }

func (p *Provider) Search(ctx context.Context, query string) ([]domain.Movie, error) {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0"),
	)
	c.SetRequestTimeout(10 * time.Second)

	var movies []domain.Movie
	searchURL := fmt.Sprintf("%s/?s=%s", BaseURL, strings.ReplaceAll(query, " ", "+"))

	c.OnHTML("a", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		// Filter only movies
		if strings.Contains(link, "/pelicula-") {
			title := strings.TrimSpace(e.Text)
			if title != "" && title != "Ficha Técnica" && title != "Descargar" && title != "Imágenes" {
				
				duplicate := false
				for _, m := range movies {
					if m.URL == link {
						duplicate = true
						break
					}
				}
				if !duplicate {
					movies = append(movies, domain.Movie{
						Title:    title,
						Year:     "N/A",
						Language: p.Language(),
						URL:      link,
						Provider: p.Name(),
					})
				}
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

	c.OnHTML("a", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		if strings.Contains(link, "out.php?s=") || strings.Contains(link, "vip.hdpastes.com") {
			srv := "Enlace Público (Mega/1Fichier)"
			if strings.Contains(link, "vip.hdpastes") {
				srv = "Enlace VIP"
			}
			options = append(options, domain.StreamOption{
				Server:  srv,
				Quality: "Auto",
				URL:     link,
			})
		}
	})

	err := c.Visit(movie.URL)
	return options, err
}
