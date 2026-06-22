import { main } from '../wailsjs/go/models';

// Sorting applied globally across the catalog grid and the sidebar tree so both
// always present items in the same order.
export type SortMode =
  | 'name-asc'
  | 'name-desc'
  | 'created-desc'
  | 'created-asc'
  | 'modified-desc'
  | 'modified-asc';

export const DEFAULT_SORT: SortMode = 'name-asc';

export const SORT_OPTIONS: { value: SortMode; label: string }[] = [
  { value: 'name-asc', label: 'Name (A→Z)' },
  { value: 'name-desc', label: 'Name (Z→A)' },
  { value: 'created-desc', label: 'Created (newest)' },
  { value: 'created-asc', label: 'Created (oldest)' },
  { value: 'modified-desc', label: 'Modified (newest)' },
  { value: 'modified-asc', label: 'Modified (oldest)' },
];

// Returns a new array sorted according to the given mode. Name sorts are
// case-insensitive; date sorts use the folder timestamps from the backend.
export function sortItems(items: main.ItemCardDTO[], mode: SortMode): main.ItemCardDTO[] {
  const out = [...items];
  switch (mode) {
    case 'name-asc':
      out.sort((a, b) => a.title.localeCompare(b.title, undefined, { sensitivity: 'base' }));
      break;
    case 'name-desc':
      out.sort((a, b) => b.title.localeCompare(a.title, undefined, { sensitivity: 'base' }));
      break;
    case 'created-desc':
      out.sort((a, b) => b.ctime - a.ctime);
      break;
    case 'created-asc':
      out.sort((a, b) => a.ctime - b.ctime);
      break;
    case 'modified-desc':
      out.sort((a, b) => b.mtime - a.mtime);
      break;
    case 'modified-asc':
      out.sort((a, b) => a.mtime - b.mtime);
      break;
  }
  return out;
}
