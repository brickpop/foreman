import * as Menu from "@radix-ui/react-dropdown-menu";
import * as Popover from "@radix-ui/react-popover";
import { useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, useParams, useSearch } from "@tanstack/react-router";
import { memo, type ReactNode, useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { NewComment, RepoDiff, ReviewComment, Review as ReviewData } from "../../api/client";
import { type DiffLine, diffFiles, type FileDiff, foldedAtFirst } from "../../app/diff";
import { fileTree, type TreeNode, treeOrder } from "../../app/fileTree";
import { agentBranch, agentOf, linkLine, projectName, when } from "../../app/model";
import { reviewOf, sessionOf } from "../../app/project";
import {
  keys,
  useCommentChange,
  useDeployments,
  useProjects,
  useRepoName,
  useReview,
  useReviewChange,
  useReviewDiff,
  useTeams,
} from "../../app/queries";
import {
  commentAt,
  type DismissChoice,
  decidedAs,
  diffKey,
  dismissChoices,
  dismissLabel,
  dismissRequest,
  drafts,
  hasSince,
  isOpen,
  keptLines,
  landsOn,
  lineKey,
  lostDrafts,
  lostNote,
  mergeHeads,
  mergeLabel,
  openRepos,
  placeComments,
  readingFile,
  reviewTeam,
  statusLabel,
  treeWidth,
  treeWidths,
} from "../../app/reviews";
import { TextBox } from "../../components/AgentText";
import { TreeChildren, TreeRow } from "../../components/FileTree";
import { Chevron } from "../../components/icons";
import { Confirm, Modal } from "../../components/Overlays";
import { RepoName } from "../../components/RepoName";
import { useToast } from "../../components/Toasts";
import {
  Button,
  CopyButton,
  Empty,
  Loading,
  SectionLabel,
  SectionTitle,
  Segmented,
} from "../../components/ui";

// One review of a lead's branch (wiki/product/features/reviews.md): its files
// by repo, each file's diff with the CEO's comments on its lines, the general
// and outdated comments, and the decision: merge the repos ticked, request
// changes (sending the draft comments), or dismiss it.

/** The comments a screen part changes, and how. Where the CEO is writing a
 * new one goes to that file only (FileBox's writing), so opening a comment
 * box redraws one file, never the whole review. */
type Comments = {
  compose: (c?: NewComment) => void;
  add: (c: NewComment) => void;
  edit: (c: ReviewComment, body: string) => void;
  remove: (c: ReviewComment) => void;
  /** The review is open: comments can be written. */
  open: boolean;
  /** A new comment is on its way: kept, then shown. */
  adding: boolean;
};

const fileId = (repo: string, path: string) => `file:${repo}/${path}`;

export function Review() {
  const { review: id } = useParams({
    from: "/projects/$project/inbox/$review",
  });
  const search = useSearch({ from: "/projects/$project/inbox/$review" });
  const navigate = useNavigate({ from: "/projects/$project/inbox/$review" });
  const review = useReview(id);
  const v = review.data;
  const since = search.since === true && v !== undefined && hasSince(v);
  const diff = useReviewDiff(v ? id : "", since);
  const projects = useProjects();
  const teams = useTeams();
  const deployments = useDeployments();
  const toast = useToast();
  const failed = (e: Error) => toast({ title: "Not done", body: e.message, color: "var(--err)" });
  const [composing, compose] = useState<NewComment>();
  // The CEO's fold or unfold of each file, by fileId; a file not touched yet
  // is as foldedAtFirst has it. A click in the tree unfolds its file.
  const [folds, setFolds] = useState<Map<string, boolean>>(new Map());
  const fold = useCallback(
    (file: string, on: boolean) => setFolds((now) => new Map(now).set(file, on)),
    [],
  );
  // The repos the CEO folded: their files leave the tree and the diffs (the CEO, 2026-11-05: a
  // review over many repos is long). A click on a file in the tree opens its repo again.
  const [shut, setShut] = useState<Set<string>>(new Set());
  const shutRepo = (repo: string, on: boolean) =>
    setShut((now) => {
      const next = new Set(now);
      if (on) next.add(repo);
      else next.delete(repo);
      return next;
    });
  const add = useCommentChange(id, (c, n: NewComment) => c.reviews.addComment(id, n), failed);
  const edit = useCommentChange(
    id,
    (c, e: { comment: string; body: string }) => c.reviews.editComment(id, e.comment, e.body),
    failed,
  );
  const remove = useCommentChange(
    id,
    (c, comment: string) => c.reviews.deleteComment(id, comment),
    failed,
  );
  // Worked out once per diff and per change of the comments, not on each
  // render: a big review has tens of thousands of lines.
  const repos = useMemo(
    () => (diff.data ?? []).map((d) => ({ ...d, files: treeOrder(diffFiles(d.diff)) })),
    [diff.data],
  );
  const placed = useMemo(() => {
    const shown = new Set(
      repos.flatMap((r) =>
        r.files.flatMap((f) =>
          f.lines.map((l) => diffKey(r.repo, f.path, l)).filter((k) => k !== undefined),
        ),
      ),
    );
    return placeComments(v?.comments ?? [], since, shown);
  }, [repos, v?.comments, since]);
  // The file the CEO is reading, lit in the tree (the CEO, 2026-11-07). A click in the tree
  // pins its file until the diffs scroll on: a short last file never reaches the top.
  const [reading, setReading] = useState<string>();
  const pane = useRef<HTMLDivElement>(null);
  const pin = useRef<{ id: string; scroll: number }>(undefined);
  const findReading = useCallback(() => {
    const el = pane.current;
    if (!el) return;
    if (pin.current) {
      if (el.scrollTop === pin.current.scroll) return;
      pin.current = undefined;
    }
    const view = el.getBoundingClientRect();
    const files = [...el.querySelectorAll<HTMLElement>("section[id^='file:']")].map((box) => {
      const { top, bottom } = box.getBoundingClientRect();
      return { id: box.id, top, bottom };
    });
    setReading(readingFile(files, view.top, view.bottom));
  }, []);
  // Folding moves the files without a scroll.
  // biome-ignore lint/correctness/useExhaustiveDependencies: on a change of what the diffs show
  useEffect(findReading, [findReading, repos, folds, shut]);
  const open = v !== undefined && isOpen(v);
  const comments: Comments = useMemo(
    () => ({
      compose,
      add: (n) => add.mutate(n, { onSuccess: () => compose(undefined) }),
      edit: (c, body) => edit.mutate({ comment: c.id, body }),
      remove: (c) => remove.mutate(c.id),
      open,
      adding: add.isPending,
    }),
    [add.mutate, add.isPending, edit.mutate, remove.mutate, open],
  );

  if (review.error)
    return (
      <Page>
        <p className="text-err">{review.error.message}</p>
      </Page>
    );
  if (!v)
    return (
      <Page>
        <Loading rows={6} />
      </Page>
    );
  const lead = agentOf(teams.data ?? [], v.deploymentId, v.lead);
  // Two panes under the header, each scrolling on its own: going to a file
  // moves the diffs, never the tree.
  return (
    <div className="flex h-full flex-col">
      <header className="flex shrink-0 flex-col gap-2 border-b border-line px-6 pt-5 pb-4">
        <div className="flex items-center gap-3">
          <Link
            to="/projects/$project/inbox"
            params={{ project: v.projectId }}
            aria-label="Back to the Inbox"
            title="Inbox"
            className="grid size-8 shrink-0 place-items-center rounded-md border border-line2 text-tx3 hover:border-line3 hover:text-tx"
          >
            ←
          </Link>
          <h1
            className="min-w-0 truncate text-[22px] font-semibold tracking-[-0.02em]"
            title={v.title || v.branch}
          >
            {v.title || v.branch}
          </h1>
        </div>
        <p className="flex flex-wrap items-center gap-2 text-[14px] text-tx3">
          <span>
            {lead?.soul.name ?? v.lead} ·{" "}
            <Link
              to="/projects/$project/repos/$repo"
              params={{ project: v.projectId, repo: v.mergeRequests[0]?.repo ?? "" }}
              search={{ branch: v.branch }}
              title="Read the branch in Repos"
              className="font-mono text-tx2 hover:text-tx"
            >
              {v.branch}
            </Link>
            <CopyButton
              text={() => v.branch}
              label="Copy the branch name"
              className="-my-1 ml-0.5 size-6 align-middle"
            />{" "}
            · {statusLabel(v.status)} · {when(v.updatedAt)} · card{" "}
            <Link
              to="/projects/$project/wiki"
              params={{ project: v.projectId }}
              search={{ entry: v.entry }}
              title={v.entry}
              className="text-tx2 hover:text-tx"
            >
              {v.entryTitle || v.entry}
            </Link>
          </span>
        </p>
        <TextBox text={v.body} label="Description" />
        {v.withdrawnReason && (
          <p className="text-[14px] text-tx3">
            Withdrawn: {v.withdrawnReason}.
            {v.supersededBy && (
              <>
                {" "}
                Superseded by{" "}
                <Link
                  {...reviewOf(v.projectId, v.supersededBy)}
                  className="font-mono text-tx2 hover:text-tx"
                >
                  {v.supersededBy}
                </Link>
                .
              </>
            )}
          </p>
        )}
        {hasSince(v) && (
          <Segmented
            label="Which change"
            value={since ? "since" : "target"}
            onChange={(to) => navigate({ search: to === "since" ? { since: true } : {} })}
            options={[
              { value: "target", label: "Against its target" },
              { value: "since", label: "Since your last review" },
            ]}
          />
        )}
      </header>
      <div className="flex min-h-0 flex-1">
        <FilesPane
          project={v.projectId}
          repos={repos}
          decided={(repo) => decidedAs(v, repo)}
          shut={shut}
          onShut={repos.length > 1 ? shutRepo : undefined}
          reading={reading}
          onOpen={(repo, path) => {
            const id = fileId(repo, path);
            shutRepo(repo, false);
            fold(id, false);
            // After the unfold renders, so the file is where it will stay.
            requestAnimationFrame(() => {
              document.getElementById(id)?.scrollIntoView({ block: "start" });
              pin.current = { id, scroll: pane.current?.scrollTop ?? 0 };
              setReading(id);
            });
          }}
        />
        {/* The padding is inside, not on the scrolling pane: a pane's own padding
            would hold a stuck file header 20px below its top, lines showing above. */}
        <div ref={pane} onScroll={findReading} className="min-w-0 flex-1 overflow-auto px-8">
          <div className="flex flex-col gap-5 py-5">
            {diff.error && <p className="text-err">{diff.error.message}</p>}
            {diff.isPending && <Loading rows={8} />}
            {diff.data?.length === 0 && <Empty>Nothing to read: the review has no repo.</Empty>}
            {repos
              .filter((r) => r.targetGone)
              .map((r) => (
                <Empty key={r.repo}>
                  <RepoName project={v.projectId} id={r.repo} />: the branch it merges into,{" "}
                  {r.target}, is gone (deleted outside Foreman). Pick another branch for it below to
                  see its change and merge it.
                </Empty>
              ))}
            {repos
              .filter((r) => r.gone)
              .map((r) => (
                <Empty key={r.repo}>
                  <RepoName project={v.projectId} id={r.repo} />: the change can't be shown. It
                  wasn't recorded when the review was decided, or git no longer has its commits.
                </Empty>
              ))}
            {repos.map((r) => (
              <section key={r.repo} className="flex flex-col gap-5">
                {/* Each repo heads its files, so one reads apart from the next. */}
                <RepoRow
                  project={v.projectId}
                  repo={r}
                  decided={decidedAs(v, r.repo)}
                  open={!shut.has(r.repo)}
                  onToggle={
                    repos.length > 1 ? () => shutRepo(r.repo, !shut.has(r.repo)) : undefined
                  }
                  big
                />
                {!shut.has(r.repo) &&
                  r.files.map((f) => (
                    <FileBox
                      key={`${r.repo}/${f.path}`}
                      repo={r.repo}
                      head={r.head}
                      file={f}
                      since={since}
                      at={placed.at}
                      comments={comments}
                      writing={
                        composing?.repo === r.repo && composing.path === f.path
                          ? composing
                          : undefined
                      }
                      folded={folds.get(fileId(r.repo, f.path)) ?? foldedAtFirst(f) !== undefined}
                      onFold={fold}
                    />
                  ))}
              </section>
            ))}
            <Conversation
              general={placed.general}
              apart={[
                { label: "Outdated: their line changed since", list: placed.outdated },
                { label: "Not in this view", list: placed.elsewhere },
              ]}
              comments={comments}
            />
          </div>
        </div>
      </div>
      {isOpen(v) && (
        <DecisionBar
          review={v}
          lead={lead?.soul.name ?? "the lead"}
          diffs={diff.data}
          dismissAs={dismissChoices(
            reviewTeam(v, deployments.data ?? [], teams.data ?? []),
            projectName(projects.data, v.projectId),
          )}
          failed={failed}
        />
      )}
    </div>
  );
}

const widthKey = "foreman.reviewTree";

function storedWidth(): number {
  try {
    return treeWidth(localStorage.getItem(widthKey));
  } catch {
    return treeWidth(null); // Storage blocked: the default width.
  }
}

/** How a file changed, by its name in the tree; nothing for an edit. */
const changeLook: Record<Exclude<FileDiff["change"], "">, string> = {
  new: "text-ok",
  deleted: "text-err",
  renamed: "text-tx3",
};

/** The changed files, under each repo's row, as a folder tree (the CEO, 2026-11-11): chains of
 * lone folders compacted into one row, folders open at first. Each repo's files scroll, both
 * ways, under its row, which stays; the right border drags (or arrow keys move) the width
 * within treeWidths, kept in this browser. */
function FilesPane({
  project,
  repos,
  decided,
  shut,
  onShut,
  reading,
  onOpen,
}: {
  project: string;
  repos: { repo: string; target: string; files: FileDiff[]; gone: boolean; targetGone: boolean }[];
  decided: (repo: string) => string;
  /** The repos folded. */
  shut: Set<string>;
  /** Folds a repo or brings it back; none with a single repo. */
  onShut?: (repo: string, on: boolean) => void;
  /** The file being read, by fileId: lit, or its folded folder lit. */
  reading?: string;
  onOpen: (repo: string, path: string) => void;
}) {
  const trees = useMemo(() => repos.map((r) => fileTree(r.files)), [repos]);
  // The folders the CEO folded, by repo and path: every other one is open.
  const [folded, setFolded] = useState<Set<string>>(new Set());
  const toggle = (key: string) =>
    setFolded((now) => {
      const next = new Set(now);
      if (!next.delete(key)) next.add(key);
      return next;
    });
  const nav = useRef<HTMLElement>(null);
  // A lit row out of the tree's sight comes into it, the tree moving the least it can.
  // biome-ignore lint/correctness/useExhaustiveDependencies: on a change of the file read only
  useEffect(() => {
    nav.current?.querySelector("[aria-current]")?.scrollIntoView({ block: "nearest" });
  }, [reading]);

  const [width, setWidth] = useState(storedWidth);
  const drag = useRef<{ x: number; width: number }>(undefined);
  const keep = (px: number) => {
    try {
      localStorage.setItem(widthKey, String(px));
    } catch {
      // Storage blocked: the default width next time.
    }
  };

  const rows = (repo: string, nodes: TreeNode<FileDiff>[]): ReactNode =>
    nodes.map((n) => {
      if (n.kind === "file") {
        const f = n.item;
        return (
          <TreeRow
            key={f.path}
            current={fileId(repo, f.path) === reading}
            title={f.from ? `${f.from} → ${f.path}` : f.path}
            onClick={() => onOpen(repo, f.path)}
            after={
              f.change && <span className={`text-[11px] ${changeLook[f.change]}`}>{f.change}</span>
            }
          >
            {n.name}
          </TreeRow>
        );
      }
      const key = `${repo}/${n.path}`;
      const open = !folded.has(key);
      return (
        <div key={`${n.path}/`} className="flex flex-col">
          <TreeRow
            folder
            open={open}
            // Folded over the file being read, the folder is lit in its place.
            current={!open && reading?.startsWith(fileId(repo, `${n.path}/`))}
            title={n.path}
            onClick={() => toggle(key)}
          >
            {n.name}
          </TreeRow>
          {open && <TreeChildren>{rows(repo, n.children)}</TreeChildren>}
        </div>
      );
    });

  return (
    <div className="relative flex shrink-0 max-[900px]:hidden" style={{ width }}>
      {/* The trees scroll down as one list, each repo's row sticking at the top while its files
          pass under it; sideways, each repo's files scroll on their own (the CEO, 2026-11-11). */}
      <nav
        ref={nav}
        className="flex w-full min-w-0 flex-col gap-5 overflow-y-auto overflow-x-hidden border-r border-line px-3 pb-5 text-[13px]"
      >
        {repos.map((r, i) => (
          <div key={r.repo} className="flex shrink-0 flex-col gap-0.5 first:pt-5">
            {/* On the page's background, so rows passing under it never show at its corners. */}
            <div className="sticky top-0 z-10 flex flex-col bg-bg pb-0.5">
              <RepoRow
                project={project}
                repo={r}
                decided={decided(r.repo)}
                open={!shut.has(r.repo)}
                onToggle={onShut && (() => onShut(r.repo, !shut.has(r.repo)))}
              />
            </div>
            {!shut.has(r.repo) && r.files.length > 0 && (
              <div className="overflow-x-auto">
                {/* As wide as its widest row, so a long name scrolls its repo's files, not its row. */}
                <div className="flex w-max min-w-full flex-col">{rows(r.repo, trees[i])}</div>
              </div>
            )}
            {r.files.length === 0 && !shut.has(r.repo) && (
              <span className={`px-1.5 ${r.targetGone ? "text-err" : "text-tx4"}`}>
                {r.targetGone ? `${r.target} is gone.` : r.gone ? "Can't show it." : "No changes."}
              </span>
            )}
          </div>
        ))}
      </nav>
      {/* biome-ignore lint/a11y/useSemanticElements: a focusable separator is the ARIA splitter; an <hr> can't be one */}
      <div
        role="separator"
        aria-orientation="vertical"
        aria-label="The file tree's width"
        aria-valuenow={width}
        aria-valuemin={treeWidths.min}
        aria-valuemax={treeWidths.max}
        tabIndex={0}
        onPointerDown={(e) => {
          e.preventDefault(); // no text selected while dragging
          e.currentTarget.setPointerCapture(e.pointerId);
          drag.current = { x: e.clientX, width };
        }}
        onPointerMove={(e) => {
          if (drag.current) setWidth(treeWidth(drag.current.width + e.clientX - drag.current.x));
        }}
        onLostPointerCapture={() => {
          drag.current = undefined;
          keep(width);
        }}
        onKeyDown={(e) => {
          const by = { ArrowLeft: -16, ArrowRight: 16 }[e.key];
          if (by === undefined) return;
          e.preventDefault();
          const to = treeWidth(width + by);
          setWidth(to);
          keep(to);
        }}
        className="absolute inset-y-0 -right-[3px] z-20 w-[6px] cursor-col-resize touch-none hover:bg-acc/40 focus-visible:bg-acc/60 focus-visible:outline-none"
      />
    </div>
  );
}

/** A repo's head, in the tree and over its diffs: `<repo> → <target>` on its own raised band, so
 * one repo reads apart from the next (the CEO, 2026-11-05), and a click folds all its files.
 * Only with two repos or more: a review's only repo doesn't fold (the CEO, 2026-11-07). */
function RepoRow({
  project,
  repo: r,
  decided,
  open,
  onToggle,
  big = false,
}: {
  project: string;
  repo: { repo: string; target: string; files: FileDiff[] };
  /** How it was decided (merged, dismissed…), or nothing while open. */
  decided: string;
  open: boolean;
  /** Folds or unfolds its files; none when it doesn't fold. */
  onToggle?: () => void;
  /** Over the diffs: roomier, with its counts. */
  big?: boolean;
}) {
  const repoName = useRepoName(project);
  const added = r.files.reduce((n, f) => n + f.added, 0);
  const removed = r.files.reduce((n, f) => n + f.removed, 0);
  const look = `flex min-w-0 shrink-0 items-center gap-2 rounded-md border border-line2 bg-raised text-left text-tx ${onToggle ? "hover:border-line3" : ""} ${big ? "px-3 py-2 text-[13.5px]" : "px-2 py-1.5 text-[12.5px]"}`;
  const inside = (
    <>
      {onToggle && <Chevron open={open} />}
      <span className="min-w-0 flex-1 truncate font-semibold" title={repoName.title(r.repo)}>
        {repoName.name(r.repo)} → <span className="font-mono">{r.target}</span>
        {decided && <span className="font-normal"> · {decided}</span>}
      </span>
      {big && r.files.length > 0 && (
        <span className="shrink-0 text-[12px] font-normal text-tx3">
          {r.files.length} {r.files.length === 1 ? "file" : "files"}{" "}
          <span className="text-ok">+{added}</span> <span className="text-err">−{removed}</span>
        </span>
      )}
    </>
  );
  if (!onToggle) return <div className={look}>{inside}</div>;
  return (
    <button
      type="button"
      aria-expanded={open}
      onClick={onToggle}
      title={open ? "Fold its files" : "Show its files"}
      className={look}
    >
      {inside}
    </button>
  );
}

function Page({ children }: { children?: ReactNode }) {
  return <div className="flex min-h-full flex-col gap-6 px-6 pt-5">{children}</div>;
}

const lineLook: Record<DiffLine["kind"], string> = {
  hunk: "bg-hover text-tx4",
  add: "bg-ok/10 text-ok",
  del: "bg-err/10 text-err",
  ctx: "text-tx3",
  meta: "text-tx4",
};

/** One file's diff: its lines numbered on each side, the comments under the
 * line they're on, and a + on a line to comment on it. Its header folds it.
 * Memoized, and drawn by the browser only near the view (content-visibility),
 * so a review of hundreds of files stays quick: a click redraws its own file. */
const FileBox = memo(function FileBox({
  repo,
  head,
  file: f,
  since,
  at,
  comments,
  writing,
  folded,
  onFold,
}: {
  repo: string;
  head: string;
  file: FileDiff;
  since: boolean;
  at: Map<string, ReviewComment[]>;
  comments: Comments;
  /** The new comment being written on this file, if one is. */
  writing?: NewComment;
  folded: boolean;
  onFold: (file: string, folded: boolean) => void;
}) {
  const id = fileId(repo, f.path);
  const why = folded ? foldedAtFirst(f) : undefined;
  const box = useRef<HTMLElement>(null);
  const header = useRef<HTMLButtonElement>(null);
  const toggle = () => {
    // Folding a file whose header is stuck at the top (its start scrolled
    // past) brings the file's start back, instead of leaving the next files
    // wherever the fold drops them.
    const stuck =
      !folded &&
      (box.current?.getBoundingClientRect().top ?? 0) <
        (header.current?.getBoundingClientRect().top ?? 0);
    onFold(id, !folded);
    if (stuck) requestAnimationFrame(() => box.current?.scrollIntoView({ block: "start" }));
  };
  return (
    // overflow-clip rounds the corners without breaking the sticky header;
    // stuck, the header is square, with nothing showing past its corners.
    <section
      ref={box}
      id={id}
      className="shrink-0 overflow-clip rounded-[10px] border border-line2"
    >
      <button
        ref={header}
        type="button"
        aria-expanded={!folded}
        onClick={toggle}
        className={`sticky top-0 z-10 flex w-full items-center gap-2 bg-raised px-3.5 py-2.5 text-left font-mono text-[12.5px] hover:brightness-110 ${folded ? "" : "border-b border-line2"}`}
      >
        <span className="w-3 text-tx3">{folded ? "▸" : "▾"}</span>
        <span className="text-tx4">{repo}</span>
        {f.from && <span className="text-tx3">{f.from} →</span>}
        <span className="font-medium text-tx">{f.path}</span>
        {f.change && <span className="text-tx4">{f.change}</span>}
        {why && <span className="font-sans text-[12px] text-tx4">{why}</span>}
        <span className="flex-1" />
        <span className="text-[11.5px] text-ok">+{f.added}</span>
        <span className="text-[11.5px] text-err">−{f.removed}</span>
      </button>
      {/* Folded, its lines aren't built at all; open, the browser draws them
          only near the view, holding their height (20px a line) meanwhile. */}
      {!folded && (
        <div
          className="bg-term py-1 font-mono text-[12.5px] leading-[1.6]"
          style={{
            contentVisibility: "auto",
            containIntrinsicSize: `auto ${f.lines.length * 20 + 8}px`,
          }}
        >
          {/* A pure rename, an empty new file, a mode change: nothing to number. */}
          {f.lines.length === 0 && (
            <p className="px-3.5 py-1 font-sans text-tx4">
              {f.change === "renamed" ? "Moved, its content unchanged." : "No lines changed."}
            </p>
          )}
          {f.lines.map((line, i) => {
            const key = diffKey(repo, f.path, line);
            const start = comments.open ? commentAt(repo, f.path, line, head, since) : undefined;
            const here =
              writing !== undefined &&
              lineKey(writing.repo, writing.path, writing.side as "new" | "old", writing.line) ===
                key;
            return (
              // biome-ignore lint/suspicious/noArrayIndexKey: a diff line's place is its identity
              <div key={i}>
                <div
                  className={`group grid grid-cols-[40px_40px_minmax(0,1fr)] ${lineLook[line.kind]}`}
                >
                  <span className="relative pr-1.5 text-right text-tx4 select-none">
                    {start && (
                      <button
                        type="button"
                        aria-label={`Comment on line ${start.line}`}
                        onClick={() => comments.compose(start)}
                        className="absolute top-px left-0.5 hidden size-[18px] place-items-center rounded bg-acc pb-[3px] leading-none font-semibold text-accInk group-hover:grid"
                      >
                        +
                      </button>
                    )}
                    {line.old}
                  </span>
                  <span className="pr-1.5 text-right text-tx4 select-none">{line.new}</span>
                  <span className="px-2 whitespace-pre-wrap select-text">{line.text || " "}</span>
                </div>
                {key && (at.get(key)?.length || here) && (
                  <div className="flex flex-col gap-2 border-y border-line bg-panel px-3 py-2.5 font-sans text-[14px]">
                    {at.get(key)?.map((c) => (
                      <CommentItem key={c.id} comment={c} comments={comments} />
                    ))}
                    {here && writing && (
                      <Composer
                        placeholder="What should change here?"
                        busy={comments.adding}
                        onSave={(body) => comments.add({ ...writing, body })}
                        onCancel={() => comments.compose(undefined)}
                      />
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </section>
  );
});

/** The general comments with a box to write one, then the comments listed
 * apart from the diff. */
function Conversation({
  general,
  apart,
  comments,
}: {
  general: ReviewComment[];
  apart: { label: string; list: ReviewComment[] }[];
  comments: Comments;
}) {
  return (
    <section className="flex flex-col gap-3 pb-6">
      <SectionTitle>General comments</SectionTitle>
      {general.map((c) => (
        <CommentItem key={c.id} comment={c} comments={comments} />
      ))}
      {comments.open && (
        <Composer
          // A new box once the comment is kept; on a failure the text stays.
          key={general.length}
          placeholder="A comment on the whole change…"
          busy={comments.adding}
          onSave={(body) =>
            comments.add({ repo: "", path: "", side: "", line: 0, commit: "", snippet: "", body })
          }
        />
      )}
      {apart
        .filter((a) => a.list.length > 0)
        .map((a) => (
          <div key={a.label} className="flex flex-col gap-2">
            <SectionLabel>{a.label}</SectionLabel>
            {a.list.map((c) => (
              <div key={c.id} className="flex flex-col gap-1">
                <span className="font-mono text-[12px] text-tx4">
                  {c.repo} {c.path}:{c.line} <span className="text-tx3">{c.snippet}</span>
                </span>
                <CommentItem comment={c} comments={comments} />
              </div>
            ))}
          </div>
        ))}
    </section>
  );
}

/** One comment: a draft can still be edited or deleted; a sent one is the
 * lead's to answer. */
function CommentItem({ comment: c, comments }: { comment: ReviewComment; comments: Comments }) {
  const [editing, setEditing] = useState(false);
  const draft = c.state === "draft";
  if (editing)
    return (
      <Composer
        initial={c.body}
        placeholder="The comment"
        onSave={(body) => {
          comments.edit(c, body);
          setEditing(false);
        }}
        onCancel={() => setEditing(false)}
      />
    );
  return (
    <div className="flex flex-col gap-1 rounded-lg border border-line bg-panel px-3 py-2">
      <span className="flex items-center gap-2 text-[12px] text-tx4">
        {draft ? (
          <span className="rounded-[4px] border border-attn px-1.5 text-attnTx">draft</span>
        ) : (
          `sent ${when(c.sentAt)}`
        )}
        <span className="flex-1" />
        {draft && comments.open && (
          <>
            <Button variant="tiny" onClick={() => setEditing(true)}>
              Edit
            </Button>
            <Button variant="tiny" onClick={() => comments.remove(c)}>
              Delete
            </Button>
          </>
        )}
      </span>
      <p className="whitespace-pre-wrap text-tx">{c.body}</p>
    </div>
  );
}

/** A box to write a comment in; it is kept as a draft. */
function Composer({
  initial = "",
  placeholder,
  busy = false,
  onSave,
  onCancel,
}: {
  initial?: string;
  placeholder: string;
  /** Its comment is being kept: Save waits, so it is never sent twice. */
  busy?: boolean;
  onSave: (body: string) => void;
  onCancel?: () => void;
}) {
  const [body, setBody] = useState(initial);
  return (
    <form
      className="flex flex-col gap-2"
      onSubmit={(e) => {
        e.preventDefault();
        if (body.trim() && !busy) onSave(body.trim());
      }}
    >
      <textarea
        rows={3}
        value={body}
        onChange={(e) => setBody(e.target.value)}
        placeholder={placeholder}
        aria-label={placeholder}
        // biome-ignore lint/a11y/noAutofocus: opened by the CEO's click, to write in
        autoFocus={onCancel !== undefined}
        className="resize-y rounded-lg border border-line2 bg-term px-3 py-2 font-sans text-[14px] text-tx placeholder:text-tx4"
      />
      <span className="flex justify-end gap-2">
        {onCancel && <Button onClick={onCancel}>Cancel</Button>}
        <Button type="submit" variant="primary" disabled={!body.trim()} busy={busy}>
          {initial ? "Save" : "Add to the draft"}
        </Button>
      </span>
    </form>
  );
}

/** The decision, at the page's foot: the repos to merge (all ticked at
 * first) at the heads the diffs show, Request changes with the draft, or
 * Dismiss: the work isn't wanted, and the CEO picks what becomes of its team
 * and branches (dismissAs, the first the default). */
function DecisionBar({
  review: v,
  lead,
  diffs,
  dismissAs,
  failed,
}: {
  review: ReviewData;
  lead: string;
  diffs: RepoDiff[] | undefined;
  dismissAs: DismissChoice[];
  failed: (e: Error) => void;
}) {
  const navigate = useNavigate();
  const repoName = useRepoName(v.projectId);
  // Merged or dismissed, the Inbox's next one is the next thing to do.
  const back = () => navigate({ to: "/projects/$project/inbox", params: { project: v.projectId } });
  const toast = useToast();
  const queries = useQueryClient();
  const [unticked, setUnticked] = useState<Set<string>>(new Set());
  const [form, setForm] = useState<"" | "changes" | "dismiss">("");
  const [text, setText] = useState("");
  const [chosen, choose] = useState<DismissChoice["value"]>("keep");
  // The team may stop meanwhile, taking its choices along: back to the default.
  const dismissing = dismissAs.find((c) => c.value === chosen) ?? dismissAs[0];
  const done = () => {
    setForm("");
    setText("");
    choose("keep");
  };
  // landing: the CEO's name for where it lands, "" for the lead's (or the target).
  const merge = useReviewChange(
    (c, m: { heads: Record<string, string>; landing: string }) =>
      c.reviews.merge(v.id, m.heads, m.landing || undefined),
    failed,
  );
  const changes = useReviewChange(
    (c, note: string) => c.reviews.requestChanges(v.id, note),
    failed,
  );
  const dismiss = useReviewChange(
    (c, choice: DismissChoice) => c.reviews.dismiss(v.id, dismissRequest(choice, text)),
    failed,
  );
  // The CEO naming a new branch for the review to land on.
  const [naming, setNaming] = useState(false);
  const busy = merge.isPending || changes.isPending || dismiss.isPending;
  const repos = openRepos(v);
  const ticked = repos.filter((r) => !unticked.has(r.repo));
  const heads = mergeHeads(
    diffs,
    ticked.map((r) => r.repo),
  );
  const draft = drafts(v.comments);
  const tick = (repo: string, on: boolean) => {
    const next = new Set(unticked);
    if (on) next.delete(repo);
    else next.add(repo);
    setUnticked(next);
  };
  // The landing a Merge waits with while the CEO confirms losing the drafts; undefined: none.
  const [confirming, setConfirming] = useState<string>();
  const mergeNow = (landing = "") =>
    heads &&
    merge.mutate(
      { heads, landing },
      {
        onSuccess: () => {
          toast({
            title: "Merged",
            body: ticked.map((r) => `${repoName.name(r.repo)} → ${landsOn(r, landing)}`).join(", "),
            color: "var(--ok)",
            // To watch the lead pick it up.
            onOpen: () => navigate(sessionOf(v.lead)),
          });
          setNaming(false);
          back();
        },
      },
    );
  const mergeAs = (landing = "") => (draft > 0 ? setConfirming(landing) : mergeNow(landing));
  // Over the lead's landing, its target itself: only when the ticked repos share one.
  const targets = [...new Set(ticked.map((r) => r.target))];
  const onTarget = ticked.some((r) => r.landing) && targets.length === 1 ? targets[0] : "";
  const variant = draft > 0 ? "secondary" : "primary";
  const off = busy || ticked.length === 0 || !heads;
  // Merge where it lands, or (its menu) on the target or under a new name (the CEO, 2026-11-11).
  const mergeButton = (
    <span className="inline-flex">
      <Button variant={variant} disabled={off} onClick={() => mergeAs()} className="rounded-r-none">
        {ticked.length > 0 ? mergeLabel(ticked) : "Merge"}
      </Button>
      <Menu.Root>
        <Menu.Trigger asChild disabled={off}>
          <Button
            variant={variant}
            aria-label="Other ways to merge"
            className="rounded-l-none border-l border-l-line2 px-2"
          >
            <span className="inline-block rotate-90">
              <Chevron open={false} />
            </span>
          </Button>
        </Menu.Trigger>
        <Menu.Portal>
          <Menu.Content
            side="top"
            align="end"
            sideOffset={6}
            className="z-50 flex min-w-56 flex-col rounded-lg border border-line2 bg-panel p-1 shadow-overlay"
          >
            {onTarget && (
              <Menu.Item className={menuItem} onSelect={() => mergeAs(onTarget)}>
                Merge to {onTarget}
              </Menu.Item>
            )}
            <Menu.Item className={menuItem} onSelect={() => setNaming(true)}>
              Merge to a new branch…
            </Menu.Item>
          </Menu.Content>
        </Menu.Portal>
      </Menu.Root>
    </span>
  );
  const open = v.mergeRequests.filter((m) => m.state === "open");
  // One repo's merge: its line (where it lands), and its checkbox when there are several to
  // pick from.
  const repoRow = (m: (typeof open)[number], several: boolean) => {
    const line = (
      <span
        className={`min-w-0 truncate font-mono text-[12.5px] ${m.conflicting ? "text-err" : "text-tx2"}`}
        title={linkLine(m, repoName.title(m.repo))}
      >
        {linkLine(m, repoName.name(m.repo))}
      </span>
    );
    return (
      <span key={m.repo} className="flex min-w-0 items-center gap-2">
        {several ? (
          <label className="flex min-w-0 items-center gap-2">
            <input
              type="checkbox"
              checked={!unticked.has(m.repo)}
              onChange={(e) => tick(m.repo, e.target.checked)}
            />
            {line}
          </label>
        ) : (
          line
        )}
      </span>
    );
  };
  return (
    <footer className="flex shrink-0 flex-col gap-3 border-t border-line bg-side px-6 py-3">
      {v.status === "changes-requested" && (
        <p className="text-[13.5px] text-tx3">
          Changes requested: waiting on {lead} to fix them and ask again.
        </p>
      )}
      <div className="flex flex-wrap items-center gap-x-5 gap-y-2">
        {/* The repos take what room the buttons leave, never theirs (the CEO, 2026-11-05):
            one is its line, cut; several are their names in one line, opening the
            picker in a popover. */}
        <span className="flex min-w-0 flex-1 basis-60">
          {open.length <= 1 ? (
            open[0] && repoRow(open[0], false)
          ) : (
            <Popover.Root>
              <Popover.Trigger
                aria-label="The repos to merge"
                className="flex min-w-0 items-center gap-2 rounded-[7px] border border-line2 bg-panel px-2.5 py-1 font-mono text-[12.5px] hover:border-line3 data-[state=open]:border-acc"
              >
                <span className="shrink-0 text-[10px] text-tx4">▸</span>
                <span className="min-w-0 truncate">
                  {open.map((m, i) => (
                    <span key={m.repo}>
                      {i > 0 && <span className="text-tx4"> · </span>}
                      <span
                        className={
                          m.conflicting
                            ? "text-err"
                            : unticked.has(m.repo)
                              ? "text-tx4"
                              : "text-tx2"
                        }
                      >
                        {unticked.has(m.repo) ? "☐" : "☑"} {repoName.name(m.repo)}
                      </span>
                    </span>
                  ))}
                </span>
                <span className="shrink-0 text-tx4">
                  {ticked.length}/{open.length}
                </span>
              </Popover.Trigger>
              <Popover.Portal>
                <Popover.Content
                  side="top"
                  align="start"
                  sideOffset={6}
                  collisionPadding={16}
                  className="z-50 flex max-h-[min(420px,60vh)] w-[min(760px,calc(100vw-32px))] flex-col gap-2 overflow-auto rounded-lg border border-line2 bg-panel p-3 shadow-overlay"
                >
                  {open.map((m) => repoRow(m, true))}
                </Popover.Content>
              </Popover.Portal>
            </Popover.Root>
          )}
        </span>
        <Button
          variant="danger"
          disabled={busy}
          onClick={() => setForm(form === "dismiss" ? "" : "dismiss")}
        >
          Dismiss
        </Button>
        {/* With drafts, sending them is the next step: Request changes leads, last. */}
        {draft > 0 && mergeButton}
        <Button
          variant={draft > 0 ? "primary" : "secondary"}
          disabled={busy}
          onClick={() => setForm(form === "changes" ? "" : "changes")}
        >
          Request changes{draft > 0 && ` (${draft})`}
        </Button>
        {draft === 0 && mergeButton}
      </div>
      <Confirm
        open={confirming !== undefined}
        title="Merge without sending the drafts?"
        body={lostDrafts(draft)}
        action="Merge anyway"
        onConfirm={() => mergeNow(confirming)}
        onClose={() => setConfirming(undefined)}
      />
      {form && (
        <form
          className="flex flex-col gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            if (form === "changes")
              changes.mutate(text.trim(), {
                onSuccess: () => {
                  done();
                  // To watch the lead pick the changes up (the CEO, 2026-11-03).
                  navigate(sessionOf(v.lead));
                },
              });
            else
              dismiss.mutate(dismissing, {
                onSuccess: (dismissed) => {
                  done();
                  if (dismissing.stopTeam) {
                    queries.invalidateQueries({ queryKey: keys.instances });
                    queries.invalidateQueries({ queryKey: keys.deployments });
                  }
                  if (dismissing.dropBranches) queries.invalidateQueries({ queryKey: keys.repos });
                  if (dismissed.kept.length > 0)
                    toast({
                      title: "Dismissed, but some branches stay",
                      body: keptLines(dismissed, repoName.name),
                      color: "var(--err)",
                    });
                  back();
                },
              });
          }}
        >
          {form === "dismiss" && (
            <fieldset className="flex flex-col gap-1 text-[13.5px] text-tx2">
              <legend className="mb-1 text-[12.5px] font-medium text-tx4">
                What becomes of it
              </legend>
              {dismissAs.map((c) => (
                <label key={c.value} className="flex items-center gap-2">
                  <input
                    type="radio"
                    name="dismiss"
                    checked={dismissing.value === c.value}
                    onChange={() => choose(c.value)}
                  />
                  {c.label}
                </label>
              ))}
            </fieldset>
          )}
          {/* A note typed stays in view when the choice won't send it, with why. */}
          {(form === "changes" || dismissing.tells || text.trim() !== "") && (
            <textarea
              rows={3}
              value={text}
              onChange={(e) => setText(e.target.value)}
              placeholder={
                form === "changes"
                  ? `A note for ${lead}, with the ${draft} draft comment${draft === 1 ? "" : "s"} (optional)`
                  : `A note for ${lead} (optional): ${lead} is told to stop work on it either way`
              }
              aria-label={form === "changes" ? `Note to ${lead}` : `Note for ${lead}`}
              className="resize-y rounded-lg border border-line2 bg-term px-3 py-2 text-tx placeholder:text-tx4"
            />
          )}
          {form === "dismiss" && lostNote(dismissing, text, lead) && (
            <p role="alert" className="text-[13px] text-err">
              {lostNote(dismissing, text, lead)}
            </p>
          )}
          <span className="flex justify-end gap-2">
            <Button onClick={done}>Cancel</Button>
            <Button
              type="submit"
              variant={form === "dismiss" ? "danger" : "primary"}
              disabled={busy || (form === "changes" && draft === 0 && !text.trim())}
            >
              {form === "changes" ? `Send to ${lead}` : dismissLabel(dismissing, text)}
            </Button>
          </span>
        </form>
      )}
      {/* Mounted while open: each opening starts with no name. */}
      {naming && (
        <MergeToDialog
          project={v.projectId}
          repos={ticked}
          busy={merge.isPending}
          onClose={() => setNaming(false)}
          onMerge={mergeAs}
        />
      )}
    </footer>
  );
}

/** An item of the Merge button's menu. */
const menuItem =
  "flex cursor-pointer items-center rounded px-2 py-1 text-[13px] text-tx2 outline-none data-[highlighted]:bg-hover data-[highlighted]:text-tx";

/** Merge to a new branch (the CEO, 2026-11-11): the repos ticked land on a branch of the CEO's
 * naming, made on Merge from each one's target (or one already at its head), the lead's
 * proposal filled in. The target, which the diff is against, stays. Nothing is pushed. */
function MergeToDialog({
  project,
  repos,
  busy,
  onClose,
  onMerge,
}: {
  project: string;
  /** The repos ticked, each with its target and the lead's landing. */
  repos: { repo: string; target: string; landing: string }[];
  busy: boolean;
  onClose: () => void;
  onMerge: (landing: string) => void;
}) {
  const proposed = new Set(repos.map((r) => r.landing));
  const [name, setName] = useState(proposed.size === 1 ? [...proposed][0] : "");
  const branch = name.trim();
  const theirs = agentBranch(branch);
  return (
    <Modal open onClose={onClose} title="Merge to a new branch">
      <form
        className="flex flex-col gap-3"
        onSubmit={(e) => {
          e.preventDefault();
          if (branch && !theirs) onMerge(branch);
        }}
      >
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          aria-label="Name of the branch it lands on"
          placeholder="f/my-feature"
          spellCheck={false}
          // biome-ignore lint/a11y/noAutofocus: opened to type the name
          autoFocus
          className="rounded-lg border border-line2 bg-term px-2.5 py-1 font-mono text-[13px] text-tx"
        />
        <p className="m-0 text-[13px] text-tx3">
          {theirs ? (
            <span className="text-err">An agent's branch: name one of yours.</span>
          ) : (
            <>
              Made from{" "}
              {repos.map((r, i) => (
                <span key={r.repo}>
                  {i > 0 && ", "}
                  <span className="font-mono text-tx2">{r.target}</span> in{" "}
                  <RepoName project={project} id={r.repo} />
                </span>
              ))}
              , as reviewed, or one of that name already there. Here only: nothing is pushed.
            </>
          )}
        </p>
        <div className="flex justify-end gap-2">
          <Button type="button" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" disabled={!branch || theirs} busy={busy}>
            {branch && !theirs ? `Merge to ${branch}` : "Merge"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
