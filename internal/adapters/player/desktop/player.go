package desktop

import (
	"context"
	"os/exec"
	"runtime"

	"dabura/internal/core/ports"
)

type DesktopPlayer struct{}

func New() ports.Player {
	return &DesktopPlayer{}
}

func (p *DesktopPlayer) GetPlayCmd(ctx context.Context, url string, forceBrowser bool) *exec.Cmd {
	if forceBrowser {
		return browserCmd(ctx, url)
	}

	if path, err := exec.LookPath("mpv"); err == nil {
		return exec.CommandContext(ctx, path, url)
	} else if path, err := exec.LookPath("vlc"); err == nil {
		return exec.CommandContext(ctx, path, url)
	}
	
	return browserCmd(ctx, url)
}

func browserCmd(ctx context.Context, url string) *exec.Cmd {
	switch runtime.GOOS {
	case "windows":
		return exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		return exec.CommandContext(ctx, "open", url)
	default:
		return exec.CommandContext(ctx, "xdg-open", url)
	}
}
