// Bundled yt-dlp and ffmpeg aren't committed (see tools/fetchytdlp and
// tools/fetchffmpeg) — they're fetched automatically before `wails dev`/
// `wails build` via wails.json's frontend:install. To fetch them
// standalone: go generate ./...
//go:generate go run ./tools/fetchytdlp
//go:generate go run ./tools/fetchffmpeg

package main

import (
	"context"
	"embed"

	"video-downloader-go/internal/manager"
	"video-downloader-go/internal/utils"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

// resolveYtdlpPath mirrors backend/utils.py's resource_path(): in a packaged
// build, yt-dlp ships next to the app executable; in development it lives in
// the project's bin/ directory. Falling back to the bare binary name lets it
// still work if the user has their own copy on PATH.
func resolveYtdlpPath() string {
	name := utils.YtdlpBinaryName()
	if path := utils.ResolveBundledPath(name); path != "" {
		return path
	}
	return name
}

func main() {
	mgr := manager.New(resolveYtdlpPath())
	app := NewApp(mgr)

	err := wails.Run(&options.App{
		Title:            "Video Downloader",
		Width:            680,
		Height:           760,
		MinWidth:         520,
		MinHeight:        480,
		Frameless:        true,
		BackgroundColour: options.NewRGB(18, 18, 18),
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: func(ctx context.Context) {
			mgr.SetEmitter(func(event string, data ...interface{}) {
				wailsruntime.EventsEmit(ctx, event, data...)
			})
			mgr.SetBrowseDirFunc(func() (string, error) {
				return wailsruntime.OpenDirectoryDialog(ctx, wailsruntime.OpenDialogOptions{
					Title: "Select Output Directory",
				})
			})
			mgr.CheckForUpdates()
		},
		OnShutdown: func(ctx context.Context) {
			mgr.Shutdown()
		},
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			Theme: windows.Dark,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
