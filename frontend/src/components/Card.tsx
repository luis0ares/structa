import { main } from '../../wailsjs/go/models';
import { OpenFolder, OpenURL, ToggleFavorite } from '../../wailsjs/go/main/App';
import { FolderIcon, GlobeIcon, StarIcon } from './icons';

export type ViewMode = 'grid' | 'list';

type Props = {
  mod: main.ModCardDTO;
  viewMode: ViewMode;
  selectedTagKeys: Set<string>;
  onFavoriteChange: (id: number, favorite: boolean) => void;
  onOpenPreview: (mod: main.ModCardDTO) => void;
  onTagClick: (tag: string) => void;
};

export function Card({ mod, viewMode, selectedTagKeys, onFavoriteChange, onOpenPreview, onTagClick }: Props) {
  const handleStar = (e: React.MouseEvent) => {
    e.stopPropagation();
    ToggleFavorite(mod.id).then((fav) => onFavoriteChange(mod.id, fav));
  };
  const handleGlobe = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (mod.sourceLink) OpenURL(mod.sourceLink);
  };
  const handleFolder = (e: React.MouseEvent) => {
    e.stopPropagation();
    OpenFolder(mod.id);
  };
  const handleImg = () => onOpenPreview(mod);

  return (
    <div className={`card ${viewMode === 'list' ? 'card-list' : ''}`} id={`card-${mod.id}`}>
      <div className="card-img" onClick={handleImg}>
        {mod.thumbUrl ? (
          <img src={mod.thumbUrl} alt={mod.title} loading="lazy" />
        ) : (
          <span className="ph">No preview</span>
        )}
      </div>
      <div className="card-body">
        <div className="card-title">{mod.title}</div>
        {mod.tags && mod.tags.length > 0 && (
          <div className="card-tags">
            {mod.tags.map((t) => {
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
        {mod.description && (
          <div className="card-content" title={mod.description}>{mod.description}</div>
        )}
      </div>
      <div className="card-actions">
        <button
          className={`icon-btn star ${mod.favorite ? 'on' : ''}`}
          onClick={handleStar}
          aria-label={mod.favorite ? 'Unfavorite' : 'Favorite'}
          title={mod.favorite ? 'Unfavorite' : 'Favorite'}
        >
          <StarIcon filled={mod.favorite} />
        </button>
        {mod.sourceLink && (
          <button
            className="icon-btn"
            onClick={handleGlobe}
            aria-label="Open source link"
            title={mod.sourceLink}
          >
            <GlobeIcon />
          </button>
        )}
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
