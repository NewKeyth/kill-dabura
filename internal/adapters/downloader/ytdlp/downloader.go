package ytdlp

import (
	"context"
	"fmt"
	"os/exec"

	"dabura/internal/core/ports"
)

type YTDlpDownloader struct{}

func New() ports.Downloader {
	return &YTDlpDownloader{}
}

func (d *YTDlpDownloader) ExtractStreamURL(ctx context.Context, url string) (string, error) {
	cmd := exec.CommandContext(ctx, "yt-dlp", "--get-url", url)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error extrayendo URL: %v (salida: %s)", err, string(out))
	}
	return string(out), nil
}

func (d *YTDlpDownloader) GetDownloadCmd(ctx context.Context, url, resolution, outputPath string) *exec.Cmd {
	format := fmt.Sprintf("bestvideo[height<=%s]+bestaudio/best[height<=%s]/best", resolution, resolution)
	return exec.CommandContext(ctx, "yt-dlp", "--newline", "-f", format, "-o", outputPath, url)
}
