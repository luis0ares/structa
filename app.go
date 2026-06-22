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
	"sync"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"structa/internal/config"
	appdb "structa/internal/db"
	"structa/internal/indexer"
	"structa/internal/meta"
	"structa/internal/paths"
	"structa/internal/profiles"
	"structa/internal/watcher"
)

// App is the Wails application. Exported methods become callable from the React frontend.
type App struct {
	ctx     context.Context
	metaDir string // %AppData%/structa — where profiles.json lives

	// Active profile state — swapped atomically when switching profiles.
	profMu     sync.RWMutex
	paths      paths.Paths
	profCancel context.CancelFunc
	db         *sql.DB
	indexer    *indexer.Indexer
	watcher    *watcher.Watcher
}

func NewApp() *App {
	return &App{}
}

// ThumbsDir returns the thumbnail directory for the active profile.
// Called from the HTTP asset handler in main.go (different goroutine).
func (a *App) ThumbsDir() string {
	a.profMu.RLock()
	defer a.profMu.RUnlock()
	return a.paths.ThumbsDir
}

// startup runs once when the Wails window is ready.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	metaDir, err := paths.AppMetaDir()
	if err != nil {
		wailsruntime.LogErrorf(ctx, "appMetaDir: %v", err)
		return
	}
	a.metaDir = metaDir

	// Auto-activate the last-used profile so the frontend can skip the picker.
	reg, _ := profiles.Load(metaDir)
	if reg.Active != "" {
		if err := a.activateProfile(reg.Active, reg); err != nil {
			wailsruntime.LogErrorf(ctx, "auto-activate profile %q: %v", reg.Active, err)
		}
	}
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

// ----- Profile DTOs -----

type ProfileDTO struct {
	Name    string `json:"name"`
	DataDir string `json:"data_dir"`
}

// ----- Profile management (bound to frontend) -----

func (a *App) GetProfiles() ([]ProfileDTO, error) {
	reg, err := profiles.Load(a.metaDir)
	if err != nil {
		return nil, err
	}
	out := make([]ProfileDTO, len(reg.Profiles))
	for i, p := range reg.Profiles {
		out[i] = ProfileDTO{Name: p.Name, DataDir: p.DataDir}
	}
	return out, nil
}

func (a *App) GetActiveProfile() (string, error) {
	reg, err := profiles.Load(a.metaDir)
	if err != nil {
		return "", err
	}
	return reg.Active, nil
}

// DetectProfile inspects a directory chosen by the user and returns a best-guess
// ProfileDTO. If the directory is (or contains) a .structa folder whose
// config.json has a saved profile_name, that name is used; otherwise the
// directory's base name is used as the profile name.
func (a *App) DetectProfile(dataDir string) (ProfileDTO, error) {
	dataDir = filepath.Clean(dataDir)
	var configPath string
	if filepath.Base(dataDir) == ".structa" {
		configPath = filepath.Join(dataDir, "config.json")
	} else {
		configPath = filepath.Join(dataDir, ".structa", "config.json")
	}
	cfg, _ := config.Load(configPath)
	name := cfg.ProfileName
	if name == "" {
		base := filepath.Base(dataDir)
		if base == ".structa" {
			base = filepath.Base(filepath.Dir(dataDir))
		}
		name = base
	}
	return ProfileDTO{Name: name, DataDir: dataDir}, nil
}

// CreateProfile registers a new named profile. dataDir is the folder chosen by
// the user; a .structa/ subdirectory will be created inside it on first use
// (unless dataDir is already a .structa folder, in which case it is used directly).
func (a *App) CreateProfile(name string, dataDir string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("profile name cannot be empty")
	}
	if dataDir == "" {
		return errors.New("data directory cannot be empty")
	}
	reg, err := profiles.Load(a.metaDir)
	if err != nil {
		return err
	}
	for _, p := range reg.Profiles {
		if p.Name == name {
			return fmt.Errorf("a profile named %q already exists", name)
		}
	}
	reg.Profiles = append(reg.Profiles, profiles.Profile{Name: name, DataDir: dataDir})
	if err := profiles.Save(a.metaDir, reg); err != nil {
		return err
	}
	// Persist the profile name into config.json so future imports can auto-detect it.
	if pp, err := paths.ResolveProfile(dataDir); err == nil {
		cfg, _ := config.Load(pp.ConfigFile)
		if cfg.ProfileName == "" {
			cfg.ProfileName = name
			_ = config.Save(pp.ConfigFile, cfg)
		}
	}
	return nil
}

