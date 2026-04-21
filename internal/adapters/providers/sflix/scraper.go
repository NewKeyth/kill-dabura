package sflix

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"dabura/internal/core/domain"
	"dabura/internal/core/ports"

	"github.com/gocolly/colly/v2"
)

const BaseURL = "https://sflix.to"

type Provider struct{}

func New() ports.Provider { return &Provider{} }

func (p *Provider) Name() string { return "SFlix" }

func (p *Provider) Language() string { return "Inglés" }

func (p *Provider) Search(ctx context.Context, query string) ([]domain.Movie, error) {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/129.0.0.0"),
	)
	c.SetRequestTimeout(10 * time.Second)

	var movies []domain.Movie
	searchURL := fmt.Sprintf("%s/search/%s", BaseURL, strings.ReplaceAll(query, " ", "-"))

	c.OnHTML("div.flw-item", func(e *colly.HTMLElement) {
		link := e.ChildAttr("a", "href")
		// Filter only movies
		if !strings.Contains(link, "/movie/") {
			return
		}

		title := e.ChildText("h2.film-name")
		year := ""
		e.ForEach("div.fd-infor > span", func(_ int, s *colly.HTMLElement) {
			txt := s.Text
			if len(txt) == 4 && txt[0] == '1' || txt[0] == '2' {
				year = txt
			}
		})

		movies = append(movies, domain.Movie{
			Title:    title,
			Year:     year,
			Language: p.Language(),
			URL:      BaseURL + link,
			Provider: p.Name(),
		})
	})

	err := c.Visit(searchURL)
	return movies, err
}

func (p *Provider) ExtractStreamURL(ctx context.Context, movie domain.Movie) ([]domain.StreamOption, error) {
	// ID is after the last '-'
	parts := strings.Split(movie.URL, "-")
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid movie URL format")
	}
	numericalId := parts[len(parts)-1]

	// 1. Get List of servers
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/129.0.0.0"),
	)
	var options []domain.StreamOption

	c.OnHTML("a[data-id]", func(e *colly.HTMLElement) {
		serverId := e.Attr("data-id")
		serverName := e.ChildText("span")
		if serverName == "" {
			serverName = "Server " + serverId
		}

		// 2. Fetch the source JSON
		sourceURL := fmt.Sprintf("%s/ajax/episode/sources/%s", BaseURL, serverId)
		req, _ := http.NewRequestWithContext(ctx, "GET", sourceURL, nil)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			defer resp.Body.Close()
			var data struct {
				Link string `json:"link"`
			}
			if json.NewDecoder(resp.Body).Decode(&data) == nil && data.Link != "" {
				options = append(options, domain.StreamOption{
					Server:  strings.TrimSpace(serverName),
					Quality: "Auto",
					URL:     data.Link,
				})
			}
		}
	})

	epListURL := fmt.Sprintf("%s/ajax/episode/list/%s", BaseURL, numericalId)
	if err := c.Visit(epListURL); err != nil {
		return nil, err
	}

	return options, nil
}
