import { useEffect, useRef, useState } from 'react';
import { main } from '../../wailsjs/go/models';
import { UpdateItemMeta } from '../../wailsjs/go/main/App';
import { CloseIcon, PlusIcon } from './icons';

type Props = {
  item: main.ItemCardDTO;
  onClose: () => void;
  onSaved: () => void;
};

export function MetadataEditorModal({ item, onClose, onSaved }: Props) {
  const [name, setName] = useState(item.title || '');
  const [tags, setTags] = useState<string[]>(item.tags || []);
  const [link, setLink] = useState(item.sourceLink || '');
  const [description, setDescription] = useState(item.description || '');
  const [favorite, setFavorite] = useState(item.favorite);
  const [hidden, setHidden] = useState(item.hidden);
  const [newTag, setNewTag] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const nameRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    nameRef.current?.focus();
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', handleKey);
    return () => window.removeEventListener('keydown', handleKey);
  }, [onClose]);

  const addTag = () => {
    const t = newTag.trim();
    if (t && !tags.some((x) => x.toLowerCase() === t.toLowerCase())) {
      setTags([...tags, t]);
    }
    setNewTag('');
  };

  const removeTag = (index: number) => {
    setTags(tags.filter((_, i) => i !== index));
  };

  const handleSave = async () => {
    setSaving(true);
    setError('');
    try {
      await UpdateItemMeta(item.id, name || item.folderName, tags.sort(), description, link, favorite, hidden);
      onSaved();
      onClose();
    } catch (e) {
      setError(String(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="modal-backdrop">
      <div className="editor-card" role="dialog" aria-modal="true" aria-label="Edit metadata">
        <div className="editor-header">
          <h3>Edit Item</h3>
          <button className="icon-btn" onClick={onClose} aria-label="Close" title="Close">
            <CloseIcon />
          </button>
        </div>
        <div className="editor-body">
          <label className="editor-label">
            Name
            <input
              ref={nameRef}
              type="text"
              className="editor-input"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={item.folderName}
            />
          </label>
          <div className="editor-label">
            Tags
            <div className="editor-tags">
              {tags.sort().map((t, i) => (
                <span key={i} className="tag-chip active editor-tag-chip">
                  {t}
                  <button type="button" className="tag-remove" onClick={() => removeTag(i)} aria-label={`Remove "${t}"`} title={`Remove "${t}"`}>
                    <CloseIcon size={10} />
                  </button>
                </span>
              ))}
            </div>
            <div className="editor-tag-add">
              <input
                type="text"
                className="editor-input"
                value={newTag}
                onChange={(e) => setNewTag(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addTag(); } }}
                placeholder="Add tag..."
              />
              <button type="button" className="icon-btn" onClick={addTag} aria-label="Add tag" title="Add tag">
                <PlusIcon />
              </button>
            </div>
          </div>
          <label className="editor-label">
            Link
            <input
              type="url"
              className="editor-input"
              value={link}
              onChange={(e) => setLink(e.target.value)}
              placeholder="https://..."
            />
          </label>
          <label className="editor-label">
            Description
            <textarea
              className="editor-input editor-textarea"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={4}
            />
          </label>
          <label className="editor-toggle-row">
            <input
              type="checkbox"
              checked={favorite}
              onChange={(e) => setFavorite(e.target.checked)}
            />
            Favorite
          </label>
          <label className="editor-toggle-row">
            <input
              type="checkbox"
              checked={hidden}
              onChange={(e) => setHidden(e.target.checked)}
            />
            Hidden
          </label>
        </div>
        {error && <div className="editor-error">{error}</div>}
        <div className="editor-actions">
          <button onClick={onClose}>Cancel</button>
          <button className="primary" onClick={handleSave} disabled={saving}>
            {saving ? 'Saving...' : 'Save'}
          </button>
        </div>
      </div>
    </div>
  );
}