// SelectProfile activates an existing profile: initialises its DB, indexer,
// and watcher, persists it as the active profile, then emits "profile:ready".
func (a *App) SelectProfile(name string) error {
	reg, err := profiles.Load(a.metaDir)
	if err != nil {
		return err
	}
	if err := a.activateProfile(name, reg); err != nil {
		return err
	}
	reg.Active = name
	if err := profiles.Save(a.metaDir, reg); err != nil {
		wailsruntime.LogErrorf(a.ctx, "save profiles: %v", err)
	}
	wailsruntime.EventsEmit(a.ctx, "profile:ready", name)
	return nil
}

// DeleteProfile removes a profile from the registry. Its data folder is NOT deleted.
func (a *App) DeleteProfile(name string) error {
	reg, err := profiles.Load(a.metaDir)
	if err != nil {
		return err
	}
	filtered := reg.Profiles[:0]
	for _, p := range reg.Profiles {
		if p.Name != name {
			filtered = append(filtered, p)
		}
	}
	reg.Profiles = filtered
	if reg.Active == name {
		reg.Active = ""
	}
	return profiles.Save(a.metaDir, reg)
}

// UpdateProfile renames an existing profile and/or changes its data directory.
func (a *App) UpdateProfile(oldName, newName, newDataDir string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return errors.New("name cannot be empty")
	}
	reg, err := profiles.Load(a.metaDir)
	if err != nil {
		return err
	}
	found := false
	for i, p := range reg.Profiles {
		if p.Name == newName && newName != oldName {
			_ = p
			return fmt.Errorf("a profile named %q already exists", newName)
		}
		if p.Name == oldName {
			reg.Profiles[i].Name = newName
			if newDataDir != "" {
				reg.Profiles[i].DataDir = newDataDir
			}
			found = true
		}
	}
	if !found {
		return fmt.Errorf("profile %q not found", oldName)
	}
	if reg.Active == oldName {
		reg.Active = newName
	}
	return profiles.Save(a.metaDir, reg)
}

// activateProfile initialises all subsystems for the named profile.
// Caller does not need to hold any lock.
func (a *App) activateProfile(name string, reg profiles.Registry) error {
	var found *profiles.Profile
	for i := range reg.Profiles {
		if reg.Profiles[i].Name == name {
			found = &reg.Profiles[i]
			break
		}
	}
	if found == nil {
		return fmt.Errorf("profile %q not found", name)
	}

	pp, err := paths.ResolveProfile(found.DataDir)
	if err != nil {
		return fmt.Errorf("resolve paths: %w", err)
	}

	d, err := appdb.Open(pp.DBFile)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}

	cfg, err := config.Load(pp.ConfigFile)
	if err != nil {
		cfg = config.Config{Tabs: []config.Tab{}}
	}

	profCtx, cancel := context.WithCancel(a.ctx)

	ix := indexer.New(d, pp, a)
	ix.SetConfig(cfg)
	ix.Start(profCtx)

	w, werr := watcher.New(ix)

	// Swap in new profile atomically.
	a.profMu.Lock()
	a.teardownLocked()
	a.paths = pp
	a.db = d
	a.profCancel = cancel
	a.indexer = ix
	if werr == nil {
		a.watcher = w
		w.SetRoots(ix.CategoryPaths())
		go w.Run(profCtx)
	}
	a.profMu.Unlock()

	go func() {
		if err := ix.Reconcile(); err != nil {
			wailsruntime.LogErrorf(a.ctx, "reconcile: %v", err)
		}
	}()

	return nil
}

