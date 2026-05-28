package watcher

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Rescanner is the callback the watcher invokes for an item folder that changed.
type Rescanner interface {
	RescanFolder(folderPath string)
	CategoryFor(categoryPath string) (tab, category, categoryPath2 string)
}

// Watcher tracks fsnotify events on category directories (and their item-folder children)
// and dispatches debounced rescan requests for the affected item folder.
type Watcher struct {
	w        *fsnotify.Watcher
	rs       Rescanner
	mu       sync.Mutex
	dirs     map[string]struct{} // configured category roots
	items    map[string]struct{} // item folders (direct children of roots)
	timers   map[string]*time.Timer
	debounce time.Duration
}

func New(rs Rescanner) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{
		w:        fw,
		rs:       rs,
		dirs:     map[string]struct{}{},
		items:    map[string]struct{}{},
		timers:   map[string]*time.Timer{},
		debounce: 500 * time.Millisecond,
	}, nil
}

// SetRoots replaces the set of watched directories with the supplied list.
// Anything currently watched but not in the new list is removed; new entries are added.
// Item folders (direct children of each root) are also watched so that file-level
// changes inside an existing item folder are detected.
func (w *Watcher) SetRoots(roots []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	wanted := map[string]struct{}{}
	for _, r := range roots {
		wanted[filepath.Clean(r)] = struct{}{}
	}
	for d := range w.dirs {
		if _, ok := wanted[d]; !ok {
			_ = w.w.Remove(d)
			delete(w.dirs, d)
		}
	}
	for d := range wanted {
		if _, ok := w.dirs[d]; ok {
			continue
		}
		if err := w.w.Add(d); err != nil {
			log.Printf("watcher: add %s: %v", d, err)
			continue
		}
		w.dirs[d] = struct{}{}
	}

	// Rebuild item-folder watches from scratch so the set always matches current roots.
	for it := range w.items {
		_ = w.w.Remove(it)
		delete(w.items, it)
	}
	added := 0
	for d := range w.dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			itemPath := filepath.Join(d, e.Name())
			if err := w.w.Add(itemPath); err != nil {
				log.Printf("watcher: add item %s: %v", itemPath, err)
				continue
			}
			w.items[itemPath] = struct{}{}
			added++
		}
	}
	log.Printf("watcher: SetRoots — %d roots, %d item folders watched", len(w.dirs), added)
}

func (w *Watcher) Run(ctx context.Context) {
	defer w.w.Close()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-w.w.Events:
			if !ok {
				return
			}
			w.handle(ev)
		case err, ok := <-w.w.Errors:
			if !ok {
				return
			}
			log.Printf("watcher: %v", err)
		}
	}
}

// handle determines which item folder is affected by the event path
// (event paths can be the category dir, a direct child, or a deeper descendant).
func (w *Watcher) handle(ev fsnotify.Event) {
	p := filepath.Clean(ev.Name)
	// Find the watched root that is an ancestor of p.
	w.mu.Lock()
	roots := make([]string, 0, len(w.dirs))
	for d := range w.dirs {
		roots = append(roots, d)
	}
	w.mu.Unlock()

	// Among all roots that are true ancestors of p, pick the most specific (longest).
	// This matters when configured roots are nested (e.g. both ".../mods" and ".../mods/Gear").
	sep := string(filepath.Separator)
	upPrefix := ".." + sep
	var root, itemFolder string
	bestLen := -1
	for _, r := range roots {
		rel, err := filepath.Rel(r, p)
		if err != nil {
			continue
		}
		if rel == "." || rel == ".." || strings.HasPrefix(rel, upPrefix) {
			continue
		}
		if len(r) <= bestLen {
			continue
		}
		bestLen = len(r)
		root = r
		idx := indexOfSep(rel, sep)
		if idx < 0 {
			itemFolder = filepath.Join(r, rel)
		} else {
			itemFolder = filepath.Join(r, rel[:idx])
		}
	}
	if root == "" {
		// Could also be the category dir itself (e.g. dir created/removed) — let reconcile handle that case
		// by re-firing on next config change. Nothing to do here.
		return
	}

	// Keep item-folder watches in sync: add when a new direct child appears, drop when it goes away.
	if p == itemFolder {
		if ev.Op&fsnotify.Create != 0 {
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				w.mu.Lock()
				if _, ok := w.items[p]; !ok {
					if err := w.w.Add(p); err == nil {
						w.items[p] = struct{}{}
						log.Printf("watcher: +item %s (total=%d)", p, len(w.items))
					} else {
						log.Printf("watcher: add item %s: %v", p, err)
					}
				}
				w.mu.Unlock()
			}
		}
		if ev.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
			w.mu.Lock()
			if _, ok := w.items[p]; ok {
				_ = w.w.Remove(p)
				delete(w.items, p)
				log.Printf("watcher: -item %s (total=%d)", p, len(w.items))
			}
			w.mu.Unlock()
		}
	}

	w.scheduleRescan(itemFolder)
}

func (w *Watcher) scheduleRescan(itemFolder string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if t, ok := w.timers[itemFolder]; ok {
		t.Stop()
	}
	w.timers[itemFolder] = time.AfterFunc(w.debounce, func() {
		w.mu.Lock()
		delete(w.timers, itemFolder)
		w.mu.Unlock()
		w.rs.RescanFolder(itemFolder)
	})
}

func indexOfSep(s, sep string) int {
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep {
			return i
		}
	}
	return -1
}
