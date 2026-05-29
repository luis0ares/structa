import { useEffect, useMemo, useState } from 'react';
import { main } from '../../wailsjs/go/models';
import { DeleteItems, ListMoveDestinations, MoveItems } from '../../wailsjs/go/main/App';
import { Card, ViewMode } from '../components/Card';
import { MetadataEditorModal } from '../components/MetadataEditorModal';
import { Sidebar, FilterState } from '../components/Sidebar';
import { PreviewModal } from '../components/PreviewModal';
import { useConfirm } from '../components/ConfirmDialog';
import { ArrowRightCircleIcon, CloseIcon, TrashIcon } from '../components/icons';

type Props = {
  tab: main.TabDTO | null;
  viewMode: ViewMode;
  showHidden: boolean;
  onFavoriteChange: (id: number, favorite: boolean) => void;
  selectMode: boolean;
  selectedIds: Set<number>;
  onToggleSelected: (id: number) => void;
  onClearSelection: () => void;
  onExitSelectMode: () => void;
};

export function CatalogView({
  tab,
  viewMode,
  showHidden,
  onFavoriteChange,
  selectMode,
  selectedIds,
  onToggleSelected,
  onClearSelection,
  onExitSelectMode,
}: Props) {
  const [search, setSearch] = useState('');
  const [filter, setFilter] = useState<FilterState>({ favoritesOnly: false, selectedTags: [] });
  const [preview, setPreview] = useState<main.ItemCardDTO | null>(null);
  const [editing, setEditing] = useState<main.ItemCardDTO | null>(null);
  const [destinations, setDestinations] = useState<main.MoveDestDTO[]>([]);
  const [moveOpen, setMoveOpen] = useState(false);
  const [bulkBusy, setBulkBusy] = useState(false);
  const confirm = useConfirm();

  useEffect(() => {
    if (!selectMode) return;
    ListMoveDestinations().then(setDestinations).catch(() => setDestinations([]));
  }, [selectMode]);

  useEffect(() => {
    if (!moveOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setMoveOpen(false);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [moveOpen]);

  const selectedCount = selectedIds.size;

  const handleMove = async (dest: main.MoveDestDTO) => {
    setMoveOpen(false);
    setBulkBusy(true);
    try {
      await MoveItems(Array.from(selectedIds), dest.path);
      onClearSelection();
    } catch (err) {
      await confirm({
        title: 'Move failed',
        message: String(err),
        confirmLabel: 'OK',
        cancelLabel: 'Close',
      });
    } finally {
      setBulkBusy(false);
    }
  };

  const handleBulkDelete = async () => {
    const ok = await confirm({
      title: 'Delete selected items?',
      message: `This will permanently delete ${selectedCount} folder${selectedCount === 1 ? '' : 's'} from disk. This cannot be undone.`,
      confirmLabel: 'Delete',
      cancelLabel: 'Cancel',
      danger: true,
    });
    if (!ok) return;
    setBulkBusy(true);
    try {
      await DeleteItems(Array.from(selectedIds));
      onClearSelection();
      onExitSelectMode();
    } catch (err) {
      await confirm({
        title: 'Delete failed',
        message: String(err),
        confirmLabel: 'OK',
        cancelLabel: 'Close',
      });
    } finally {
      setBulkBusy(false);
    }
  };

  const tagSet = useMemo(() => new Set(filter.selectedTags), [filter.selectedTags]);
  const hasFilter = filter.favoritesOnly || tagSet.size > 0;

  // When hidden items aren't being shown, strip them from the tab before any other
  // logic so search, tag filtering, and the sidebar all see the visible subset only.
  const effectiveTab = useMemo<main.TabDTO | null>(() => {
    if (!tab) return null;
    if (showHidden) return tab;
    return {
      ...tab,
      categories: tab.categories.map((c) => ({
        ...c,
        items: c.items.filter((m) => !m.hidden),
      })),
    } as main.TabDTO;
  }, [tab, showHidden]);

  const jumpTo = (id: number) => {
    const el = document.getElementById(`card-${id}`);
    if (el) el.scrollIntoView({ behavior: 'smooth', block: 'center' });
  };

  const toggleTag = (raw: string) => {
    const key = raw.toLowerCase();
    setFilter((f) => ({
      ...f,
      selectedTags: f.selectedTags.includes(key)
        ? f.selectedTags.filter((t) => t !== key)
        : [...f.selectedTags, key],
    }));
  };

  if (!effectiveTab) {
    return (
      <main className="content">
        <div className="empty">
          <h2>Welcome to Structa</h2>
          <p>Click the settings icon in the top right to add your first tab and category.</p>
        </div>
      </main>
    );
  }

  const q = search.trim().toLowerCase();

  const passes = (m: main.ItemCardDTO): boolean => {
    if (filter.favoritesOnly && !m.favorite) return false;
    if (tagSet.size > 0) {
      const itemTags = (m.tags || []).map((t) => t.toLowerCase());
      let ok = false;
      for (const t of tagSet) {
        if (itemTags.includes(t)) { ok = true; break; }
      }
      if (!ok) return false;
    }
    return true;
  };

  const visible = effectiveTab.categories.map((c) => {
    const catHit = c.name.toLowerCase().includes(q);
    const items = c.items.filter((m) => {
      if (!passes(m)) return false;
      if (!q) return true;
      return (
        catHit ||
        m.title.toLowerCase().includes(q) ||
        (m.tags || []).some((t) => t.toLowerCase().includes(q))
      );
    });
    return { ...c, items };
  });
  const hasAny = visible.some((c) => c.items.length > 0);

  return (
    <>
      <Sidebar
        tab={effectiveTab}
        search={search}
        onSearchChange={setSearch}
        filter={filter}
        onFilterChange={setFilter}
        onJumpToItem={jumpTo}
      />
      <main className="content">
        {!hasAny ? (
          <div className="empty">
            {q || hasFilter ? <p>No items match the current filters</p> : (
              <>
                <p>No items indexed in this tab yet.</p>
                <p style={{ fontSize: 12 }}>
                  Add folders in the settings, or drop items into a watched folder — they'll appear automatically.
                </p>
              </>
            )}
          </div>
        ) : (
          visible.map((c) =>
            c.items.length === 0 ? null : (
              <section key={c.name} className="cat-section">
                <h3 className="cat-section-h">{c.name}</h3>
                <div className={`grid ${viewMode === 'list' ? 'list' : ''}`}>
                  {c.items.map((m) => (
                    <Card
                      key={m.id}
                      item={m}
                      viewMode={viewMode}
                      selectedTagKeys={tagSet}
                      onFavoriteChange={onFavoriteChange}
                      onOpenPreview={setPreview}
                      onTagClick={toggleTag}
                      onEditMeta={setEditing}
                      selectMode={selectMode}
                      selected={selectedIds.has(m.id)}
                      onToggleSelected={onToggleSelected}
                    />
                  ))}
                </div>
              </section>
            ),
          )
        )}
      </main>
      {selectMode && selectedCount > 0 && (
        <div className="bulk-bar" role="region" aria-label="Bulk actions">
          <div className="bulk-bar-count">
            <strong>{selectedCount}</strong> selected
          </div>
          <div className="bulk-bar-actions">
            <div className="bulk-move-wrap">
              <button
                type="button"
                className="bulk-btn"
                onClick={() => setMoveOpen((v) => !v)}
                disabled={bulkBusy || destinations.length === 0}
                title={destinations.length === 0 ? 'No category folders configured' : 'Move to folder'}
              >
                <ArrowRightCircleIcon size={14} />
                Move to…
              </button>
              {moveOpen && (
                <div className="bulk-move-menu" role="menu">
                  {destinations.length === 0 ? (
                    <div className="bulk-move-empty">No folders configured</div>
                  ) : (
                    destinations.map((d) => (
                      <button
                        key={d.path}
                        type="button"
                        role="menuitem"
                        className="bulk-move-item"
                        onClick={() => handleMove(d)}
                      >
                        <span className="bulk-move-item-label">{d.tab} / {d.category}</span>
                        <span className="bulk-move-item-path">{d.path}</span>
                      </button>
                    ))
                  )}
                </div>
              )}
            </div>
            <button
              type="button"
              className="bulk-btn danger"
              onClick={handleBulkDelete}
              disabled={bulkBusy}
            >
              <TrashIcon size={14} />
              Delete
            </button>
            <button
              type="button"
              className="bulk-btn ghost"
              onClick={onClearSelection}
              disabled={bulkBusy}
            >
              <CloseIcon size={14} />
              Clear
            </button>
          </div>
        </div>
      )}
      {preview && (
        <PreviewModal
          itemId={preview.id}
          fallbackUrl={preview.thumbUrl}
          onClose={() => setPreview(null)}
        />
      )}
      {editing && (
        <MetadataEditorModal
          item={editing}
          onClose={() => setEditing(null)}
          onSaved={() => onFavoriteChange(editing.id, editing.favorite)}
        />
      )}
    </>
  );
}
