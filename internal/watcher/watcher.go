package watcher

import (
	"context"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Rescanner is the callback the watcher invokes for an item folder that changed.
type Rescanner interface {
	RescanFolder(folderPath string)
	CategoryFor(categoryPath string) (tab, category, categoryPath2 string)
}

// Watcher tracks fsnotify events on category directories and dispatches debounced
// rescan requests for the affected direct-child item folder.
type Watcher struct {
	w     *fsnotify.Watcher
	rs    Rescanner
	mu    sync.Mutex
	dirs  map[string]struct{}
	timers map[string]*time.Timer
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
		timers:   map[string]*time.Timer{},
		debounce: 500 * time.Millisecond,
	}, nil
}

// SetRoots replaces the set of watched directories with the supplied list.
// Anything currently watched but not in the new list is removed; new entries are added.
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

	var root, itemFolder string
	for _, r := range roots {
		rel, err := filepath.Rel(r, p)
		if err != nil || rel == "." || rel == ".." || hasParentMarker(rel) {
			continue
		}
		root = r
		// First component of rel is the item folder name.
		sep := string(filepath.Separator)
		idx := indexOfSep(rel, sep)
		if idx < 0 {
			itemFolder = filepath.Join(r, rel)
		} else {
			itemFolder = filepath.Join(r, rel[:idx])
		}
		break
	}
	if root == "" {
		// Could also be the category dir itself (e.g. dir created/removed) — let reconcile handle that case
		// by re-firing on next config change. Nothing to do here.
		return
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

func hasParentMarker(rel string) bool {
	return rel == ".." || (len(rel) >= 3 && rel[:3] == "..")
}

func indexOfSep(s, sep string) int {
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep {
			return i
		}
	}
	return -1
}
