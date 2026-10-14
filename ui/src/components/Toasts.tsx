import { createContext, type ReactNode, useCallback, useContext, useState } from "react";

// In-app toasts (wiki/product/features/app-shell.md#toasts): top right,
// dismissed after 6.5 s or on click; a click goes where the toast is about.

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
        className="fixed top-5 right-5 z-[60] flex w-80 flex-col gap-2"
        aria-live="polite"
        role="status"
      >
        {shown.map((t) => (
          <button
            key={t.id}
            type="button"
            onClick={() => {
              dismiss(t.id);
              t.onOpen?.();
            }}
            className="flex gap-2.5 rounded-[10px] border border-line3 bg-panel px-3.5 py-3 text-left shadow-overlay"
          >
            <span className="mt-1.5 size-2 shrink-0 rounded-full" style={{ background: t.color }} />
            <span className="flex flex-col gap-0.5">
              <span className="text-[13.5px] font-semibold">{t.title}</span>
              <span className="text-[13px] leading-[1.4] text-tx2">{t.body}</span>
            </span>
          </button>
        ))}
      </div>
    </ToastContext.Provider>
  );
}
