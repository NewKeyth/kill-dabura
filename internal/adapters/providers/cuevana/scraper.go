
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func (p *Provider) Language() string { return "Latino" }

func (p *Provider) Search(ctx context.Context, query string) ([]domain.Movie, error) {
	var movies []domain.Movie
	searchURL := fmt.Sprintf("%s/?s=%s", baseURL, url.QueryEscape(query))

	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/115.0.0.0 Safari/537.36"),
	)
	c.SetRequestTimeout(15 * time.Second)
	ch := make(chan error, 1)

	c.OnHTML("article.tooltip-content", func(e *colly.HTMLElement) {
		a := e.DOM.Find("h2 a").First()
		href, exists := a.Attr("href")
		if !exists {
			return
		}

		title := strings.TrimSpace(a.Text())
		if title != "" && (strings.Contains(href, "/pelicula/") || strings.Contains(href, "/serie/")) {
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
		}
	}

	return movies, nil
}

type RyusakiServer struct {
	ID         string `json:"id"`
	ServerName string `json:"serverName"`
	Language   string `json:"language"`
	Type       string `json:"type"`
	URL        string `json:"url"`
}

type RyusakiServersResponse struct {
	Success bool            `json:"success"`
	Servers []RyusakiServer `json:"servers"`
}

type RyusakiStreamResponse struct {
	Success   bool   `json:"success"`
	StreamUrl string `json:"streamUrl"`
}

func (p *Provider) ExtractStreamURL(ctx context.Context, movie domain.Movie) ([]domain.StreamOption, error) {
	var options []domain.StreamOption

	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/115.0.0.0 Safari/537.36"),
	)
	c.SetRequestTimeout(15 * time.Second)

	var nonce, tmdbId, contentType, season, episode string

	c.OnHTML("html", func(e *colly.HTMLElement) {
		html, _ := e.DOM.Html()
		re := regexp.MustCompile(`"nonce":"([^"]+)"`)
		matches := re.FindStringSubmatch(html)
		if len(matches) > 1 {
			nonce = matches[1]
		}
	})

	c.OnHTML("#player-wrapper", func(e *colly.HTMLElement) {
		tmdbId = e.Attr("data-id")
		contentType = e.Attr("data-type")
		season = e.Attr("data-season")
		episode = e.Attr("data-episode")
	})

	err := c.Visit(movie.URL)
	if err != nil {
	}

	if nonce == "" || tmdbId == "" {
		return nil, fmt.Errorf("no se encontraron tokens de acceso API ryusaki en la pelicula")
	}

	apiUrl := fmt.Sprintf("%s/wp-json/ryusaki-sync/v1/get-servers?type=%s&tmdb_id=%s", baseURL, contentType, tmdbId)
	if season != "" {
		apiUrl += "&season=" + season
	}
	if episode != "" {
		apiUrl += "&episode=" + episode
	}

	req, _ := http.NewRequest("GET", apiUrl, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/115.0.0.0 Safari/537.36")
	req.Header.Set("X-WP-Nonce", nonce)
	req.Header.Set("Referer", movie.URL)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var serversResp RyusakiServersResponse
	if err := json.Unmarshal(bodyBytes, &serversResp); err != nil {
		return nil, err
	}

	for _, srv := range serversResp.Servers {
		name := fmt.Sprintf("%s [%s]", strings.Title(srv.ServerName), strings.ToUpper(srv.Language))
		if strings.Contains(strings.ToLower(name), "download") {
			continue
		}

		if srv.Type == "embed" {
			// POST to get stream
			form := url.Values{}
			form.Add("tmdbId", tmdbId)
			form.Add("contentType", contentType)
			form.Add("serverId", srv.ID)
			if season != "" {
				form.Add("season", season)
			}
			if episode != "" {
				form.Add("episode", episode)
			}

			postReq, _ := http.NewRequest("POST", baseURL+"/wp-json/ryusaki-sync/v1/request-stream", strings.NewReader(form.Encode()))
			postReq.Header.Set("User-Agent", "Mozilla/5.0")
			postReq.Header.Set("X-WP-Nonce", nonce)
			postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			postResp, err := client.Do(postReq)
			if err == nil {
				postBody, _ := io.ReadAll(postResp.Body)
				postResp.Body.Close()

				var streamResp RyusakiStreamResponse
				if json.Unmarshal(postBody, &streamResp) == nil && streamResp.StreamUrl != "" {
					options = append(options, domain.StreamOption{
						Server:  name,
						Quality: "HD",
					})
				}
			}
		} else if srv.URL != "" {
			options = append(options, domain.StreamOption{
				Server:  name,
				Quality: "HD",
			})
		}
	}

	if len(options) == 0 {
	}
	return options, nil
}

	if strings.Contains(streamURL, "app.mysync.mov/stream/") {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(streamURL)
		if err == nil {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			html := string(bodyBytes)
			
			re := regexp.MustCompile(`window\.location\.replace\("([^"]+)"\)`)
			matches := re.FindStringSubmatch(html)
			if len(matches) > 1 {
				return matches[1]
			}
		}
	}
	return streamURL
}
