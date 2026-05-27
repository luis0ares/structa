import { useEffect, useState } from 'react';
import { GetPreviews } from '../../wailsjs/go/main/App';
import { ArrowLeftIcon, ArrowRightIcon, CloseIcon } from './icons';

type Props = {
  itemId: number | null;
  fallbackUrl?: string;
  onClose: () => void;
};

export function PreviewModal({ itemId, fallbackUrl, onClose }: Props) {
  const [previews, setPreviews] = useState<string[]>([]);
  const [idx, setIdx] = useState(0);

  useEffect(() => {
    if (itemId == null) return;
    GetPreviews(itemId)
      .then((urls) => {
        const list = urls && urls.length ? urls : fallbackUrl ? [fallbackUrl] : [];
        setPreviews(list);
        setIdx(0);
      })
      .catch(() => {
        setPreviews(fallbackUrl ? [fallbackUrl] : []);
        setIdx(0);
      });
  }, [itemId, fallbackUrl]);

  useEffect(() => {
    if (itemId == null) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
      else if (e.key === 'ArrowRight' || e.key.toLowerCase() === 'd') step(1);
      else if (e.key === 'ArrowLeft' || e.key.toLowerCase() === 'a') step(-1);
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [previews, itemId]);

  const step = (d: number) => {
    if (!previews.length) return;
    setIdx((i) => (i + d + previews.length) % previews.length);
  };

  if (itemId == null) return null;

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <button className="modal-btn close" onClick={onClose} aria-label="Close">
        <CloseIcon />
      </button>
      {previews.length > 1 && (
        <>
          <button
            className="modal-btn prev"
            onClick={(e) => { e.stopPropagation(); step(-1); }}
            aria-label="Previous"
          >
            <ArrowLeftIcon />
          </button>
          <button
            className="modal-btn next"
            onClick={(e) => { e.stopPropagation(); step(1); }}
            aria-label="Next"
          >
            <ArrowRightIcon />
          </button>
        </>
      )}
      <div className="modal-img-wrap" onClick={(e) => e.stopPropagation()}>
        {previews[idx] ? (
          <img src={previews[idx]} alt="" />
        ) : (
          <div style={{ color: 'white' }}>No previews</div>
        )}
      </div>
      {previews.length > 1 && (
        <div className="modal-counter">{idx + 1} / {previews.length}</div>
      )}
    </div>
  );
}
