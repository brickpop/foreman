import * as Menu from "@radix-ui/react-dropdown-menu";
import * as Popover from "@radix-ui/react-popover";
import { Link } from "@tanstack/react-router";
import { useEffect, useRef } from "react";
import type { Instance } from "../../api/client";
import { FilesInbox, FilesShared } from "../../api/types/home";
import { repoName } from "../../app/model";
import { useDesktop, useOpenFolder, useOpenTerminal, useRepos } from "../../app/queries";
import { branchesOf, worktreeRows } from "../../app/repos";
import { Chevron } from "../../components/icons";
import { RepoName } from "../../components/RepoName";
import { Button, CopyButton } from "../../components/ui";
import { useProject } from "../Project";

// The Session header's agent parts (wiki/product/features/sessions.md#header): the
// shown agent's branch, which a click copies; its worktrees, each a menu row to copy,
// open in a terminal or the file manager, or view; and the project's file folders.

/** The shown agent's branch, as mono text with the copy icon on its right: a click
 * copies it. One per branch, should its repos ever be on different ones. */
export function AgentBranch({ shown }: { shown: Instance }) {
  const repos = useRepos(shown.id.split("@")[1]);
  const branches = branchesOf(repos.data ?? [], shown.agentId);
  // A worktree is made when its agent starts: one not seen yet is looked for
  // again as the agent's status moves (git actions refresh the rest).
  const hasSession = !["asleep", "stopped", "queued"].includes(shown.status);
  // biome-ignore lint/correctness/useExhaustiveDependencies: on a status change only
  useEffect(() => {
    if (branches.length === 0 && hasSession && repos.isSuccess) repos.refetch();
  }, [shown.status]);
  if (branches.length === 0) return null;
  return (
    <span className="flex min-w-0 items-center gap-1.5">
      <span className="text-tx4">·</span>
      {branches.map((b) => (
        <CopyButton
          key={b}
          text={() => b}
          label={`Copy the branch name ${b}`}
          // shrink!: a long branch truncates rather than pushing the menus out,
          // keeping its icon and a few letters.
          className="min-w-20 shrink! gap-1.5 px-1.5 py-0.5 font-mono text-[12px]"
        >
          <span className="truncate">{b}</span>
        </CopyButton>
      ))}
    </span>
  );
}

const trigger =
  "inline-flex shrink-0 items-center gap-1 rounded-md border border-line2 px-2 py-0.5 text-[12px] text-tx3 outline-none hover:bg-hover hover:text-tx focus-visible:outline-2 focus-visible:outline-acc data-[state=open]:bg-hover data-[state=open]:text-tx";
const panel = "z-50 rounded-lg border border-line2 bg-panel p-1.5 shadow-overlay";

/** Worktrees ▾: a row per repo where the shown agent has a branch checked out. */
export function WorktreesMenu({ shown, name }: { shown: Instance; name: string }) {
  const project = useProject();
  // Opened by the pointer, the focus stays on the trigger: no ring on a button
  // the CEO didn't move to. Opened by the keyboard, it goes to the first button.
  const byPointer = useRef(false);
  const repos = useRepos(project.id);
  const rows = worktreeRows(repos.data ?? [], shown.agentId);
  const { canOpen, canTerminal } = useDesktop();
  const folder = useOpenFolder();
  const terminal = useOpenTerminal();
  return (
    <Popover.Root>
      <Popover.Trigger
        className={trigger}
        // A click from Enter or Space has no detail (0); the pointer's counts clicks.
        onClick={(e) => {
          byPointer.current = e.detail > 0;
        }}
      >
        Worktrees
        <Chevron open />
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          align="end"
          sideOffset={4}
          aria-label={`${name}'s worktrees`}
          onOpenAutoFocus={(e) => byPointer.current && e.preventDefault()}
          className={`${panel} max-w-[calc(100vw-32px)]`}
        >
          {rows.length === 0 ? (
            <p className="m-0 px-2 py-1 text-[13px] text-tx3">No branch checked out yet</p>
          ) : (
            <ul className="m-0 grid list-none grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-x-3 gap-y-1 p-0">
              {rows.map((w) => {
                const target = { instance: shown.id, repo: w.repo };
                const repo = repoName(project, w.repo);
                return (
                  <li
                    key={w.repo}
                    className="col-span-3 grid grid-cols-subgrid items-center px-2 py-1"
                  >
                    <RepoName project={project.id} id={w.repo} className="text-[13px] text-tx" />
                    <span className="truncate font-mono text-[12px] text-tx3" title={w.branch}>
                      {w.branch}
                    </span>
                    <span className="flex items-center gap-1">
                      <CopyButton
                        text={() => w.branch}
                        label={`Copy the branch name ${w.branch}`}
                        icon={null}
                        className="gap-1 rounded-[5px] border border-line2 px-2 py-0.5 text-xs"
                      >
                        Copy
                      </CopyButton>
                      {canTerminal && (
                        <Button
                          variant="tiny"
                          aria-label={`Open a terminal at ${repo}`}
                          onClick={() => terminal.mutate(target)}
                        >
                          Terminal
                        </Button>
                      )}
                      {canOpen && (
                        <Button
                          variant="tiny"
                          aria-label={`Open ${repo}'s folder`}
                          onClick={() => folder.mutate(target)}
                        >
                          Folder
                        </Button>
                      )}
                      <Popover.Close asChild>
                        <Link
                          to="/projects/$project/repos/$repo"
                          params={{ project: project.id, repo: w.repo }}
                          search={{ branch: w.branch }}
                          aria-label={`View ${repo} on ${w.branch}`}
                          className="rounded-[5px] border border-line2 px-2 py-0.5 text-xs !text-tx3 hover:border-line3"
                        >
                          View
                        </Link>
                      </Popover.Close>
                    </span>
                  </li>
                );
              })}
            </ul>
          )}
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}

const item =
  "flex cursor-pointer items-center rounded px-2 py-1 text-[13px] text-tx2 outline-none data-[highlighted]:bg-hover data-[highlighted]:text-tx";

/** Files ▾: the project's file folders, opened on the desktop. Only where Foreman's
 * machine can open folders. */
export function FilesMenu() {
  const project = useProject();
  const { canOpen } = useDesktop();
  const open = useOpenFolder();
  if (!canOpen) return null;
  return (
    <Menu.Root modal={false}>
      <Menu.Trigger className={trigger}>
        Files
        <Chevron open />
      </Menu.Trigger>
      <Menu.Portal>
        <Menu.Content align="end" sideOffset={4} className={`${panel} flex min-w-44 flex-col`}>
          {(
            [
              [FilesShared, "Shared files"],
              [FilesInbox, "File inbox"],
            ] as const
          ).map(([folder, label]) => (
            <Menu.Item
              key={folder}
              onSelect={() => open.mutate({ project: project.id, folder })}
              title={`Open on the desktop: ${project.files}/${folder}`}
              className={item}
            >
              {label}
            </Menu.Item>
          ))}
        </Menu.Content>
      </Menu.Portal>
    </Menu.Root>
  );
}
