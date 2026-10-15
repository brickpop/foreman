import { createContext, type ReactNode, useCallback, useContext, useState } from "react";

// In-app toasts (wiki/product/features/app-shell.md#toasts): top right,
// dismissed after 6.5 s, on click (which goes where the toast is about), or
// with its × (which goes nowhere).

export type Toast = {
  title: string;
  body: string;
  /** The dot's color (a token). */
  color: string;
  /** Where a click goes. */
  onOpen?: () => void;
};

type Shown = Toast & { id: number };

const ToastContext = createContext<(t: Toast) => void>(() => {});

/** Shows a toast. */
export function useToast(): (t: Toast) => void {
  return useContext(ToastContext);
}

const lifetime = 6500;

export function Toasts({ children }: { children: ReactNode }) {
  const [shown, setShown] = useState<Shown[]>([]);
  const dismiss = useCallback(
    (id: number) => setShown((all) => all.filter((t) => t.id !== id)),
    [],
  );
  const show = useCallback(
    (t: Toast) => {
      const id = Date.now() + Math.random();
      setShown((all) => [...all, { ...t, id }]);
      setTimeout(() => dismiss(id), lifetime);
    },
    [dismiss],
  );
  return (
    <ToastContext.Provider value={show}>
      {children}
      <div
        // pointer-events-auto: an open dialog turns pointer events off everywhere else.
        className="pointer-events-auto fixed top-5 right-5 z-[60] flex w-80 flex-col gap-2"
        aria-live="polite"
        role="status"
      >
        {shown.map((t) => (
          <div
            key={t.id}
            className="flex items-start rounded-[10px] border border-line3 bg-panel shadow-overlay"
          >
            <button
              type="button"
              onClick={() => {
                dismiss(t.id);
                t.onOpen?.();
              }}
              className="flex min-w-0 flex-1 gap-2.5 py-3 pl-3.5 text-left"
            >
              <span
                className="mt-1.5 size-2 shrink-0 rounded-full"
                style={{ background: t.color }}
              />
              <span className="flex min-w-0 flex-col gap-0.5">
                <span className="text-[13.5px] font-semibold">{t.title}</span>
                <span className="text-[13px] leading-[1.4] text-tx2">{t.body}</span>
              </span>
            </button>
            {/* Dismisses without going anywhere. */}
            <button
              type="button"
              aria-label="Dismiss"
              onClick={() => dismiss(t.id)}
              className="px-3 py-2.5 text-[16px] leading-none text-tx4 hover:text-tx"
            >
              ×
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}
