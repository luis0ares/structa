package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"structa/internal/config"
	appdb "structa/internal/db"
	"structa/internal/indexer"
	"structa/internal/paths"
	"structa/internal/watcher"
)

// App is the Wails application. Exported methods become callable from the React frontend.
type App struct {
	ctx     context.Context
	paths   paths.Paths
	db      *sql.DB
	indexer *indexer.Indexer
	watcher *watcher.Watcher
}

func NewApp() *App {
	return &App{}
}

// startup runs once when the Wails window is created.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	p, err := paths.Resolve()
	if err != nil {
		wailsruntime.LogErrorf(ctx, "paths: %v", err)
		return
	}
	a.paths = p

	d, err := appdb.Open(p.DBFile)
	if err != nil {
		wailsruntime.LogErrorf(ctx, "db open: %v", err)
		return
	}
	a.db = d

	cfg, err := config.Load(p.ConfigFile)
	if err != nil {
		wailsruntime.LogErrorf(ctx, "config load: %v", err)
		cfg = config.Config{Tabs: []config.Tab{}}
	}

	a.indexer = indexer.New(d, p, a)
	a.indexer.SetConfig(cfg)
	a.indexer.Start(ctx)

	w, err := watcher.New(a.indexer)
	if err != nil {
		wailsruntime.LogErrorf(ctx, "watcher: %v", err)
	} else {
		a.watcher = w
		a.watcher.SetRoots(a.indexer.CategoryPaths())
		go a.watcher.Run(ctx)
	}

	// Initial reconcile on a goroutine so the window opens immediately.
	go func() {
		if err := a.indexer.Reconcile(); err != nil {
			wailsruntime.LogErrorf(ctx, "reconcile: %v", err)
		}
	}()
}

// OnStatus implements indexer.Notifier — pushes status to the frontend.
func (a *App) OnStatus(s indexer.Status) {
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "indexer:status", s)
	}
}

// OnCatalogUpdated implements indexer.Notifier — tells the frontend to refetch.
func (a *App) OnCatalogUpdated() {
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "catalog:updated")
	}
}

// ----- DTOs returned to the frontend -----

type ItemCardDTO struct {
	ID          int64    `json:"id"`
	Title       string   `json:"title"`
	FolderPath  string   `json:"folderPath"`
	ThumbURL    string   `json:"thumbUrl"`
	Favorite    bool     `json:"favorite"`
	SourceLink  string   `json:"sourceLink"`
	Description string   `json:"description"`
	Content     []string `json:"content"`
	Tags        []string `json:"tags"`
}

type CategoryDTO struct {
	Name  string        `json:"name"`
	Items []ItemCardDTO `json:"items"`
}

type TabDTO struct {
	Name       string        `json:"name"`
	Categories []CategoryDTO `json:"categories"`
}

// ----- Bound methods -----

// GetCatalog returns the full catalog grouped by tab → category → items.
func (a *App) GetCatalog() ([]TabDTO, error) {
	if a.db == nil {
		return []TabDTO{}, nil
	}
	rows, err := appdb.AllCards(a.db)
	if err != nil {
		return nil, err
	}
	tabIdx := map[string]int{}
	catIdx := map[string]int{}
	out := []TabDTO{}

	// Seed tab order from current config so empty categories still render.
	cfg := a.indexer.Config()
	for _, t := range cfg.Tabs {
		tabIdx[t.Name] = len(out)
		td := TabDTO{Name: t.Name, Categories: []CategoryDTO{}}
		for _, c := range t.Categories {
			catKey := t.Name + "\x00" + c.Name
			catIdx[catKey] = len(td.Categories)
			td.Categories = append(td.Categories, CategoryDTO{Name: c.Name, Items: []ItemCardDTO{}})
		}
		out = append(out, td)
	}

	ensureTab := func(name string) int {
		if i, ok := tabIdx[name]; ok {
			return i
		}
		tabIdx[name] = len(out)
		out = append(out, TabDTO{Name: name, Categories: []CategoryDTO{}})
		return tabIdx[name]
	}
	ensureCat := func(tabName, catName string) (int, int) {
		ti := ensureTab(tabName)
		key := tabName + "\x00" + catName
		if ci, ok := catIdx[key]; ok {
			return ti, ci
		}
		catIdx[key] = len(out[ti].Categories)
		out[ti].Categories = append(out[ti].Categories, CategoryDTO{Name: catName, Items: []ItemCardDTO{}})
		return ti, catIdx[key]
	}

	for _, r := range rows {
		ti, ci := ensureCat(r.Tab, r.Category)
		var content, tags []string
		_ = json.Unmarshal([]byte(r.ContentJSON), &content)
		_ = json.Unmarshal([]byte(r.TagsJSON), &tags)
		card := ItemCardDTO{
			ID:          r.ID,
			Title:       r.Title,
			FolderPath:  r.FolderPath,
			Favorite:    r.Favorite,
			Description: r.Description,
			Content:     content,
			Tags:        tags,
		}
		if r.ThumbPath.Valid && r.ThumbPath.String != "" {
			card.ThumbURL = "/thumbs/" + r.ThumbPath.String
		}
		if r.SourceLink.Valid {
			card.SourceLink = r.SourceLink.String
		}
		out[ti].Categories[ci].Items = append(out[ti].Categories[ci].Items, card)
	}
	return out, nil
}