// teardownLocked stops the current profile's subsystems. Must be called with profMu held.
func (a *App) teardownLocked() {
	if a.profCancel != nil {
		a.profCancel()
		a.profCancel = nil
	}
	if a.db != nil {
		_ = a.db.Close()
		a.db = nil
	}
	a.indexer = nil
	a.watcher = nil
	a.paths = paths.Paths{}
}

// ----- DTOs returned to the frontend -----

type ItemCardDTO struct {
	ID          int64    `json:"id"`
	Title       string   `json:"title"`
	FolderName  string   `json:"folderName"`
	FolderPath  string   `json:"folderPath"`
	ThumbURL    string   `json:"thumbUrl"`
	Favorite    bool     `json:"favorite"`
	Hidden      bool     `json:"hidden"`
	SourceLink  string   `json:"sourceLink"`
	Description string   `json:"description"`
	Content     []string `json:"content"`
	Tags        []string `json:"tags"`
	MTime       float64  `json:"mtime"`
	CTime       float64  `json:"ctime"`
}

type CategoryDTO struct {
	Name  string        `json:"name"`
	Items []ItemCardDTO `json:"items"`
}

type TabDTO struct {
	Name       string        `json:"name"`
	Categories []CategoryDTO `json:"categories"`
}

// MoveDestDTO describes one valid destination for a bulk move.
type MoveDestDTO struct {
	Tab      string `json:"tab"`
	Category string `json:"category"`
	Path     string `json:"path"`
	Label    string `json:"label"`
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
			FolderName:  r.FolderName,
			FolderPath:  r.FolderPath,
			Favorite:    r.Favorite,
			Hidden:      r.Hidden,
			Description: r.Description,
			Content:     content,
			Tags:        tags,
			MTime:       r.MTime,
			CTime:       r.CTime,
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

// ToggleFavorite flips the favorite flag in the DB and persists it to structa.yaml.
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
	m, hasYAML, _ := meta.Read(card.FolderPath)
	if !hasYAML {
		m.Name = card.Title
		if m.Name == card.FolderName {
			m.Name = ""
		}
		var tags []string
		_ = json.Unmarshal([]byte(card.TagsJSON), &tags)
		m.Tags = tags
		m.Description = card.Description
		if card.SourceLink.Valid {
			m.Link = card.SourceLink.String
		}
	}
	m.Favorite = newFav
	m.Hidden = card.Hidden
	_ = meta.Write(card.FolderPath, m)
	return newFav, nil
}

// UpdateItemMeta saves custom metadata (name, tags, description, link, favorite, hidden) to structa.yaml.
func (a *App) UpdateItemMeta(id int64, name string, tags []string, description string, link string, favorite bool, hidden bool) error {
	if a.db == nil {
		return errors.New("db not ready")
	}
	card, err := appdb.GetCard(a.db, id)
	if err != nil || card == nil {
		return fmt.Errorf("card not found: %w", err)
	}
	yamlName := name
	if yamlName == card.FolderName {
		yamlName = ""
	}
	m := meta.ItemMeta{
		Name:        yamlName,
		Tags:        tags,
		Description: description,
		Link:        link,
		Favorite:    favorite,
		Hidden:      hidden,
	}
	if err := meta.Write(card.FolderPath, m); err != nil {
		return err
	}
	displayTitle := name
	if displayTitle == "" {
		displayTitle = card.FolderName
	}
	tagsJSON, _ := json.Marshal(tags)
	sourceLink := sql.NullString{}
	if link != "" {
		sourceLink = sql.NullString{String: link, Valid: true}
	}
	_ = appdb.UpsertDetails(a.db, appdb.DetailsRow{
		FolderID:     card.ID,
		Title:        displayTitle,
		Favorite:     favorite,
		Hidden:       hidden,
		SourceLink:   sourceLink,
		ContentJSON:  card.ContentJSON,
		ThumbPath:    card.ThumbPath,
		PreviewPaths: card.PreviewPaths,
		TagsJSON:     string(tagsJSON),
		Description:  description,
	})
	a.OnCatalogUpdated()
	return nil
}

