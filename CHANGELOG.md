# tag 1.0.0 - Metadata, Profiles & Bulk Actions

The first big quality-of-life release. Per-item metadata is consolidated into a single `structa.yaml` and editable from inside the app. Libraries become portable thanks to multi-profile support — each profile is a folder with a self-contained `.structa/` sidecar, so you can swap, copy, sync, or back up an entire library at the filesystem level. Bulk-select mode and a hidden-item flag round out the day-to-day workflow.

### Features
- **Multi-profile support** — profiles live in `profiles.json` under `%APPDATA%\structa\`, but every catalog/config/thumbnail for a profile is stored inside a `.structa/` sidecar in the profile's folder. Switch, create, or import profiles from the topbar.
- **In-app metadata editor** — new modal to edit name, tags, link, description, favorite, and hidden flags. Saves back to `structa.yaml`.
- **Consolidated `structa.yaml` format** — replaces the previous loose `tags.txt` / `description.txt` / `link.url` / `.favorite` files with a single YAML document per item.
- **Hidden items** — new `hidden:` field in `structa.yaml` plus an eye toggle in the topbar to show or re-hide them in bulk. Hidden items are excluded from search and tag filters by default.
- **Bulk multi-select mode** — checkbox toolbar action enables multi-select across the grid, with move and delete operations and a per-category select-all checkbox.
- **Tag sorting** — tags are now sorted consistently on cards and in the editor modal.
- **Filter UX tweak** — the *Clear filters* button now lives outside the collapsing filter panel so it's always reachable.
- **Updated application icons.**
- **Generic example library** — bundled `example/` directory rewritten with neutral categories (Archives, Documents, Images, Videos) and the new `structa.yaml` + `preview.jpg` layout.

### Fixes
- Indexer no longer mis-skips folders whose names contain `.ignore` as a substring (only exact `.ignore` directories are skipped).
- Watcher excludes the profile's `.structa/` sidecar from the item-level watch tree, preventing self-triggered rescans.
- Profile picker dialog no longer creates an empty directory when the user cancels the folder picker.
- Profile inline edit form now fills the available width instead of double-bordering.

---

# tag 0.0.2 - CI & Example Data

Adds the release pipeline and the first version of the bundled example library, plus a frontend component refactor in preparation for the metadata editor.

### Features
- **GitHub Actions release workflow** — manual `workflow_dispatch` trigger builds a Windows `.exe` via Wails and publishes it to a tagged GitHub Release.
- **Example data** — first pass at a bundled `example/` directory so new users can see a populated catalog without supplying their own files.
- **Frontend component refactor** — extracted reusable building blocks ahead of the upcoming metadata-editor and profile features.

---

# tag 0.0.1 - Initial Release

First public cut of Structa.

### Features
- **Wails v2 desktop shell** — Go backend, React + TypeScript + Vite frontend, packaged as a single Windows executable using the built-in WebView2 runtime.
- **Auto-indexing pipeline** — startup reconcile scan plus an `fsnotify` watcher on every configured folder; changes propagate to the UI within ~500 ms via `catalog:updated` events.
- **SQLite catalog** — pure-Go `modernc.org/sqlite` (no CGO) with additive migrations via `ensureColumn`.
- **On-disk thumbnail cache** — covers resized to 300 px, slideshow previews to 600 px, stored under `%APPDATA%\structa\thumbs\<sha1(folder)>\`.
- **Tabs / Categories / Folders configuration** — three-column form editor with a native folder picker; no raw JSON editing required.
- **Per-item file conventions** — `preview*.png/jpg/webp`, `link.url`, `.favorite`, `tags.txt`, `description.txt`.
- **Grid and list catalog views** with sidebar tab navigation, tag filtering, and free-text search across name, category, and tags.
- **System theme support** — follows `prefers-color-scheme`.
- **Manual rebuild button** — forces a full re-scan bypassing the content-hash cache.
