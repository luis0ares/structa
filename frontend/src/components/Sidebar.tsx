import { useMemo, useRef, useState } from "react";
import { main } from "../../wailsjs/go/models";
import { SortMode, sortItems } from "../sort";
import { ChevronIcon, FilterIcon, SearchIcon, StarIcon } from "./icons";

export type FilterState = {
  favoritesOnly: boolean;
  selectedTags: string[]; // lower-case
};

type Props = {
  tab: main.TabDTO | null;
  search: string;
  onSearchChange: (s: string) => void;
  filter: FilterState;
  onFilterChange: (f: FilterState) => void;
  onJumpToItem: (id: number) => void;
  sort: SortMode;
};

export function Sidebar({
  tab,
  search,
  onSearchChange,
  filter,
  onFilterChange,
  onJumpToItem,
  sort,
}: Props) {
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
  const [filterOpen, setFilterOpen] = useState(false);
  const filterButtonRef = useRef<HTMLButtonElement | null>(null);

  // Unique tags across the current tab, with their occurrence count.
  const tagOptions = useMemo(() => {
    const counts = new Map<string, { label: string; n: number }>();
    if (tab) {
      for (const c of tab.categories) {
        for (const m of c.items) {
          for (const raw of m.tags || []) {
            const key = raw.toLowerCase();
            const existing = counts.get(key);
            if (existing) existing.n += 1;
            else counts.set(key, { label: raw, n: 1 });
          }
        }
      }
    }
    return Array.from(counts.entries())
      .sort((a, b) => a[1].label.localeCompare(b[1].label))
      .map(([key, v]) => ({ key, label: v.label, n: v.n }));
  }, [tab]);

  const selectedSet = useMemo(
    () => new Set(filter.selectedTags),
    [filter.selectedTags],
  );
  const hasFilter = filter.favoritesOnly || filter.selectedTags.length > 0;

  const passes = (m: main.ItemCardDTO) => {
    if (filter.favoritesOnly && !m.favorite) return false;
    if (selectedSet.size > 0) {
      const itemTags = (m.tags || []).map((t) => t.toLowerCase());
      let ok = false;
      for (const t of selectedSet) {
        if (itemTags.includes(t)) {
          ok = true;
          break;
        }
      }
      if (!ok) return false;
    }
    return true;
  };

  const filtered = useMemo(() => {
    if (!tab) return [];
    const q = search.trim().toLowerCase();
    return tab.categories
      .map((c) => {
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
        return { ...c, items: sortItems(items, sort) };
      })
      .filter((c) => (q || hasFilter ? c.items.length > 0 : true));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tab, search, filter.favoritesOnly, filter.selectedTags, sort]);

  const toggle = (name: string) =>
    setCollapsed((s) => ({ ...s, [name]: !s[name] }));

  const toggleTag = (key: string) => {
    const next = selectedSet.has(key)
      ? filter.selectedTags.filter((t) => t !== key)
      : [...filter.selectedTags, key];
    onFilterChange({ ...filter, selectedTags: next });
  };

  const clearFilters = () =>
    onFilterChange({ favoritesOnly: false, selectedTags: [] });

  return (
    <aside className="sidebar">
      <div className="sidebar-search">
        <div className="sidebar-search-row">
          <div className="sidebar-search-input">
            <input
              type="search"
              placeholder="Search by name or tags"
              value={search}
              onChange={(e) => onSearchChange(e.target.value)}
              style={{ paddingLeft: 30 }}
            />
            <SearchIcon
              size={16}
              style={{
                position: "absolute",
                left: 9,
                top: 9,
                color: "var(--fg-muted)",
              }}
            />
          </div>
          <button
            ref={filterButtonRef}
            type="button"
            className={`filter-btn ${hasFilter ? "has-filter" : ""}`}
            onClick={() => setFilterOpen((v) => !v)}
            title="Filters"
            aria-label="Filters"
            aria-expanded={filterOpen}
          >
            <FilterIcon size={16} />
          </button>
        </div>
        {filterOpen && (
          <>
            <div className="filter-panel" role="region" aria-label="Filters">
              <div className="filter-panel-section">
                <div className="filter-panel-label">Quick</div>
                <label className="filter-row">
                  <input
                    type="checkbox"
                    checked={filter.favoritesOnly}
                    onChange={(e) =>
                      onFilterChange({
                        ...filter,
                        favoritesOnly: e.target.checked,
                      })
                    }
                  />
                  <span className="label">
                    <StarIcon
                      size={12}
                      style={{ verticalAlign: "-2px", marginRight: 4 }}
                      filled
                    />
                    Favorites only
                  </span>
                </label>
              </div>
              <div className="filter-panel-section">
                <div className="filter-panel-label">
                  Tags {tagOptions.length > 0 && `(${tagOptions.length})`}
                </div>
                {tagOptions.length === 0 ? (
                  <div className="config-empty" style={{ padding: "6px" }}>
                    No tags found in this tab
                  </div>
                ) : (
                  tagOptions.map((t) => (
                    <label key={t.key} className="filter-row">
                      <input
                        type="checkbox"
                        checked={selectedSet.has(t.key)}
                        onChange={() => toggleTag(t.key)}
                      />
                      <span className="label">{t.label}</span>
                      <span className="count">{t.n}</span>
                    </label>
                  ))
                )}
              </div>
            </div>
            {hasFilter && (
              <div className="filter-panel-footer">
                <button onClick={clearFilters}>Clear filters</button>
              </div>
            )}
          </>
        )}
      </div>
      <div className="sidebar-tree">
        {filtered.length === 0 && (
          <div className="config-empty">No matches</div>
        )}
        {filtered.map((c) => {
          if (c.items.length === 0) return null;

          const isCollapsed = !!collapsed[c.name];
          return (
            <div key={c.name} className="cat-group">
              <div
                className={`cat-header ${isCollapsed ? "collapsed" : ""}`}
                onClick={() => toggle(c.name)}
              >
                <ChevronIcon className="chev" size={14} />
                <span>{c.name}</span>
                <span
                  style={{
                    marginLeft: "auto",
                    color: "var(--fg-muted)",
                    fontWeight: 400,
                  }}
                >
                  {c.items.length}
                </span>
              </div>
              {!isCollapsed && (
                <div className="cat-children">
                  {c.items.map((m) => (
                    <div
                      key={m.id}
                      className={`cat-item ${m.favorite ? "favorite" : ""}`}
                      title={m.title}
                      onClick={() => onJumpToItem(m.id)}
                    >
                      {m.title}
                    </div>
                  ))}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </aside>
  );
}
