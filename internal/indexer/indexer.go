package indexer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"structa/internal/config"
	appdb "structa/internal/db"
	"structa/internal/paths"
)

// Status describes what the indexer is currently doing. Mirrored to the frontend.
type Status struct {
	Scanning    bool   `json:"scanning"`
	CurrentPath string `json:"currentPath"`
	QueueDepth  int    `json:"queueDepth"`
}

// Notifier receives indexer lifecycle events. The Wails App implements this.
type Notifier interface {
	OnStatus(s Status)
	OnCatalogUpdated()
}

// Indexer reconciles disk state with the catalog DB and watches for live changes.
type Indexer struct {
	db       *sql.DB
	paths    paths.Paths
	notifier Notifier

	cfgMu sync.RWMutex
	cfg   config.Config

	jobs chan job

	statusMu sync.Mutex
	status   Status

	dirty chan struct{} // signals: a batch finished, fire catalog:updated debounced
}

type job struct {
	tab          string
	category     string
	categoryPath string
	folderPath   string
}

func New(d *sql.DB, p paths.Paths, n Notifier) *Indexer {
	return &Indexer{
		db:       d,
		paths:    p,
		notifier: n,
		jobs:     make(chan job, 1024),
		dirty:    make(chan struct{}, 1),
	}
}

func (ix *Indexer) SetConfig(c config.Config) {
	ix.cfgMu.Lock()
	ix.cfg = c
	ix.cfgMu.Unlock()
}

func (ix *Indexer) Config() config.Config {
	ix.cfgMu.RLock()
	defer ix.cfgMu.RUnlock()
	return ix.cfg
}

// CategoryPaths returns the unique disk paths that should be watched.
func (ix *Indexer) CategoryPaths() []string {
	ix.cfgMu.RLock()
	defer ix.cfgMu.RUnlock()
	seen := map[string]struct{}{}
	out := []string{}
	for _, t := range ix.cfg.Tabs {
		for _, c := range t.Categories {
			for _, f := range c.Folders {
				cf := filepath.Clean(f)
				if _, ok := seen[cf]; ok {
					continue
				}
				seen[cf] = struct{}{}
				out = append(out, cf)
			}
		}
	}
	return out
}

func (ix *Indexer) Status() Status {
	ix.statusMu.Lock()
	defer ix.statusMu.Unlock()
	return ix.status
}

func (ix *Indexer) setStatus(s Status) {
	ix.statusMu.Lock()
	ix.status = s
	ix.statusMu.Unlock()
	if ix.notifier != nil {
		ix.notifier.OnStatus(s)
	}
}

// Start launches the worker pool and the dirty-debounce goroutine.
// Call Reconcile separately to enqueue work.
func (ix *Indexer) Start(ctx context.Context) {
	workers := runtime.NumCPU()
	if workers < 2 {
		workers = 2
	}
	for i := 0; i < workers; i++ {
		go ix.worker(ctx)
	}
	go ix.dirtyLoop(ctx)
}

func (ix *Indexer) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case j, ok := <-ix.jobs:
			if !ok {
				return
			}
			ix.setStatus(Status{Scanning: true, CurrentPath: j.folderPath, QueueDepth: len(ix.jobs)})
			if err := processFolder(ix.db, ix.paths, j.tab, j.category, j.categoryPath, j.folderPath); err != nil {
				log.Printf("indexer: process %s: %v", j.folderPath, err)
			}
			ix.markDirty()
			if len(ix.jobs) == 0 {
				ix.setStatus(Status{Scanning: false, QueueDepth: 0})
			}
		}
	}
}

func (ix *Indexer) markDirty() {
	select {
	case ix.dirty <- struct{}{}:
	default:
	}
}

func (ix *Indexer) dirtyLoop(ctx context.Context) {
	var timer *time.Timer
	fire := func() {
		if ix.notifier != nil {
			ix.notifier.OnCatalogUpdated()
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ix.dirty:
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(500*time.Millisecond, fire)
		}
	}
}

// Reconcile walks every configured category, enqueues changed item folders,
// and prunes DB rows whose folder_path no longer exists on disk.
func (ix *Indexer) Reconcile() error { return ix.reconcile(false) }

// ForceReconcile reprocesses every folder regardless of content-hash, useful
// after changing per-item file conventions (e.g. tags.txt) or recovering from
// a corrupted index.
func (ix *Indexer) ForceReconcile() error { return ix.reconcile(true) }

