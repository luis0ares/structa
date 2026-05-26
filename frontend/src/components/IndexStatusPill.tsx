import { indexer } from '../../wailsjs/go/models';

export function IndexStatusPill({ status }: { status: indexer.Status }) {
  const scanning = status.scanning;
  const label = scanning
    ? (status.currentPath ? `Indexing ${shortPath(status.currentPath)}…` : 'Indexing…')
    : 'Up to date';
  return (
    <div className={`status-pill ${scanning ? 'scanning' : ''}`} title={status.currentPath || ''}>
      <span className="dot" />
      <span>{label}</span>
    </div>
  );
}

function shortPath(p: string): string {
  const parts = p.replace(/\\/g, '/').split('/').filter(Boolean);
  return parts.slice(-2).join('/');
}
