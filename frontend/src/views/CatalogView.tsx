import { useMemo, useState } from 'react';
import { main } from '../../wailsjs/go/models';
import { Card, ViewMode } from '../components/Card';
import { MetadataEditorModal } from '../components/MetadataEditorModal';
import { Sidebar, FilterState } from '../components/Sidebar';
import { PreviewModal } from '../components/PreviewModal';

type Props = {
  tab: main.TabDTO | null;
  viewMode: ViewMode;
  onFavoriteChange: (id: number, favorite: boolean) => void;
};

export function CatalogView({ tab, viewMode, onFavoriteChange }: Props) {
  const [search, setSearch] = useState('');
  const [filter, setFilter] = useState<FilterState>({ favoritesOnly: false, selectedTags: [] });
  const [preview, setPreview] = useState<main.ItemCardDTO | null>(null);
  const [editing, setEditing] = useState<main.ItemCardDTO | null>(null);

  const tagSet = useMemo(() => new Set(filter.selectedTags), [filter.selectedTags]);
  const hasFilter = filter.favoritesOnly || tagSet.size > 0;

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

  if (!tab) {
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

  const visible = tab.categories.map((c) => {
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
        tab={tab}
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
                    />
                  ))}
                </div>
              </section>
            ),
          )
        )}
      </main>
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
