# Structa

A native desktop app for indexing and browsing libraries of folders that contain preview images and metadata. Built with [Wails v2](https://wails.io/) — Go backend + React/TypeScript frontend, packaged as a single Windows executable.

Structa is an evolution of an existing file-indexing/organization project, rebuilt around an automatic watcher, an on-disk thumbnail cache, and a system-themed UI. It is general-purpose: any directory tree whose leaves are folders containing images and a few optional text files will work — game asset packs, photography collections, design references, model libraries, etc.

## Highlights

- **One executable** — no Python, no Node runtime on the user's machine. Uses the WebView2 runtime built into Windows 10/11.
- **Automatic indexing** — no manual "scan" button. On launch the indexer reconciles the catalog with disk; while running, `fsnotify` watches every configured folder so adding or removing a folder updates the grid within ~500 ms.
- **On-disk thumbnail cache** — previews are resized to disk under `%APPDATA%/structa/thumbs/` and referenced by path. The SQLite database stays small.
- **System theme** — light or dark, follows `prefers-color-scheme` (the Windows app theme setting).
- **Form-based settings** — three-column editor (Tabs → Categories → Folders) with a native folder picker. No raw JSON editing required.
- **Grid or list views** — toggle in the topbar. Grid clamps tags to 2 lines and descriptions to 3 lines; list shows every tag and up to 10 description lines on a horizontal card.
- **Manual rebuild** — a refresh button in the topbar forces a full re-scan of every configured folder, bypassing the content-hash cache. Useful after changing per-item file conventions (e.g. renaming `tags.txt`) or when the index looks stale.

## Per-item files

Each indexed item is a direct subdirectory of a configured category folder. The indexer reads these optional files from inside the item folder:

| File              | Effect |
|-------------------|--------|
| `preview1.*`, `preview.a.*`, `preview.*` (priority order), then any `preview*.png/jpg/webp` | First match becomes the card cover; all are shown in the preview modal |
| `link.url`        | Standard Windows shortcut. The `URL=` line is parsed and becomes the globe-icon link |
| `mod.favorite`    | Marker file (any content). When present, the item is marked as favorite |
| `tags.txt`        | One tag per line. Tags appear as chips on the card; clicking a chip adds the tag to the active filter; the sidebar search also matches tags case-insensitively |
| `description.txt` | Free-form text shown on the card under the tags. Newlines are preserved; grid view truncates to 3 lines, list view to 10 |

Folder names containing `.ignore` are skipped entirely.

### Example `tags.txt`
```
landscape
high-res
sketch
work-in-progress
```

Searching for `sketch`, `Sketch`, or `SKETCH` in the sidebar will show only items whose title, configured category, or `tags.txt` contains that substring (case-insensitive). Clicking any chip on a card adds that tag to the active filter; clicking it again removes it.

### Example `description.txt`
```
Pen and ink studies done on location.
Some sheets include the reference photos.
```

## Where state lives

Everything lives under `%APPDATA%\structa\`:

```
%APPDATA%\structa\
├── catalog.db        SQLite index (folders + folder_details)
├── config.json       Tabs / categories / folder list
└── thumbs\
    └── <sha1(folder_path)>\
        ├── thumb.jpg      (300 px cover)
        └── preview-N.jpg  (600 px slideshow images)
```

For a full reindex without closing the app, click the refresh icon in the topbar (between the status pill and the gear). For a from-scratch rebuild including thumbnails, close the app and delete `catalog.db` and `thumbs/` — the next launch will regenerate both.

## Building from source

### Prerequisites

- **Go** 1.23+
- **Node.js** 20+
- **Wails CLI** (install via `make wails-cli`)
- **WebView2 runtime** — already on Windows 10/11

### One-shot commands

```bash
make deps           # go mod download + npm install
make build          # produces build/bin/structa.exe
make build-installer# produces build/bin + NSIS installer
make dev            # hot-reload dev window
```

If you don't have `make` on Windows, the equivalent direct calls are:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
go mod download
cd frontend; npm install
wails build           # production
wails dev             # dev with hot reload
```

After modifying any Go method exposed on `App`, regenerate the TS bindings with `make generate` (or `wails generate module`).

## Project layout

```
structa/
├── main.go                    Wails bootstrap, /thumbs HTTP handler
├── app.go                     App struct + bound methods (GetCatalog, ToggleFavorite, ...)
├── internal/
│   ├── paths/                 %APPDATA%/structa path helpers
│   ├── config/                config.json marshal/unmarshal
│   ├── db/                    SQLite schema, migrations, queries (modernc.org/sqlite, no CGO)
│   ├── indexer/               walk → diff → process pipeline + thumbnail generation
│   └── watcher/               fsnotify with per-item debounce
└── frontend/
    └── src/
        ├── App.tsx            Layout + tab routing + event subscriptions
        ├── views/
        │   ├── CatalogView.tsx
        │   └── ConfigView.tsx
        └── components/        Sidebar, Card, PreviewModal, IndexStatusPill, ConfirmDialog, icons
```

## How auto-indexing works

1. On startup, `paths.Resolve()` ensures `%APPDATA%/structa/` and the thumb cache exist.
2. The DB is opened (pure-Go SQLite — `modernc.org/sqlite`); `ensureColumn` migrates additive schema changes.
3. The indexer loads `config.json`, then for every `(tab → category → folder)` triple it lists direct subdirectories and compares each against the `folders` table by `(mtime, content_hash)`. Only changed item folders are enqueued onto a worker pool.
4. Per item folder, `process.go` resizes the cover (300 px) and previews (600 px) into the thumbs dir, parses `link.url`, `mod.favorite`, `tags.txt`, and `description.txt`, then upserts both rows.
5. fsnotify watches every configured folder. Events on a child path are mapped back to the owning item folder and dispatched to a debounced (`500 ms`) `RescanFolder` call.
6. The Go side emits `catalog:updated` events that the React frontend listens to (`EventsOn`), so the grid refreshes without a reload.

## Troubleshooting

- **"No tabs configured"** — open the gear icon (top right), add a tab, then a category, then a folder. The grid populates as the indexer processes the contents.
- **Card has no thumbnail** — the folder has no file matching `preview*` (any extension). Add a `preview1.png` (or any `.jpg`/`.webp`).
- **Renaming a tab/category leaves stale rows** — `Save` re-runs reconcile; orphan rows whose path no longer matches any configured folder are pruned and their thumbs deleted.
- **Tags or description don't show after editing the file in place** — most edits are caught by fsnotify, but on some filesystems (or after the app was offline during the change) the content-hash check can skip the folder. Click the refresh icon in the topbar to force-rebuild every row; the button is disabled while a scan is already in progress.
- **Network drives** — fsnotify can be unreliable on SMB/NFS shares. The startup reconcile scan still catches changes; restart the app or hit the refresh button to re-detect.

## License

Released under the [MIT License](LICENSE).
