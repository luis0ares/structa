import { useEffect, useState } from 'react';
import { config } from '../../wailsjs/go/models';
import { GetConfig, PickFolder, SaveConfig } from '../../wailsjs/go/main/App';
import { PencilIcon, PlusIcon, TrashIcon } from '../components/icons';
import { useConfirm } from '../components/ConfirmDialog';

type Tab = config.Tab;
type Category = config.Category;

type Draft = { tabs: Tab[] };

type DragState =
  | { kind: 'tab' | 'cat'; from: number }
  | null;

type DropTarget =
  | { kind: 'tab' | 'cat'; index: number; before: boolean }
  | null;

function emptyTab(name: string): Tab {
  return config.Tab.createFrom({ tab_name: name, categories: [] });
}
function emptyCategory(name: string): Category {
  return config.Category.createFrom({ name, folders: [] });
}

function moveItem<T>(arr: T[], from: number, to: number): T[] {
  if (from === to || from < 0 || to < 0 || from >= arr.length || to > arr.length) return arr;
  const next = arr.slice();
  const [item] = next.splice(from, 1);
  next.splice(to > from ? to - 1 : to, 0, item);
  return next;
}

export function ConfigView({ onSaved }: { onSaved: () => void }) {
  const [draft, setDraft] = useState<Draft>({ tabs: [] });
  const [original, setOriginal] = useState<string>('[]');
  const [tabIdx, setTabIdx] = useState(0);
  const [catIdx, setCatIdx] = useState(0);
  const [editing, setEditing] = useState<{ kind: 'tab' | 'cat'; idx: number } | null>(null);
  const [drag, setDrag] = useState<DragState>(null);
  const [drop, setDrop] = useState<DropTarget>(null);
  const confirm = useConfirm();

  useEffect(() => {
    GetConfig().then((c) => {
      const tabs = c.tabs || [];
      setDraft({ tabs });
      setOriginal(JSON.stringify(tabs));
      setTabIdx(0);
      setCatIdx(0);
    });
  }, []);

  const dirty = JSON.stringify(draft.tabs) !== original;

  const tab = draft.tabs[tabIdx];
  const category = tab?.categories?.[catIdx];

  // -------- Tab ops --------
  const addTab = () => {
    const name = uniqueName('New Tab', draft.tabs.map((t) => t.tab_name));
    const next = { tabs: [...draft.tabs, emptyTab(name)] };
    setDraft(next);
    setTabIdx(next.tabs.length - 1);
    setCatIdx(0);
    setEditing({ kind: 'tab', idx: next.tabs.length - 1 });
  };
  const removeTab = (i: number) => {
    const next = { tabs: draft.tabs.filter((_, k) => k !== i) };
    setDraft(next);
    setTabIdx(Math.min(i, Math.max(0, next.tabs.length - 1)));
    setCatIdx(0);
  };
  const renameTab = (i: number, name: string) => {
    const next = { tabs: draft.tabs.map((t, k) => (k === i ? config.Tab.createFrom({ tab_name: name, categories: t.categories }) : t)) };
    setDraft(next);
  };
  const reorderTabs = (from: number, to: number) => {
    const newTabs = moveItem(draft.tabs, from, to);
    if (newTabs === draft.tabs) return;
    const selectedTab = draft.tabs[tabIdx];
    setDraft({ tabs: newTabs });
    const newSelectedIdx = newTabs.findIndex((t) => t === selectedTab);
    if (newSelectedIdx >= 0) setTabIdx(newSelectedIdx);
  };

  // -------- Category ops --------
  const addCategory = () => {
    if (!tab) return;
    const name = uniqueName('New Category', tab.categories.map((c) => c.name));
    const newCats = [...tab.categories, emptyCategory(name)];
    updateTab(tabIdx, { categories: newCats });
    setCatIdx(newCats.length - 1);
    setEditing({ kind: 'cat', idx: newCats.length - 1 });
  };
  const removeCategory = (i: number) => {
    if (!tab) return;
    const newCats = tab.categories.filter((_, k) => k !== i);
    updateTab(tabIdx, { categories: newCats });
    setCatIdx(Math.min(i, Math.max(0, newCats.length - 1)));
  };
  const renameCategory = (i: number, name: string) => {
    if (!tab) return;
    const newCats = tab.categories.map((c, k) =>
      k === i ? config.Category.createFrom({ name, folders: c.folders }) : c,
    );
    updateTab(tabIdx, { categories: newCats });
  };
  const reorderCategories = (from: number, to: number) => {
    if (!tab) return;
    const newCats = moveItem(tab.categories, from, to);
    if (newCats === tab.categories) return;
    const selectedCat = tab.categories[catIdx];
    updateTab(tabIdx, { categories: newCats });
    const newSelectedIdx = newCats.findIndex((c) => c === selectedCat);
    if (newSelectedIdx >= 0) setCatIdx(newSelectedIdx);
  };

  // -------- Folder ops --------
  const addFolder = async () => {
    if (!category) return;
    try {
      const dir = await PickFolder();
      if (!dir) return;
      if (category.folders.includes(dir)) return;
      const newCats = tab!.categories.map((c, k) =>
        k === catIdx ? config.Category.createFrom({ name: c.name, folders: [...c.folders, dir] }) : c,
      );
      updateTab(tabIdx, { categories: newCats });
    } catch {
      /* user cancelled */
    }
  };
  const removeFolder = (i: number) => {
    if (!category) return;
    const newCats = tab!.categories.map((c, k) =>
      k === catIdx ? config.Category.createFrom({ name: c.name, folders: c.folders.filter((_, j) => j !== i) }) : c,
    );
    updateTab(tabIdx, { categories: newCats });
  };

  const updateTab = (i: number, patch: Partial<Tab>) => {
    setDraft({
      tabs: draft.tabs.map((t, k) =>
        k === i ? config.Tab.createFrom({ tab_name: patch.tab_name ?? t.tab_name, categories: patch.categories ?? t.categories }) : t,
      ),
    });
  };

  const save = async () => {
    const cfg = config.Config.createFrom({ tabs: draft.tabs });
    await SaveConfig(cfg);
    setOriginal(JSON.stringify(draft.tabs));
    onSaved();
  };
  const revert = () => {
    const tabs = JSON.parse(original) as Tab[];
    setDraft({ tabs: tabs.map((t) => config.Tab.createFrom(t)) });
  };

  // -------- Drag handlers --------
  const onRowDragOver = (
    kind: 'tab' | 'cat',
    index: number,
    e: React.DragEvent<HTMLDivElement>,
  ) => {
    if (drag?.kind !== kind) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
    const rect = e.currentTarget.getBoundingClientRect();
    const before = e.clientY - rect.top < rect.height / 2;
    if (drop?.kind !== kind || drop.index !== index || drop.before !== before) {
      setDrop({ kind, index, before });
    }
  };
  const onRowDrop = (kind: 'tab' | 'cat', e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    if (!drag || drag.kind !== kind || !drop || drop.kind !== kind) return;
    const to = drop.before ? drop.index : drop.index + 1;
    if (kind === 'tab') reorderTabs(drag.from, to);
    else reorderCategories(drag.from, to);
    setDrag(null);
    setDrop(null);
  };
  const onDragEnd = () => { setDrag(null); setDrop(null); };

  const dropClass = (kind: 'tab' | 'cat', i: number): string => {
    if (!drop || drop.kind !== kind || drop.index !== i) return '';
    return drop.before ? 'drop-above' : 'drop-below';
  };
  const dragClass = (kind: 'tab' | 'cat', i: number): string =>
    drag && drag.kind === kind && drag.from === i ? 'dragging' : '';

  return (
    <div className="config-view">
      {/* Tabs column */}
      <div className="config-col">
        <div className="config-col-header">
          <span>Tabs</span>
          <div className="spacer" />
          <button className="icon-btn" onClick={addTab} title="Add tab" aria-label="Add tab">
            <PlusIcon />
          </button>
        </div>
        <div className="config-list">
          {draft.tabs.length === 0 && <div className="config-empty">No tabs. Click + to add one.</div>}
          {draft.tabs.map((t, i) => {
            const isEditing = editing?.kind === 'tab' && editing.idx === i;
            return (
              <div
                key={i}
                className={`config-row ${i === tabIdx ? 'active' : ''} ${dropClass('tab', i)} ${dragClass('tab', i)}`}
                draggable={!isEditing}
                onDragStart={() => { setDrag({ kind: 'tab', from: i }); setDrop(null); }}
                onDragOver={(e) => onRowDragOver('tab', i, e)}
                onDragLeave={() => { if (drop?.kind === 'tab' && drop.index === i) setDrop(null); }}
                onDrop={(e) => onRowDrop('tab', e)}
                onDragEnd={onDragEnd}
                onClick={() => { setTabIdx(i); setCatIdx(0); setEditing(null); }}
              >
                {isEditing ? (
                  <input
                    autoFocus
                    defaultValue={t.tab_name}
                    onBlur={(e) => { renameTab(i, e.target.value || t.tab_name); setEditing(null); }}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') (e.target as HTMLInputElement).blur();
                      if (e.key === 'Escape') setEditing(null);
                    }}
                    onClick={(e) => e.stopPropagation()}
                  />
                ) : (
                  <span className="label">{t.tab_name}</span>
                )}
                <button
                  className="icon-btn"
                  draggable={false}
                  onDragStart={(e) => e.preventDefault()}
                  onClick={(e) => { e.stopPropagation(); setEditing({ kind: 'tab', idx: i }); }}
                  title="Rename"
                  aria-label="Rename"
                >
                  <PencilIcon size={14} />
                </button>
                <button
                  className="icon-btn"
                  draggable={false}
                  onDragStart={(e) => e.preventDefault()}
                  onClick={async (e) => {
                    e.stopPropagation();
                    if (await confirm({
                      title: 'Delete tab',
                      message: `Delete tab "${t.tab_name}" and all its categories?`,
                      confirmLabel: 'Delete',
                      danger: true,
                    })) removeTab(i);
                  }}
                  title="Delete"
                  aria-label="Delete"
                >
                  <TrashIcon size={14} />
                </button>
              </div>
            );
          })}
        </div>
      </div>

      {/* Categories column */}
      <div className="config-col">
        <div className="config-col-header">
          <span>Categories</span>
          <div className="spacer" />
          <button
            className="icon-btn"
            onClick={addCategory}
            disabled={!tab}
            title="Add category"
            aria-label="Add category"
          >
            <PlusIcon />
          </button>
        </div>
        <div className="config-list">
          {!tab && <div className="config-empty">Select a tab</div>}
          {tab && tab.categories.length === 0 && (
            <div className="config-empty">No categories yet</div>
          )}
          {tab && tab.categories.map((c, i) => {
            const isEditing = editing?.kind === 'cat' && editing.idx === i;
            return (
              <div
                key={i}
                className={`config-row ${i === catIdx ? 'active' : ''} ${dropClass('cat', i)} ${dragClass('cat', i)}`}
                draggable={!isEditing}
                onDragStart={() => { setDrag({ kind: 'cat', from: i }); setDrop(null); }}
                onDragOver={(e) => onRowDragOver('cat', i, e)}
                onDragLeave={() => { if (drop?.kind === 'cat' && drop.index === i) setDrop(null); }}
                onDrop={(e) => onRowDrop('cat', e)}
                onDragEnd={onDragEnd}
                onClick={() => { setCatIdx(i); setEditing(null); }}
              >
                {isEditing ? (
                  <input
                    autoFocus
                    defaultValue={c.name}
                    onBlur={(e) => { renameCategory(i, e.target.value || c.name); setEditing(null); }}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') (e.target as HTMLInputElement).blur();
                      if (e.key === 'Escape') setEditing(null);
                    }}
                    onClick={(e) => e.stopPropagation()}
                  />
                ) : (
                  <span className="label">{c.name}</span>
                )}
                <button
                  className="icon-btn"
                  draggable={false}
                  onDragStart={(e) => e.preventDefault()}
                  onClick={(e) => { e.stopPropagation(); setEditing({ kind: 'cat', idx: i }); }}
                  title="Rename"
                  aria-label="Rename"
                >
                  <PencilIcon size={14} />
                </button>
                <button
                  className="icon-btn"
                  draggable={false}
                  onDragStart={(e) => e.preventDefault()}
                  onClick={async (e) => {
                    e.stopPropagation();
                    if (await confirm({
                      title: 'Delete category',
                      message: `Delete category "${c.name}" and its folder list?`,
                      confirmLabel: 'Delete',
                      danger: true,
                    })) removeCategory(i);
                  }}
                  title="Delete"
                  aria-label="Delete"
                >
                  <TrashIcon size={14} />
                </button>
              </div>
            );
          })}
        </div>
      </div>

      {/* Folders column */}
      <div className="config-col">
        <div className="config-col-header">
          <span>Folders {category ? `in "${category.name}"` : ''}</span>
          <div className="spacer" />
          <button
            className="icon-btn"
            onClick={addFolder}
            disabled={!category}
            title="Add folder"
            aria-label="Add folder"
          >
            <PlusIcon />
          </button>
        </div>
        <div className="config-list">
          {!category && <div className="config-empty">Select a category</div>}
          {category && category.folders.length === 0 && (
            <div className="config-empty">No folders. Click + to pick one.</div>
          )}
          {category && category.folders.map((f, i) => (
            <div key={i} className="config-row">
              <span className="label folder-path" title={f}>{f}</span>
              <button
                className="icon-btn"
                onClick={() => removeFolder(i)}
                title="Remove"
                aria-label="Remove"
              >
                <TrashIcon size={14} />
              </button>
            </div>
          ))}
        </div>
        <div className="config-footer">
          {dirty && <span className="dirty-badge">Unsaved changes</span>}
          <button onClick={revert} disabled={!dirty}>Revert</button>
          <button className="primary" onClick={save} disabled={!dirty}>Save</button>
        </div>
      </div>
    </div>
  );
}

function uniqueName(base: string, existing: string[]): string {
  if (!existing.includes(base)) return base;
  let i = 2;
  while (existing.includes(`${base} ${i}`)) i++;
  return `${base} ${i}`;
}