// ListMoveDestinations returns one entry per configured category root path.
func (a *App) ListMoveDestinations() ([]MoveDestDTO, error) {
	if a.indexer == nil {
		return []MoveDestDTO{}, nil
	}
	cfg := a.indexer.Config()
	out := []MoveDestDTO{}
	seen := map[string]bool{}
	for _, t := range cfg.Tabs {
		for _, c := range t.Categories {
			for _, p := range c.Folders {
				clean := filepath.Clean(p)
				if seen[clean] {
					continue
				}
				seen[clean] = true
				out = append(out, MoveDestDTO{
					Tab:      t.Name,
					Category: c.Name,
					Path:     clean,
					Label:    fmt.Sprintf("%s / %s — %s", t.Name, c.Name, clean),
				})
			}
		}
	}
	return out, nil
}

// MoveItems moves each item folder into destDir.
func (a *App) MoveItems(ids []int64, destDir string) error {
	if a.db == nil {
		return errors.New("db not ready")
	}
	if len(ids) == 0 {
		return nil
	}
	destClean := filepath.Clean(destDir)
	valid, err := a.ListMoveDestinations()
	if err != nil {
		return err
	}
	allowed := false
	for _, d := range valid {
		if d.Path == destClean {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("destination not in configured category roots: %s", destDir)
	}
	if err := os.MkdirAll(destClean, 0o755); err != nil {
		return err
	}
	var firstErr error
	for _, id := range ids {
		card, err := appdb.GetCard(a.db, id)
		if err != nil || card == nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("item %d: not found", id)
			}
			continue
		}
		src := card.FolderPath
		if filepath.Clean(filepath.Dir(src)) == destClean {
			continue
		}
		dst := filepath.Join(destClean, card.FolderName)
		if _, err := os.Stat(dst); err == nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("item %d: destination already exists: %s", id, dst)
			}
			continue
		}
		if err := os.Rename(src, dst); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("item %d: %w", id, err)
			}
			continue
		}
		_, _ = appdb.DeleteByPath(a.db, src)
	}
	if a.indexer != nil {
		go func() {
			if err := a.indexer.Reconcile(); err != nil {
				wailsruntime.LogErrorf(a.ctx, "reconcile after move: %v", err)
			}
		}()
	}
	a.OnCatalogUpdated()
	return firstErr
}

// DeleteItems removes each item folder from disk and purges its DB row.
func (a *App) DeleteItems(ids []int64) error {
	if a.db == nil {
		return errors.New("db not ready")
	}
	if len(ids) == 0 {
		return nil
	}
	var firstErr error
	for _, id := range ids {
		card, err := appdb.GetCard(a.db, id)
		if err != nil || card == nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("item %d: not found", id)
			}
			continue
		}
		if err := os.RemoveAll(card.FolderPath); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("item %d: %w", id, err)
			}
			continue
		}
		_, _ = appdb.DeleteByPath(a.db, card.FolderPath)
	}
	if a.indexer != nil {
		go func() {
			if err := a.indexer.Reconcile(); err != nil {
				wailsruntime.LogErrorf(a.ctx, "reconcile after delete: %v", err)
			}
		}()
	}
	a.OnCatalogUpdated()
	return firstErr
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
	if a.paths.ConfigFile == "" {
		return config.Config{Tabs: []config.Tab{}}, nil
	}
	return config.Load(a.paths.ConfigFile)
}

// SaveConfig persists the configuration, refreshes the watcher and reconciles.
func (a *App) SaveConfig(cfg config.Config) error {
	if a.paths.ConfigFile == "" {
		return errors.New("no active profile")
	}
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
	if dir == "" {
		return "", nil
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

// RescanAll forces a full reconcile.
func (a *App) RescanAll() error {
	if a.indexer == nil {
		return errors.New("indexer not ready")
	}
	return a.indexer.Reconcile()
}

// ForceReindex reprocesses every folder regardless of content-hash.
func (a *App) ForceReindex() error {
	if a.indexer == nil {
		return errors.New("indexer not ready")
	}
	return a.indexer.ForceReconcile()
}

func describe(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", err))
}
