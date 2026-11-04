import * as Dialog from "@radix-ui/react-dialog";
import type { ReactNode } from "react";
import { Button } from "./ui";

// Modals and drawers (wiki/product/features/app-shell.md#overlays), over Radix
// Dialog: focus trap, Escape closes the topmost, the backdrop closes a modal.

/** A centered modal (Deploy, Add project). */
export function Modal({
  open,
  onClose,
  title,
  children,
  width = 520,
  buttonsOnly = false,
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  /** Wider for a terminal; 520 by default. */
  width?: number;
  /** Closes through its own buttons only: a terminal inside needs Escape, and a
   * stray click outside mustn't end what runs in it. */
  buttonsOnly?: boolean;
}) {
  return (
    <Dialog.Root open={open} onOpenChange={(o) => !o && onClose()}>
      <Dialog.Portal>
        <Dialog.Overlay
          className="fixed inset-0 z-50 grid place-items-center p-5"
          style={{ background: "rgba(5,6,7,0.55)" }}
        >
          <Dialog.Content
            aria-describedby={undefined}
            onEscapeKeyDown={(e) => buttonsOnly && e.preventDefault()}
            onPointerDownOutside={(e) => buttonsOnly && e.preventDefault()}
            style={{ width: `min(${width}px, 100%)` }}
            className="max-h-[calc(100dvh-40px)] overflow-y-auto rounded-[14px] border border-line bg-panel p-6 shadow-overlay focus:outline-none"
          >
            <div className="mb-4 flex items-start justify-between gap-3">
              <Dialog.Title className="m-0 min-w-0 text-xl font-semibold">{title}</Dialog.Title>
              <Dialog.Close
                aria-label="Close"
                className="-mt-1 -mr-2 grid size-8 shrink-0 place-items-center rounded-md text-[20px] text-tx3 hover:bg-hover hover:text-tx focus-visible:outline-2 focus-visible:outline-acc"
              >
                ×
              </Dialog.Close>
            </div>
            {children}
          </Dialog.Content>
        </Dialog.Overlay>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

/** A drawer from the right (the board's side sheet, layer 44). It overlays, doesn't push. */
export function Drawer({
  open,
  onClose,
  title,
  width,
  children,
  keepOpenOn,
}: {
  open: boolean;
  onClose: () => void;
  title: ReactNode;
  width: number;
  children: ReactNode;
  /** A selector for what a click outside may land on without closing it: on
   * the board, another card, which the drawer then shows. */
  keepOpenOn?: string;
}) {
  return (
    <Dialog.Root open={open} onOpenChange={(o) => !o && onClose()} modal={false}>
      <Dialog.Portal>
        <Dialog.Content
          aria-describedby={undefined}
          onInteractOutside={(e) => {
            const at = e.target instanceof Element ? e.target : null;
            if (keepOpenOn && at?.closest(keepOpenOn)) e.preventDefault();
          }}
          className="fixed top-0 right-0 flex h-full flex-col border-l border-line2 bg-panel shadow-overlay focus:outline-none"
          style={{ width: `min(${width}px, 100vw)`, zIndex: 44 }}
        >
          <header className="flex items-center justify-between gap-3 border-b border-line px-5 py-4">
            <Dialog.Title className="min-w-0 text-[15px] font-semibold">{title}</Dialog.Title>
            <Dialog.Close
              aria-label="Close"
              className="text-[18px] text-tx3 hover:text-tx focus-visible:outline-2 focus-visible:outline-acc"
            >
              ×
            </Dialog.Close>
          </header>
          <div className="min-h-0 flex-1 overflow-auto">{children}</div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

/** Asks before a setting that weakens safety, or before deleting. */
export function Confirm({
  open,
  title,
  body,
  action,
  onConfirm,
  onClose,
  children,
}: {
  open: boolean;
  title: string;
  body: string;
  action: string;
  onConfirm: () => void;
  onClose: () => void;
  /** Options of the action, under the body (Remove project: also delete its repo). */
  children?: ReactNode;
}) {
  return (
    <Modal open={open} onClose={onClose} title={title}>
      <div className="flex flex-col gap-4">
        <p className="m-0 text-tx2">{body}</p>
        {children}
        <div className="flex justify-end gap-2">
          <Button onClick={onClose}>Cancel</Button>
          <Button
            variant="danger"
            onClick={() => {
              onConfirm();
              onClose();
            }}
          >
            {action}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
