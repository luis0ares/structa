package db

import (
	"database/sql"
	"time"
)

type FolderRow struct {
	ID           int64
	Tab          string
	Category     string
	CategoryPath string
	FolderName   string
	FolderPath   string
	MTime        float64
	ContentHash  string
}

type DetailsRow struct {
	FolderID     int64
	Title        string
	Favorite     bool
	SourceLink   sql.NullString
	ContentJSON  string
	ThumbPath    sql.NullString
	PreviewPaths string
	TagsJSON     string
	Description  string
}

type CardRow struct {
	FolderRow
	DetailsRow
}

// GetByPath returns the folder row for a given folder_path, or nil if not present.
func GetByPath(d *sql.DB, folderPath string) (*FolderRow, error) {
	row := d.QueryRow(`SELECT id, tab, category, category_path, folder_name, folder_path, mtime, content_hash
	                   FROM folders WHERE folder_path = ?`, folderPath)
	var f FolderRow
	err := row.Scan(&f.ID, &f.Tab, &f.Category, &f.CategoryPath, &f.FolderName, &f.FolderPath, &f.MTime, &f.ContentHash)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// Upsert inserts or updates a folder row and returns the row id.
func Upsert(d *sql.DB, f FolderRow) (int64, error) {
	now := time.Now().Unix()
	res, err := d.Exec(`
		INSERT INTO folders (tab, category, category_path, folder_name, folder_path, mtime, content_hash, indexed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(folder_path) DO UPDATE SET
			tab=excluded.tab,
			category=excluded.category,
			category_path=excluded.category_path,
			folder_name=excluded.folder_name,
			mtime=excluded.mtime,
			content_hash=excluded.content_hash,
			indexed_at=excluded.indexed_at
	`, f.Tab, f.Category, f.CategoryPath, f.FolderName, f.FolderPath, f.MTime, f.ContentHash, now)
	if err != nil {
		return 0, err
	}
	// On conflict the LastInsertId may not be correct — re-query.
	if id, err := res.LastInsertId(); err == nil && id != 0 {
		// Verify the id corresponds to this folder_path (could be a previous insert reused).
		var got string
		if err := d.QueryRow(`SELECT folder_path FROM folders WHERE id = ?`, id).Scan(&got); err == nil && got == f.FolderPath {
			return id, nil
		}
	}
	var id int64
	if err := d.QueryRow(`SELECT id FROM folders WHERE folder_path = ?`, f.FolderPath).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func UpsertDetails(d *sql.DB, r DetailsRow) error {
	if r.TagsJSON == "" {
		r.TagsJSON = "[]"
	}
	_, err := d.Exec(`
		INSERT INTO folder_details (folder_id, title, favorite, source_link, content_json, thumb_path, preview_paths, tags_json, description)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(folder_id) DO UPDATE SET
			title=excluded.title,
			favorite=excluded.favorite,
			source_link=excluded.source_link,
			content_json=excluded.content_json,
			thumb_path=excluded.thumb_path,
			preview_paths=excluded.preview_paths,
			tags_json=excluded.tags_json,
			description=excluded.description
	`, r.FolderID, r.Title, boolToInt(r.Favorite), nullableString(r.SourceLink), r.ContentJSON, nullableString(r.ThumbPath), r.PreviewPaths, r.TagsJSON, r.Description)
	return err
}

func DeleteByPath(d *sql.DB, folderPath string) (int64, error) {
	// Returns the deleted row id (or 0) so callers can purge the thumbs dir.
	var id int64
	err := d.QueryRow(`SELECT id FROM folders WHERE folder_path = ?`, folderPath).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if _, err := d.Exec(`DELETE FROM folders WHERE id = ?`, id); err != nil {
		return 0, err
	}
	return id, nil
}

// AllFolderPaths lists every folder_path known to the DB.
func AllFolderPaths(d *sql.DB) ([]string, error) {
	rows, err := d.Query(`SELECT folder_path FROM folders`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// AllCards returns one joined row per known folder, ordered by tab → category → folder_name.
func AllCards(d *sql.DB) ([]CardRow, error) {
	rows, err := d.Query(`
		SELECT f.id, f.tab, f.category, f.category_path, f.folder_name, f.folder_path, f.mtime, f.content_hash,
		       fd.title, fd.favorite, fd.source_link, fd.content_json, fd.thumb_path, fd.preview_paths, fd.tags_json, fd.description
		FROM folders f
		LEFT JOIN folder_details fd ON fd.folder_id = f.id
		ORDER BY f.tab, f.category, f.folder_name COLLATE NOCASE
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CardRow
	for rows.Next() {
		var c CardRow
		var fav sql.NullInt64
		var title, contentJSON, previewPaths, tagsJSON, description sql.NullString
		if err := rows.Scan(&c.ID, &c.Tab, &c.Category, &c.CategoryPath, &c.FolderName, &c.FolderPath, &c.MTime, &c.ContentHash,
			&title, &fav, &c.SourceLink, &contentJSON, &c.ThumbPath, &previewPaths, &tagsJSON, &description); err != nil {
			return nil, err
		}
		c.FolderID = c.ID
		if title.Valid {
			c.Title = title.String
		} else {
			c.Title = c.FolderName
		}
		c.Favorite = fav.Valid && fav.Int64 != 0
		c.ContentJSON = nonEmptyJSON(contentJSON)
		c.PreviewPaths = nonEmptyJSON(previewPaths)
		c.TagsJSON = nonEmptyJSON(tagsJSON)
		if description.Valid {
			c.Description = description.String
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func GetCard(d *sql.DB, id int64) (*CardRow, error) {
	row := d.QueryRow(`
		SELECT f.id, f.tab, f.category, f.category_path, f.folder_name, f.folder_path, f.mtime, f.content_hash,
		       fd.title, fd.favorite, fd.source_link, fd.content_json, fd.thumb_path, fd.preview_paths, fd.tags_json, fd.description
		FROM folders f
		LEFT JOIN folder_details fd ON fd.folder_id = f.id
		WHERE f.id = ?
	`, id)
	var c CardRow
	var fav sql.NullInt64
	var title, contentJSON, previewPaths, tagsJSON, description sql.NullString
	if err := row.Scan(&c.ID, &c.Tab, &c.Category, &c.CategoryPath, &c.FolderName, &c.FolderPath, &c.MTime, &c.ContentHash,
		&title, &fav, &c.SourceLink, &contentJSON, &c.ThumbPath, &previewPaths, &tagsJSON, &description); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	c.FolderID = c.ID
	if title.Valid {
		c.Title = title.String
	} else {
		c.Title = c.FolderName
	}
	c.Favorite = fav.Valid && fav.Int64 != 0
	c.ContentJSON = nonEmptyJSON(contentJSON)
	c.PreviewPaths = nonEmptyJSON(previewPaths)
	c.TagsJSON = nonEmptyJSON(tagsJSON)
	if description.Valid {
		c.Description = description.String
	}
	return &c, nil
}

func nonEmptyJSON(s sql.NullString) string {
	if s.Valid && s.String != "" {
		return s.String
	}
	return "[]"
}

func SetFavorite(d *sql.DB, id int64, favorite bool) error {
	_, err := d.Exec(`UPDATE folder_details SET favorite = ? WHERE folder_id = ?`, boolToInt(favorite), id)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullableString(s sql.NullString) interface{} {
	if !s.Valid {
		return nil
	}
	return s.String
}
