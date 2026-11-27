import { useEffect, useRef, useState } from "react";

// A menu a mouse opens by resting on its trigger (the CEO, 2026-11-04): the
// session's branch menu and the rail's project popover share it, so both feel the same.

/** How long the pointer rests on the trigger before it opens: passing over it on the way to
 * something else opens nothing (the CEO, 2026-11-05). */
const delay = 300;
/** How long the pointer may be off the trigger and the menu before it closes:
 * the gap between them is crossed without it closing. */
const grace = 200;

const mouse = (run: () => void) => (e: React.PointerEvent) => e.pointerType === "mouse" && run();

/** The open state, `show` to open at once, and the pointer handlers to spread on
 * the trigger and on the menu's content. */
export function useHoverMenu() {
  const [open, setOpen] = useState(false);
  // One timer, opening or closing: whichever the pointer last asked for.
  const timer = useRef<number>(undefined);
  const later = (to: boolean, ms: number) => {
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => setOpen(to), ms);
  };
  const show = () => {
    window.clearTimeout(timer.current);
    setOpen(true);
  };
  const rest = () => (open ? show() : later(true, delay));
  const leave = () => (open ? later(false, grace) : window.clearTimeout(timer.current));
  // Unmounted while a timer runs: none left behind.
  useEffect(() => {
    return () => window.clearTimeout(timer.current);
  }, []);
  return {
    open,
    setOpen,
    show,
    trigger: { onPointerEnter: mouse(rest), onPointerLeave: mouse(leave) },
    content: { onPointerEnter: mouse(show), onPointerLeave: mouse(leave) },
  };
}
