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

	"structa/internal/paths"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	// Resolve the on-disk cache path early so the asset server can serve thumbs.
	p, err := paths.Resolve()
	if err != nil {
		log.Fatalf("paths: %v", err)
	}

	err = wails.Run(&options.App{
		Title:  "Structa",
		Width:  1280,
		Height: 820,
		MinWidth: 900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: thumbsHandler(p.ThumbsDir),
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

// thumbsHandler serves /thumbs/<...> URLs from the on-disk thumbnail cache.
// For any other path it returns 404 so Wails can fall through to its 404 page.
func thumbsHandler(thumbsDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "/thumbs/"
		if !strings.HasPrefix(r.URL.Path, prefix) {
			http.NotFound(w, r)
			return
		}
		rel := strings.TrimPrefix(r.URL.Path, prefix)
		// Reject traversal attempts.
		if strings.Contains(rel, "..") {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		abs := filepath.Join(thumbsDir, filepath.FromSlash(rel))
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, abs)
	})
}
