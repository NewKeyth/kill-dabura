package services

import (
	"context"
	"fmt"
	"os/exec"

	"dabura/internal/adapters/downloader/filemoon"
	"dabura/internal/adapters/downloader/voe"
	"dabura/internal/core/domain"
	"dabura/internal/core/ports"
)

// MovieService orquesta la lógica de negocio central
type MovieService struct {
	providers  []ports.Provider
	downloader ports.Downloader
	player     ports.Player
}

// NewMovieService inicializa el servicio con sus dependencias inyectadas
func NewMovieService(providers []ports.Provider, dl ports.Downloader, pl ports.Player) *MovieService {
	return &MovieService{
		providers:  providers,
		downloader: dl,
		player:     pl,
	}
}

// SearchProvider busca asíncronamente en un proveedor específico.
func (s *MovieService) SearchProvider(ctx context.Context, providerName, query string) ([]domain.Movie, error) {
	for _, p := range s.providers {
		if p.Name() == providerName {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			return p.Search(ctx, query)
		}
	}
	return nil, fmt.Errorf("provider %s not found", providerName)
}

// GetProvidersForLanguage devuelve el nombre de los proveedores disponibles para un Tab
func (s *MovieService) GetProvidersForLanguage(lang string) []string {
	var out []string
	for _, p := range s.providers {
		if p.Language() == lang {
			out = append(out, p.Name())
		}
	}
	if len(out) == 0 {
		out = append(out, "SoloLatino")
	}
	return out
}

// GetLanguages retorna la lista de lenguajes disponibles parseando los Providers
func (s *MovieService) GetLanguages() []string {
	langMap := make(map[string]bool)
	var out []string
	for _, p := range s.providers {
		l := p.Language()
		if !langMap[l] && l != "" {
			langMap[l] = true
			out = append(out, l)
		}
	}
	if len(out) == 0 {
		out = append(out, "Latino", "España", "Inglés", "Portugués")
	}
	return out
}

// ExtractStreamURLs delega en el provider para conseguir el abanico de enlaces de Servidores disponibles
func (s *MovieService) ExtractStreamURLs(ctx context.Context, movie domain.Movie) ([]domain.StreamOption, error) {
	var options []domain.StreamOption
	var err error

	for _, p := range s.providers {
		if p.Name() == movie.Provider {
			options, err = p.ExtractStreamURL(ctx, movie)
			break
		}
	}

	if err != nil || len(options) == 0 {
		// Fallback crudo al enlace principal
		options = append(options, domain.StreamOption{
			Server: "Direct",
			URL:    movie.URL,
		})
	}

	// 2. Interceptar Extractores Nativos ocultos en las URLs de los servidores (Voe, Filemoon, etc.)
	for i, opt := range options {
		if voe.IsVoeURL(opt.URL) {
			nativeURL, vErr := voe.Extract(opt.URL)
			if vErr == nil && nativeURL != "" {
				options[i].URL = nativeURL
				options[i].Server = "Voe Native"
			}
		} else if filemoon.IsFilemoonURL(opt.URL) {
			nativeURL, fErr := filemoon.Extract(opt.URL)
			if fErr == nil && nativeURL != "" {
				options[i].URL = nativeURL
				options[i].Server = "Filemoon Native"
			}
		}
	}

	return options, nil
}

// ResolveDirectURL resuelve el HLS post-selección del usuario vía yt-dlp final
func (s *MovieService) ResolveDirectURL(ctx context.Context, url string) (string, error) {
	finalURL, dlErr := s.downloader.ExtractStreamURL(ctx, url)
	if dlErr != nil || finalURL == "" {
		return url, nil
	}
	return finalURL, nil
}

// GetPlayCmd retorna el descriptor de ejecución OS para reproducir
func (s *MovieService) GetPlayCmd(ctx context.Context, streamURL string, forceBrowser bool) *exec.Cmd {
	return s.player.GetPlayCmd(ctx, streamURL, forceBrowser)
}

// GetDownloadCmd retorna el descriptor de ejecución OS para youtube-dl / yt-dlp
func (s *MovieService) GetDownloadCmd(ctx context.Context, streamURL, resolution, outputPath string) *exec.Cmd {
	return s.downloader.GetDownloadCmd(ctx, streamURL, resolution, outputPath)
}
