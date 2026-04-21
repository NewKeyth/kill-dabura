package ports

import (
	"context"
	"os/exec"

	"dabura/internal/core/domain"
)

// Provider define el contrato que todo scraper debe implementar
type Provider interface {
	Name() string
	Language() string
	Search(ctx context.Context, query string) ([]domain.Movie, error)
	ExtractStreamURL(ctx context.Context, movie domain.Movie) ([]domain.StreamOption, error)
}

// Downloader define el contrato para extraer URLs o descargar
type Downloader interface {
	ExtractStreamURL(ctx context.Context, videoURL string) (string, error)
	GetDownloadCmd(ctx context.Context, videoURL, resolution, outputPath string) *exec.Cmd
}

// Player define el contrato para reproducir el video en el OS local
type Player interface {
	GetPlayCmd(ctx context.Context, streamURL string, forceBrowser bool) *exec.Cmd
}
