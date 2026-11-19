import { useState } from "react";
import type { CleanupBranch } from "../api/client";
import { plural } from "../app/model";
import { useCleanupBranches, useDeleteBranches, useRepoName } from "../app/queries";
import {
  type CleanupGroup,
  type CleanupSection,
  cleanupGroups,
  cleanupPick,
  cleanupSections,
  pickKey,
} from "../app/repos";
import { Chevron } from "./icons";
import { Modal } from "./Overlays";
import { RepoName } from "./RepoName";
import { useToast } from "./Toasts";
import { Button, Empty, Spinner } from "./ui";

// A project's branch cleanup (wiki/product/features/repos.md#branch-cleanup): the branches of
// all its code repos in groups, the safest to delete first, each ticked or unticked at once; the
// merged and superseded ones (the agents', then the CEO's) ticked at first. One that can't go is
// listed apart, with why. Foreman keeps a copy of each deleted branch for 30 days.

export function BranchCleanup({ project }: { project: string }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button onClick={() => setOpen(true)}>Clean up branches</Button>
      {open && <CleanupDialog project={project} onClose={() => setOpen(false)} />}
    </>
  );
}

function CleanupDialog({ project, onClose }: { project: string; onClose: () => void }) {
  const branches = useCleanupBranches(project);
  const remove = useDeleteBranches(project);
  const repoName = useRepoName(project);
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
    remove.mutate(
      (branches.data ?? [])
        .filter((b) => pick.has(pickKey(b)))
        .map(({ repo, branch }) => ({ repo, branch })),
      {
        onSuccess: (done) => {
          toast({
            title: `${plural(done.removed.length, "branch", "branches")} deleted`,
            body:
              done.kept.length > 0
                ? `Kept: ${done.kept.map((k) => `${repoName.name(k.repo)} ${k.branch} (${k.why})`).join(", ")}.`
                : "Foreman keeps a copy of each for 30 days.",
            color: done.kept.length > 0 ? "var(--attn)" : "var(--ok)",
          });
          onClose();
        },
        onError: (e) => toast({ title: "Not deleted", body: e.message, color: "var(--err)" }),
      },
    );
  return (
    <Modal open onClose={onClose} title="Clean up branches" width={640}>
      <div className="flex flex-col gap-4">
        <p className="m-0 text-tx2">
          The branches of this project's code repos that can go. Deleting a merged or superseded one
          loses nothing. Foreman keeps a copy of each for 30 days.
        </p>
        {branches.isError && <p className="m-0 text-err">{branches.error.message}</p>}
        {branches.isPending && (
          <p className="m-0 flex items-center gap-2.5 text-tx3">
            <Spinner />
            Reading the branches of each repo: where each one's work stands…
          </p>
        )}
        {branches.data && branches.data.length === 0 && <Empty>No branches to clean up.</Empty>}
        {branches.data && branches.data.length > 0 && (
          <div className="flex max-h-[55vh] flex-col gap-4 overflow-auto">
            {cleanupSections.map((s) => (
              <Section
                key={s.key}
                project={project}
                section={s}
                groups={groups.filter((g) => g.section === s.key)}
                pick={pick}
                onTick={tick}
              />
            ))}
            {staying.length > 0 && (
              <div className="flex flex-col gap-1">
                <h3 className="m-0 text-[13px] font-semibold text-tx2">Staying</h3>
                <div className="flex flex-col rounded-lg border border-line">
                  {staying.map((b) => (
                    <Row key={pickKey(b)} project={project} b={b} />
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

/** One section: each group with its own checkbox over its rows. A folded section shows only
 * its title, its count, and how many of it are ticked. */
function Section({
  project,
  section: { title, note, folded },
  groups,
  pick,
  onTick,
}: {
  project: string;
  section: CleanupSection;
  groups: CleanupGroup[];
  pick: Set<string>;
  onTick: (names: string[], on: boolean) => void;
}) {
  const [open, setOpen] = useState(!folded);
  if (groups.length === 0) return null;
  const names = groups.flatMap((g) => g.branches.map(pickKey));
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
            const names = g.branches.map(pickKey);
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
                    key={pickKey(b)}
                    project={project}
                    b={b}
                    on={pick.has(pickKey(b))}
                    onTick={(on) => onTick([pickKey(b)], on)}
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
  project,
  b,
  on = false,
  onTick,
}: {
  project: string;
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
      <RepoName
        project={project}
        id={b.repo}
        className="max-w-[30%] shrink-0 truncate text-[12px] text-tx4"
      />
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
