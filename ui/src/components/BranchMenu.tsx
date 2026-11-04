import * as Menu from "@radix-ui/react-dropdown-menu";
import { Link } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";
import { type BranchPlace, placeLine } from "../app/repos";
import { Chevron } from "./icons";
import { CopyButton } from "./ui";

// A branch as a menu of the repos that have it (the Session's header,
// wiki/product/features/sessions.md#agent-tabs): each opens that repo's page on
// the branch; the button on its left copies the name. A mouse opens it by hovering (the CEO,
// 2026-11-04); a click is for touch, and Enter or Space for the keyboard.

const item =
  "flex cursor-pointer items-center gap-3 rounded px-2 py-1 text-[13px] text-tx2 outline-none data-[highlighted]:bg-hover data-[highlighted]:text-tx";

/** How long the pointer may be off the trigger and the menu before it closes:
 * the gap between them is crossed without it closing. */
const grace = 200;

export function BranchMenu({
  project,
  branch,
  places,
}: {
  project: string;
  branch: string;
  places: { changed: BranchPlace[]; unchanged: BranchPlace[] };
}) {
  const [open, setOpen] = useState(false);
  const closing = useRef<number>(undefined);
  const hold = () => {
    window.clearTimeout(closing.current);
    setOpen(true);
  };
  const leave = () => {
    closing.current = window.setTimeout(() => setOpen(false), grace);
  };
  // Unmounted while closing (another agent shown): no timer left behind.
  useEffect(() => {
    return () => window.clearTimeout(closing.current);
  }, []);
  const mouse = (run: () => void) => (e: React.PointerEvent) => e.pointerType === "mouse" && run();
  return (
    <span className="flex min-w-0 items-stretch">
      <CopyButton
        text={() => branch}
        label="Copy the branch name"
        className="w-6 rounded-r-none border border-r-0 border-line2"
      />
      <Menu.Root open={open} onOpenChange={setOpen} modal={false}>
        <Menu.Trigger
          onPointerEnter={mouse(hold)}
          onPointerLeave={mouse(leave)}
          // Hovering opened it: a click mustn't close it again.
          onPointerDown={(e) => e.pointerType === "mouse" && e.preventDefault()}
          className="flex min-w-0 items-center gap-1 rounded-md rounded-l-none border border-line2 px-2 py-0.5 font-mono text-[11.5px] text-tx3 outline-none hover:text-tx focus-visible:outline-2 focus-visible:outline-acc data-[state=open]:bg-hover data-[state=open]:text-tx"
        >
          <span className="truncate">{branch}</span>
          <span className="rotate-90">
            <Chevron open={false} />
          </span>
        </Menu.Trigger>
        <Menu.Portal>
          <Menu.Content
            align="start"
            sideOffset={4}
            onPointerEnter={mouse(hold)}
            onPointerLeave={mouse(leave)}
            className="z-50 flex min-w-56 flex-col rounded-lg border border-line2 bg-panel p-1 shadow-overlay"
          >
            {places.changed.map((p) => (
              <Place key={p.repo} project={project} branch={branch} place={p} />
            ))}
            {places.changed.length > 0 && places.unchanged.length > 0 && (
              <Menu.Separator className="my-1 h-px bg-line" />
            )}
            {places.unchanged.map((p) => (
              <Place key={p.repo} project={project} branch={branch} place={p} />
            ))}
          </Menu.Content>
        </Menu.Portal>
      </Menu.Root>
    </span>
  );
}

/** A repo that has the branch: opens its page on it. */
function Place({
  project,
  branch,
  place: p,
}: {
  project: string;
  branch: string;
  place: BranchPlace;
}) {
  return (
    <Menu.Item asChild>
      <Link
        to="/projects/$project/repos/$repo"
        params={{ project, repo: p.repo }}
        search={{ branch }}
        className={item}
      >
        <span className="min-w-0 flex-1 truncate">{p.repo}</span>
        <span className="shrink-0 text-[12px] text-tx4">{placeLine(p)}</span>
      </Link>
    </Menu.Item>
  );
}
