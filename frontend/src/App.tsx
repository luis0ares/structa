import { useCallback, useEffect, useState } from 'react';
import { EventsOn } from '../wailsjs/runtime';
import { ForceReindex, GetCatalog, IndexerStatus } from '../wailsjs/go/main/App';
import { indexer, main } from '../wailsjs/go/models';
import { CatalogView } from './views/CatalogView';
import { ConfigView } from './views/ConfigView';
import { IndexStatusPill } from './components/IndexStatusPill';
import { ConfirmProvider } from './components/ConfirmDialog';
import { CheckSquareIcon, EyeIcon, EyeOffIcon, GearIcon, GridIcon, ListIcon, RefreshIcon } from './components/icons';
import type { ViewMode } from './components/Card';

const SETTINGS_TAB = '__settings__';

function App() {
  const [catalog, setCatalog] = useState<main.TabDTO[]>([]);
  const [activeTab, setActiveTab] = useState<string>('');
  const [status, setStatus] = useState<indexer.Status>(
    indexer.Status.createFrom({ scanning: false, currentPath: '', queueDepth: 0 }),
  );
  const [viewMode, setViewMode] = useState<ViewMode>('grid');
  const [showHidden, setShowHidden] = useState(false);
  const [selectMode, setSelectMode] = useState(false);
  const [selectedIds, setSelectedIds] = useState<Set<number>>(() => new Set());

  const toggleSelected = useCallback((id: number) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const clearSelection = useCallback(() => setSelectedIds(new Set()), []);

  const selectGroup = useCallback((ids: number[], select: boolean) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (select) ids.forEach((id) => next.add(id));
      else ids.forEach((id) => next.delete(id));
      return next;
    });
  }, []);

  const exitSelectMode = useCallback(() => {
    setSelectMode(false);
    setSelectedIds(new Set());
  }, []);

  const refresh = useCallback(async () => {
    const data = await GetCatalog();
    setCatalog(data);
    if (data.length > 0 && (activeTab === '' || activeTab === SETTINGS_TAB && false)) {
      // Only auto-select on first load
      setActiveTab((prev) => prev || data[0].name);
    }
  }, [activeTab]);

  useEffect(() => {
    refresh();
    IndexerStatus().then((s) => setStatus(s));
    const off1 = EventsOn('catalog:updated', () => refresh());
    const off2 = EventsOn('indexer:status', (s: any) => setStatus(indexer.Status.createFrom(s)));
    return () => { off1(); off2(); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const currentTab =
    activeTab && activeTab !== SETTINGS_TAB
      ? catalog.find((t) => t.name === activeTab) ?? catalog[0] ?? null
      : null;

  const onFavoriteChange = (id: number, favorite: boolean) => {
    setCatalog((cat) =>
      cat.map((t) =>
        main.TabDTO.createFrom({
          name: t.name,
          categories: t.categories.map((c) =>
            main.CategoryDTO.createFrom({
              name: c.name,
              items: c.items.map((m) => (m.id === id ? main.ItemCardDTO.createFrom({ ...m, favorite }) : m)),
            }),
          ),
        }),
      ),
    );
  };

  return (
    <ConfirmProvider>
    <div className="app">
      <header className="topbar">
        <div className="tabs">
          {catalog.map((t) => (
            <div
              key={t.name}
              className={`tab ${activeTab === t.name ? 'active' : ''}`}
              onClick={() => setActiveTab(t.name)}
            >
              {t.name}
            </div>
          ))}
          {catalog.length === 0 && (
            <div className="tab" style={{ color: 'var(--fg-muted)', borderBottom: 'none', cursor: 'default' }}>
              No tabs configured
            </div>
          )}
        </div>
        <div className="topbar-right">
          <IndexStatusPill status={status} />
          <button
            className="topbar-tab-btn"
            onClick={() => ForceReindex()}
            title="Rebuild index (rescan every folder)"
            aria-label="Rebuild index"
            disabled={status.scanning}
          >
            <RefreshIcon />
          </button>
          {activeTab !== SETTINGS_TAB && (
            <>
              <span className="topbar-divider" aria-hidden="true" />
              <div className="view-toggle" role="group" aria-label="View mode">
                <button
                  className={viewMode === 'grid' ? 'active' : ''}
                  onClick={() => setViewMode('grid')}
                  title="Grid view"
                  aria-label="Grid view"
                  aria-pressed={viewMode === 'grid'}
                >
                  <GridIcon size={14} />
                </button>
                <button
                  className={viewMode === 'list' ? 'active' : ''}
                  onClick={() => setViewMode('list')}
                  title="List view"
                  aria-label="List view"
                  aria-pressed={viewMode === 'list'}
                >
                  <ListIcon size={14} />
                </button>
              </div>
              <button
                className={`topbar-tab-btn ${showHidden ? 'active' : ''}`}
                onClick={() => setShowHidden((v) => !v)}
                title={showHidden ? 'Hiding from view (showing hidden items)' : 'Show hidden items'}
                aria-label="Toggle hidden items"
                aria-pressed={showHidden}
              >
                {showHidden ? <EyeIcon /> : <EyeOffIcon />}
              </button>
              <button
                className={`topbar-tab-btn ${selectMode ? 'active' : ''}`}
                onClick={() => (selectMode ? exitSelectMode() : setSelectMode(true))}
                title={selectMode ? 'Exit selection mode' : 'Select items in bulk'}
                aria-label="Toggle selection mode"
                aria-pressed={selectMode}
              >
                <CheckSquareIcon />
              </button>
            </>
          )}
          <button
            className={`topbar-tab-btn ${activeTab === SETTINGS_TAB ? 'active' : ''}`}
            onClick={() => setActiveTab(SETTINGS_TAB)}
            title="Settings"
            aria-label="Settings"
          >
            <GearIcon />
          </button>
        </div>
      </header>

      <div className="body" style={activeTab === SETTINGS_TAB ? { gridTemplateColumns: '1fr' } : undefined}>
        {activeTab === SETTINGS_TAB ? (
          <ConfigView onSaved={refresh} />
        ) : (
          <CatalogView
            tab={currentTab}
            viewMode={viewMode}
            showHidden={showHidden}
            onFavoriteChange={onFavoriteChange}
            selectMode={selectMode}
            selectedIds={selectedIds}
            onToggleSelected={toggleSelected}
            onSelectGroup={selectGroup}
            onClearSelection={clearSelection}
            onExitSelectMode={exitSelectMode}
          />
        )}
      </div>
    </div>
    </ConfirmProvider>
  );
}

export default App;
