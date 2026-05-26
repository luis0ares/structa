import { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react';

type ConfirmOptions = {
  title?: string;
  message: string;
  confirmLabel?: string;
  cancelLabel?: string;
  danger?: boolean;
};

type ConfirmFn = (opts: ConfirmOptions) => Promise<boolean>;

const ConfirmContext = createContext<ConfirmFn>(() => Promise.resolve(false));

export function useConfirm(): ConfirmFn {
  return useContext(ConfirmContext);
}

export function ConfirmProvider({ children }: { children: React.ReactNode }) {
  const [opts, setOpts] = useState<ConfirmOptions | null>(null);
  const resolverRef = useRef<((v: boolean) => void) | null>(null);
  const confirmBtnRef = useRef<HTMLButtonElement | null>(null);

  const request: ConfirmFn = useCallback(
    (o) =>
      new Promise<boolean>((resolve) => {
        resolverRef.current = resolve;
        setOpts(o);
      }),
    [],
  );

  const close = (value: boolean) => {
    resolverRef.current?.(value);
    resolverRef.current = null;
    setOpts(null);
  };

  useEffect(() => {
    if (!opts) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        close(false);
      } else if (e.key === 'Enter') {
        e.preventDefault();
        close(true);
      }
    };
    window.addEventListener('keydown', onKey);
    // Focus the confirm button on open.
    setTimeout(() => confirmBtnRef.current?.focus(), 0);
    return () => window.removeEventListener('keydown', onKey);
  }, [opts]);

  return (
    <ConfirmContext.Provider value={request}>
      {children}
      {opts && (
        <div
          className="modal-backdrop"
          onClick={() => close(false)}
          role="presentation"
        >
          <div
            className="confirm-card"
            role="dialog"
            aria-modal="true"
            aria-labelledby={opts.title ? 'confirm-title' : undefined}
            onClick={(e) => e.stopPropagation()}
          >
            {opts.title && (
              <h3 id="confirm-title" className="confirm-title">{opts.title}</h3>
            )}
            <p className="confirm-message">{opts.message}</p>
            <div className="confirm-actions">
              <button onClick={() => close(false)}>
                {opts.cancelLabel ?? 'Cancel'}
              </button>
              <button
                ref={confirmBtnRef}
                className={opts.danger ? 'danger-primary' : 'primary'}
                onClick={() => close(true)}
              >
                {opts.confirmLabel ?? 'OK'}
              </button>
            </div>
          </div>
        </div>
      )}
    </ConfirmContext.Provider>
  );
}
