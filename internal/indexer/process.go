package indexer

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cespare/xxhash/v2"

	appdb "structa/internal/db"
	"structa/internal/paths"
)

// scanResult holds everything extracted from a single mod folder.
type scanResult struct {
	Title        string
	Favorite     bool
	SourceLink   string
	Description  string
	Tags         []string
	Content      []string
	ThumbRel     string
	PreviewRels  []string
	MTime        float64
	ContentHash  string
}

// processFolder reads disk state for one mod folder, regenerates thumbnails,
// and writes/refreshes the corresponding rows in the database.
func processFolder(d *sql.DB, p paths.Paths, tab, category, categoryPath, folderPath string) error {
	info, err := os.Stat(folderPath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("not a directory")
	}

	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return err
	}

	res := &scanResult{
		Title:       filepath.Base(folderPath),
		Tags:        []string{},
		Content:     []string{},
		PreviewRels: []string{},
		MTime:       float64(info.ModTime().UnixNano()) / 1e9,
	}

	// Identify cover candidates by priority, plus all `preview*` files.
	var coverCandidate string
	coverPriority := -1
	var previews []string

	for _, e := range entries {
		name := e.Name()
		lower := strings.ToLower(name)
		full := filepath.Join(folderPath, name)
		switch {
		case lower == "mod.favorite":
			res.Favorite = true
		case lower == "link.url":
			if link, err := readURLFile(full); err == nil && link != "" {
				res.SourceLink = link
			}
		case lower == "tags.txt":
			if tags, err := readTagsFile(full); err == nil {
				res.Tags = tags
			}
		case lower == "description.txt":
			if desc, err := os.ReadFile(full); err == nil {
				res.Description = strings.TrimSpace(string(desc))
			}
		}
		if !e.IsDir() && isImageExt(name) && strings.HasPrefix(lower, "preview") {
			previews = append(previews, name)
			pr := previewPriority(lower)
			if pr > coverPriority {
				coverPriority = pr
				coverCandidate = name
			}
		}
		// Content list: subfolder and archive filenames (skip preview images and marker files).
		if e.IsDir() {
			res.Content = append(res.Content, name)
		} else if !strings.HasPrefix(lower, "preview") && lower != "mod.favorite" && lower != "link.url" && lower != "tags.txt" && lower != "description.txt" {
			res.Content = append(res.Content, name)
		}
	}

	sort.Strings(res.Content)
	sort.Strings(previews)

	// Generate thumbnails. Thumb dir is keyed by hash of folder_path (stable across rename of id).
	thumbDir := p.ThumbDir(folderPath)
	// Remove any previous thumbs so renames/removals don't linger.
	_ = os.RemoveAll(thumbDir)

	if coverCandidate != "" {
		src := filepath.Join(folderPath, coverCandidate)
		dst := filepath.Join(thumbDir, "thumb.jpg")
		if err := resizeToJPEG(src, dst, 300); err == nil {
			res.ThumbRel = filepath.ToSlash(filepath.Join(paths.FolderKey(folderPath), "thumb.jpg"))
		}
	}
	for i, name := range previews {
		src := filepath.Join(folderPath, name)
		dst := filepath.Join(thumbDir, fmt.Sprintf("preview-%d.jpg", i))
		if err := resizeToJPEG(src, dst, 600); err == nil {
			res.PreviewRels = append(res.PreviewRels, filepath.ToSlash(filepath.Join(paths.FolderKey(folderPath), fmt.Sprintf("preview-%d.jpg", i))))
		}
	}

	res.ContentHash = computeContentHash(entries)

	contentJSON, _ := json.Marshal(res.Content)
	previewJSON, _ := json.Marshal(res.PreviewRels)
	tagsJSON, _ := json.Marshal(res.Tags)

	id, err := appdb.Upsert(d, appdb.FolderRow{
		Tab:          tab,
		Category:     category,
		CategoryPath: categoryPath,
		FolderName:   filepath.Base(folderPath),
		FolderPath:   folderPath,
		MTime:        res.MTime,
		ContentHash:  res.ContentHash,
	})
	if err != nil {
		return err
	}

	det := appdb.DetailsRow{
		FolderID:     id,
		Title:        res.Title,
		Favorite:     res.Favorite,
		ContentJSON:  string(contentJSON),
		PreviewPaths: string(previewJSON),
		TagsJSON:     string(tagsJSON),
		Description:  res.Description,
	}
	if res.SourceLink != "" {
		det.SourceLink = sql.NullString{String: res.SourceLink, Valid: true}
	}
	if res.ThumbRel != "" {
		det.ThumbPath = sql.NullString{String: res.ThumbRel, Valid: true}
	}
	return appdb.UpsertDetails(d, det)
}

// readTagsFile parses tags.txt — one tag per line. Empty lines and surrounding
// whitespace are stripped. Duplicate tags (case-insensitive) are collapsed.
func readTagsFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	seen := map[string]struct{}{}
	out := []string{}
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		key := strings.ToLower(line)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, line)
	}
	return out, sc.Err()
}

// readURLFile parses a Windows-style .url shortcut file and returns the URL= value.
func readURLFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if eq := strings.IndexByte(line, '='); eq > 0 {
			key := strings.TrimSpace(strings.ToUpper(line[:eq]))
			val := strings.TrimSpace(line[eq+1:])
			if key == "URL" && val != "" {
				return val, nil
			}
		}
	}
	return "", sc.Err()
}

// previewPriority assigns a priority to a filename matching the legacy cover-selection rule:
// preview1 > preview.a > preview (exact, any extension) > other preview*.
func previewPriority(lower string) int {
	base := strings.TrimSuffix(lower, filepath.Ext(lower))
	switch {
	case base == "preview1":
		return 3
	case base == "preview.a":
		return 2
	case base == "preview":
		return 1
	default:
		return 0
	}
}

// computeContentHash returns a stable hash over (name, isDir, size, mtime) of a folder's direct children.
// It is used to decide whether to re-process a mod folder. Reading sizes/mtimes from DirEntry avoids extra stats.
func computeContentHash(entries []os.DirEntry) string {
	type rec struct {
		name  string
		isDir bool
		size  int64
		mtime int64
	}
	recs := make([]rec, 0, len(entries))
	for _, e := range entries {
		r := rec{name: e.Name(), isDir: e.IsDir()}
		if fi, err := e.Info(); err == nil {
			r.size = fi.Size()
			r.mtime = fi.ModTime().UnixNano()
		}
		recs = append(recs, r)
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].name < recs[j].name })
	h := xxhash.New()
	for _, r := range recs {
		fmt.Fprintf(h, "%s|%t|%d|%d\n", r.name, r.isDir, r.size, r.mtime)
	}
	return fmt.Sprintf("%x", h.Sum64())
}