// GetPreviews returns the URLs of all preview images for an item.
func (a *App) GetPreviews(id int64) ([]string, error) {
	if a.db == nil {
		return []string{}, nil
	}
	card, err := appdb.GetCard(a.db, id)
	if err != nil || card == nil {
		return []string{}, err
	}
	var rels []string
	_ = json.Unmarshal([]byte(card.PreviewPaths), &rels)
	out := make([]string, 0, len(rels))
	for _, r := range rels {
		out = append(out, "/thumbs/"+r)
	}
	return out, nil
}

// ToggleFavorite flips the favorite flag in the DB and writes/removes the
// `.favorite` marker file on disk so it survives a fresh index.
func (a *App) ToggleFavorite(id int64) (bool, error) {
	if a.db == nil {
		return false, errors.New("db not ready")
	}
	card, err := appdb.GetCard(a.db, id)
	if err != nil || card == nil {
		return false, err
	}
	newFav := !card.Favorite
	if err := appdb.SetFavorite(a.db, id, newFav); err != nil {
		return false, err
	}
	marker := filepath.Join(card.FolderPath, ".favorite")
	if newFav {
		_ = os.WriteFile(marker, []byte{}, 0o644)
	} else {
		_ = os.Remove(marker)
	}
	return newFav, nil
}

// OpenFolder opens the item's folder in the system file manager.
func (a *App) OpenFolder(id int64) error {
	if a.db == nil {
		return errors.New("db not ready")
	}
	card, err := appdb.GetCard(a.db, id)
	if err != nil || card == nil {
		return err
	}
	return exec.Command("explorer", card.FolderPath).Start()
}

// OpenURL opens a URL in the system browser.
func (a *App) OpenURL(url string) error {
	if a.ctx == nil {
		return errors.New("ctx not ready")
	}
	wailsruntime.BrowserOpenURL(a.ctx, url)
	return nil
}

// GetConfig returns the current configuration.
func (a *App) GetConfig() (config.Config, error) {
	return config.Load(a.paths.ConfigFile)
}

// SaveConfig persists the configuration, refreshes the watcher and reconciles.
func (a *App) SaveConfig(cfg config.Config) error {
	if err := config.Save(a.paths.ConfigFile, cfg); err != nil {
		return err
	}
	a.indexer.SetConfig(cfg)
	if a.watcher != nil {
		a.watcher.SetRoots(a.indexer.CategoryPaths())
	}
	go func() {
		if err := a.indexer.Reconcile(); err != nil {
			wailsruntime.LogErrorf(a.ctx, "reconcile after save: %v", err)
		}
	}()
	return nil
}

// PickFolder shows a native directory picker and returns the chosen path.
func (a *App) PickFolder() (string, error) {
	if a.ctx == nil {
		return "", errors.New("ctx not ready")
	}
	dir, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Choose a folder",
	})
	if err != nil {
		return "", err
	}
	return filepath.Clean(dir), nil
}

// IndexerStatus returns the current indexer status.
func (a *App) IndexerStatus() indexer.Status {
	if a.indexer == nil {
		return indexer.Status{}
	}
	return a.indexer.Status()
}

// RescanAll forces a full reconcile (useful when a user wants to retrigger after debugging).
func (a *App) RescanAll() error {
	if a.indexer == nil {
		return errors.New("indexer not ready")
	}
	return a.indexer.Reconcile()
}

// ForceReindex reprocesses every folder regardless of content-hash. Use when
// per-item file conventions have changed (e.g. renaming tags.txt) or when the
// index appears stale.
func (a *App) ForceReindex() error {
	if a.indexer == nil {
		return errors.New("indexer not ready")
	}
	return a.indexer.ForceReconcile()
}

// helper used only for log lines; keeps the build dependency-free.
func describe(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", err))
}
