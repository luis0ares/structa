import { main } from '../../wailsjs/go/models';
import { OpenFolder, OpenURL, ToggleFavorite } from '../../wailsjs/go/main/App';
import { FolderIcon, GlobeIcon, PencilIcon, StarIcon } from './icons';

export type ViewMode = 'grid' | 'list';

type Props = {
  item: main.ItemCardDTO;
  viewMode: ViewMode;
  selectedTagKeys: Set<string>;
  onFavoriteChange: (id: number, favorite: boolean) => void;
  onOpenPreview: (item: main.ItemCardDTO) => void;
  onTagClick: (tag: string) => void;
  onEditMeta: (item: main.ItemCardDTO) => void;
};

export function Card({ item, viewMode, selectedTagKeys, onFavoriteChange, onOpenPreview, onTagClick, onEditMeta }: Props) {
  const handleStar = (e: React.MouseEvent) => {
    e.stopPropagation();
    ToggleFavorite(item.id).then((fav) => onFavoriteChange(item.id, fav));
  };
  const handleGlobe = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (item.sourceLink) OpenURL(item.sourceLink);
  };
  const handleFolder = (e: React.MouseEvent) => {
    e.stopPropagation();
    OpenFolder(item.id);
  };
  const handleEdit = (e: React.MouseEvent) => {
    e.stopPropagation();
    onEditMeta(item);
  };
  const handleImg = () => onOpenPreview(item);

  return (
    <div className={`card ${viewMode === 'list' ? 'card-list' : ''}`} id={`card-${item.id}`}>
      <div className="card-img" onClick={handleImg}>
        {item.thumbUrl ? (
          <img src={item.thumbUrl} alt={item.title} loading="lazy" />
        ) : (
          <span className="ph">No preview</span>
        )}
      </div>
      <div className="card-body">
        <div className="card-title">{item.title}</div>
        {item.tags && item.tags.length > 0 && (
          <div className="card-tags">
            {item.tags.sort().map((t) => {
              const active = selectedTagKeys.has(t.toLowerCase());
              return (
                <button
                  key={t}
                  type="button"
                  className={`tag-chip ${active ? 'active' : ''}`}
                  onClick={(e) => { e.stopPropagation(); onTagClick(t); }}
                  title={active ? `Remove "${t}" filter` : `Filter by "${t}"`}
                >
                  {t}
                </button>
              );
            })}
          </div>
        )}
        {item.description && (
          <div className="card-content" title={item.description}>{item.description}</div>
        )}
      </div>
      <div className="card-actions">
        <button
          className={`icon-btn star ${item.favorite ? 'on' : ''}`}
          onClick={handleStar}
          aria-label={item.favorite ? 'Unfavorite' : 'Favorite'}
          title={item.favorite ? 'Unfavorite' : 'Favorite'}
        >
          <StarIcon filled={item.favorite} />
        </button>
        {item.sourceLink && (
          <button
            className="icon-btn"
            onClick={handleGlobe}
            aria-label="Open source link"
            title={item.sourceLink}
          >
            <GlobeIcon />
          </button>
        )}
        <button
          className="icon-btn"
          onClick={handleEdit}
          aria-label="Edit metadata"
          title="Edit metadata"
        >
          <PencilIcon />
        </button>
        <div className="spacer" />
        <button
          className="icon-btn"
          onClick={handleFolder}
          aria-label="Open folder"
          title="Open folder in Explorer"
        >
          <FolderIcon />
        </button>
      </div>
    </div>
  );
}