func (ix *Indexer) reconcile(force bool) error {
	cfg := ix.Config()

	// Build the desired set: every direct subdirectory of every configured folder path.
	type want struct {
		tab          string
		category     string
		categoryPath string
		folderPath   string
	}
	desired := map[string]want{}

	for _, tab := range cfg.Tabs {
		for _, cat := range tab.Categories {
			for _, catPath := range cat.Folders {
				cp := filepath.Clean(catPath)
				entries, err := os.ReadDir(cp)
				if err != nil {
					if !errors.Is(err, os.ErrNotExist) {
						log.Printf("indexer: read %s: %v", cp, err)
					}
					continue
				}
				for _, e := range entries {
					if !e.IsDir() {
						continue
					}
					if strings.Contains(strings.ToLower(e.Name()), ".ignore") {
						continue
					}
					fp := filepath.Join(cp, e.Name())
					desired[fp] = want{tab.Name, cat.Name, cp, fp}
				}
			}
		}
	}

	// Decide which need (re)processing.
	for fp, w := range desired {
		if force {
			ix.jobs <- job{w.tab, w.category, w.categoryPath, w.folderPath}
			continue
		}
		existing, err := appdb.GetByPath(ix.db, fp)
		if err != nil {
			log.Printf("indexer: lookup %s: %v", fp, err)
			continue
		}
		needsWork := existing == nil ||
			existing.Tab != w.tab ||
			existing.Category != w.category ||
			existing.CategoryPath != w.categoryPath
		if !needsWork {
			// Compare hash to detect content changes.
			if entries, err := os.ReadDir(fp); err == nil {
				if computeContentHash(entries) != existing.ContentHash {
					needsWork = true
				}
			}
		}
		if needsWork {
			ix.jobs <- job{w.tab, w.category, w.categoryPath, w.folderPath}
		}
	}

	// Prune: anything in DB but not in desired set.
	existing, err := appdb.AllFolderPaths(ix.db)
	if err != nil {
		return err
	}
	sort.Strings(existing)
	for _, fp := range existing {
		if _, ok := desired[fp]; !ok {
			ix.removeFolder(fp)
		}
	}
	ix.markDirty()
	return nil
}

// RescanFolder is called by the watcher when an item folder changes.
// folderPath must be a direct child of a configured category path.
func (ix *Indexer) RescanFolder(folderPath string) {
	folderPath = filepath.Clean(folderPath)
	tab, category, categoryPath := ix.lookupCategoryFor(folderPath)
	if tab == "" {
		// Folder isn't covered by current config; if DB has it, treat as deletion.
		if existing, _ := appdb.GetByPath(ix.db, folderPath); existing != nil {
			ix.removeFolder(folderPath)
			ix.markDirty()
		}
		return
	}
	if _, err := os.Stat(folderPath); errors.Is(err, os.ErrNotExist) {
		ix.removeFolder(folderPath)
		ix.markDirty()
		return
	}
	ix.jobs <- job{tab, category, categoryPath, folderPath}
}

func (ix *Indexer) lookupCategoryFor(folderPath string) (string, string, string) {
	cfg := ix.Config()
	parent := filepath.Clean(filepath.Dir(folderPath))
	for _, t := range cfg.Tabs {
		for _, c := range t.Categories {
			for _, f := range c.Folders {
				if filepath.Clean(f) == parent {
					return t.Name, c.Name, parent
				}
			}
		}
	}
	return "", "", ""
}

func (ix *Indexer) removeFolder(folderPath string) {
	id, err := appdb.DeleteByPath(ix.db, folderPath)
	if err != nil {
		log.Printf("indexer: delete %s: %v", folderPath, err)
		return
	}
	_ = id // id no longer needed — thumbs are keyed by path hash, not id
	_ = os.RemoveAll(ix.paths.ThumbDir(folderPath))
}

// CategoryFor returns (tab, category, categoryPath) for a given category disk path,
// or empty strings if unknown.
func (ix *Indexer) CategoryFor(categoryPath string) (string, string, string) {
	cp := filepath.Clean(categoryPath)
	cfg := ix.Config()
	for _, t := range cfg.Tabs {
		for _, c := range t.Categories {
			for _, f := range c.Folders {
				if filepath.Clean(f) == cp {
					return t.Name, c.Name, cp
				}
			}
		}
	}
	return "", "", ""
}

// String is for debug logging.
func (s Status) String() string { return fmt.Sprintf("scanning=%v queue=%d path=%s", s.Scanning, s.QueueDepth, s.CurrentPath) }
