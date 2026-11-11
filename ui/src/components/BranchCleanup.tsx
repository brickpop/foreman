import { useState } from "react";
import type { CleanupBranch } from "../api/client";
import { plural } from "../app/model";
import { useCleanupBranches, useDeleteBranches } from "../app/queries";
import { type CleanupGroup, cleanupGroups, cleanupPick } from "../app/repos";
import { Chevron } from "./icons";
import { Modal } from "./Overlays";
import { useToast } from "./Toasts";
import { Button, Empty, Spinner } from "./ui";

// A code repo's branch cleanup (wiki/product/features/repos.md#branch-cleanup): its branches in
// groups (the agents', the CEO's, those in stray worktrees), each ticked or unticked at once; the
// agents' merged and superseded ones ticked at first. One that can't go is listed apart, with why.
// Foreman keeps a copy of each deleted branch for 30 days.

export function BranchCleanup({ project, repo }: { project: string; repo: string }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button className="ml-auto" onClick={() => setOpen(true)}>
        Clean up branches
      </Button>
      {open && <CleanupDialog project={project} repo={repo} onClose={() => setOpen(false)} />}
    </>
  );
}

function CleanupDialog({
  project,
  repo,
  onClose,
}: {
  project: string;
  repo: string;
  onClose: () => void;
}) {
  const branches = useCleanupBranches(project, repo);
  const remove = useDeleteBranches(project, repo);
  const toast = useToast();
  const { groups, staying } = cleanupGroups(branches.data ?? []);
  // Undefined until the CEO ticks something: the groups on at first.
  const [ticked, setTicked] = useState<Set<string>>();
  const pick = ticked ?? cleanupPick(groups);
  const tick = (names: string[], on: boolean) => {
    const next = new Set(pick);
    for (const n of names) {
      if (on) next.add(n);
      else next.delete(n);
    }
    setTicked(next);
  };
  const run = () =>
    remove.mutate([...pick], {
      onSuccess: (done) => {
        toast({
          title: `${plural(done.removed.length, "branch", "branches")} deleted`,
          body:
            done.kept.length > 0
              ? `Kept: ${done.kept.map((k) => `${k.branch} (${k.why})`).join(", ")}.`
              : "Foreman keeps a copy of each for 30 days.",
          color: done.kept.length > 0 ? "var(--attn)" : "var(--ok)",
        });
        onClose();
      },
      onError: (e) => toast({ title: "Not deleted", body: e.message, color: "var(--err)" }),
    });
  const section = (s: CleanupGroup["section"]) => groups.filter((g) => g.section === s);
  return (
    <Modal open onClose={onClose} title="Clean up branches" width={640}>
      <div className="flex flex-col gap-4">
        <p className="m-0 text-tx2">
          The branches of {repo} that can go. Deleting a merged or superseded one loses nothing.
          Foreman keeps a copy of each for 30 days.
        </p>
        {branches.isError && <p className="m-0 text-err">{branches.error.message}</p>}
        {branches.isPending && (
          <p className="m-0 flex items-center gap-2.5 text-tx3">
            <Spinner />
            Reading the branches of {repo}: where each one's work stands…
          </p>
        )}
        {branches.data && branches.data.length === 0 && <Empty>No branches to clean up.</Empty>}
        {branches.data && branches.data.length > 0 && (
          <div className="flex max-h-[55vh] flex-col gap-4 overflow-auto">
            <Section
              title="Agents' branches"
              groups={section("agents")}
              pick={pick}
              onTick={tick}
            />
            <Section
              title="Your branches"
              note="Deleted here only: a copy upstream stays."
              folded
              groups={section("yours")}
              pick={pick}
              onTick={tick}
            />
            <Section
              title="In stray worktrees"
              note="checked out in a folder no agent of any project has (an old workspace's, one since deleted): picked, that worktree is removed first, unless it holds changes not committed."
              groups={section("stray")}
              pick={pick}
              onTick={tick}
            />
            {staying.length > 0 && (
              <div className="flex flex-col gap-1">
                <h3 className="m-0 text-[13px] font-semibold text-tx2">Staying</h3>
                <div className="flex flex-col rounded-lg border border-line">
                  {staying.map((b) => (
                    <Row key={b.branch} b={b} />
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
        <div className="flex justify-end gap-2">
          <Button onClick={onClose}>Cancel</Button>
          <Button variant="danger" disabled={pick.size === 0} busy={remove.isPending} onClick={run}>
            {remove.isPending ? "Deleting" : "Delete"} {plural(pick.size, "branch", "branches")}
          </Button>
        </div>
      </div>
    </Modal>
  );
}

/** The agents' branches, or the CEO's: each group with its own checkbox over its rows. A
 * folded section shows only its title, its count, and how many of it are ticked. */
function Section({
  title,
  note,
  groups,
  pick,
  onTick,
  folded = false,
}: {
  title: string;
  note?: string;
  groups: CleanupGroup[];
  pick: Set<string>;
  onTick: (names: string[], on: boolean) => void;
  /** Folded at first (the CEO's own branches: the CEO, 2026-11-11). */
  folded?: boolean;
}) {
  const [open, setOpen] = useState(!folded);
  if (groups.length === 0) return null;
  const names = groups.flatMap((g) => g.branches.map((b) => b.branch));
  const ticked = names.filter((n) => pick.has(n)).length;
  return (
    <div className="flex flex-col gap-1">
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
        className="flex items-center gap-1.5 text-left text-[13px] font-semibold text-tx2 hover:text-tx"
      >
        <Chevron open={open} />
        {title}
        <span className="font-normal text-tx3">
          ({names.length}
          {ticked > 0 && `, ${ticked} ticked`}){note && ` · ${note}`}
        </span>
      </button>
      {open && (
        <div className="flex flex-col rounded-lg border border-line">
          {groups.map((g) => {
            const names = g.branches.map((b) => b.branch);
            const count = names.filter((n) => pick.has(n)).length;
            return (
              <div key={g.key} className="flex flex-col border-b border-line last:border-b-0">
                <label className="flex items-center gap-2.5 bg-raised px-3 py-2 text-[13px] text-tx">
                  <input
                    type="checkbox"
                    checked={count === names.length}
                    ref={(el) => {
                      if (el) el.indeterminate = count > 0 && count < names.length;
                    }}
                    onChange={(e) => onTick(names, e.target.checked)}
                  />
                  <span className="font-semibold">{g.label}</span>
                  <span className="text-tx3">({names.length})</span>
                  <span className="ml-auto text-[12px] text-tx3">{g.hint}</span>
                </label>
                {g.branches.map((b) => (
                  <Row
                    key={b.branch}
                    b={b}
                    on={pick.has(b.branch)}
                    onTick={(on) => onTick([b.branch], on)}
                  />
                ))}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

/** One branch: ticked or not, or with why it stays (no checkbox then). */
function Row({
  b,
  on = false,
  onTick,
}: {
  b: CleanupBranch;
  on?: boolean;
  onTick?: (on: boolean) => void;
}) {
  const by = b.supersededBy ?? [];
  // A label only around a checkbox: a staying row has none.
  const Line = onTick ? "label" : "div";
  return (
    <Line
      className={`flex items-center gap-2.5 border-t border-line py-1.5 pr-3 first:border-t-0 ${onTick ? "pl-8 text-tx2 hover:bg-hover" : "pl-3 text-tx4"}`}
    >
      {onTick && <input type="checkbox" checked={on} onChange={(e) => onTick(e.target.checked)} />}
      <span className="min-w-0 flex-1 truncate font-mono text-[12.5px]" title={b.branch}>
        {b.branch}
      </span>
      {b.legacy && <span className="text-[11px] text-tx3">legacy</span>}
      {b.stray && b.state === "unmerged" && (
        <span className="shrink-0 text-[12px] text-attnTx">not merged</span>
      )}
      {b.stays ? (
        // A stray worktree's folder may be long: cut, whole in the tooltip.
        <span className="min-w-0 shrink truncate text-[12px]" title={b.stays}>
          {b.stray ? `in ${b.stray}` : b.stays}
        </span>
      ) : (
        by.length > 0 && (
          <span className="min-w-0 shrink truncate text-[12px] text-tx3" title={by.join(", ")}>
            in {by[0]}
            {by.length > 1 && ` +${by.length - 1}`}
          </span>
        )
      )}
    </Line>
  );
}
