package main

import (
	"embed"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:    "Structa",
		Width:    1280,
		Height:   820,
		MinWidth: 900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: thumbsHandler(app),
		},
		BackgroundColour: &options.RGBA{R: 27, G: 27, B: 30, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		log.Printf("Error: %v", err)
	}
}

// thumbsHandler serves /thumbs/<...> from the active profile's thumbnail cache.
// The thumbs directory is resolved lazily via app.ThumbsDir() so it follows
// profile switches without restarting Wails.
func thumbsHandler(app *App) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "/thumbs/"
		if !strings.HasPrefix(r.URL.Path, prefix) {
			http.NotFound(w, r)
			return
		}
		thumbsDir := app.ThumbsDir()
		if thumbsDir == "" {
			http.NotFound(w, r)
			return
		}
		rel := strings.TrimPrefix(r.URL.Path, prefix)
		if strings.Contains(rel, "..") {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		abs := filepath.Join(thumbsDir, filepath.FromSlash(rel))
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, abs)
	})
}
