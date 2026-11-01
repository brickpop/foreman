import { useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, useParams, useSearch } from "@tanstack/react-router";
import { memo, type ReactNode, useCallback, useMemo, useRef, useState } from "react";
import type { NewComment, RepoDiff, ReviewComment, Review as ReviewData } from "../../api/client";
import { type DiffLine, diffFiles, type FileDiff, foldedAtFirst } from "../../app/diff";
import { agentOf, forgeOf, linkLine, mainOf, when } from "../../app/model";
import { reviewOf, sessionOf } from "../../app/project";
import {
  keys,
  useDeployments,
  useProjects,
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
  lineKey,
  lostNote,
  mergeHeads,
  mergeLabel,
  openRepos,
  placeComments,
  reviewTeam,
  statusLabel,
} from "../../app/reviews";
import { Avatar } from "../../components/Avatar";
import { useToast } from "../../components/Toasts";
import { Button, Empty, Loading, SectionLabel, Segmented } from "../../components/ui";

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
};

const fileId = (repo: string, path: string) => `file:${repo}/${path}`;

export function Review() {
  const { review: id } = useParams({ from: "/projects/$project/inbox/$review" });
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
  const add = useReviewChange((c, n: NewComment) => c.reviews.addComment(id, n), failed);
  const edit = useReviewChange(
    (c, e: { comment: string; body: string }) => c.reviews.editComment(id, e.comment, e.body),
    failed,
  );
  const remove = useReviewChange(
    (c, comment: string) => c.reviews.deleteComment(id, comment),
    failed,
  );
  // Worked out once per diff and per change of the comments, not on each
  // render: a big review has tens of thousands of lines.
  const repos = useMemo(
    () => (diff.data ?? []).map((d) => ({ ...d, files: diffFiles(d.diff) })),
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
  const open = v !== undefined && isOpen(v);
  const comments: Comments = useMemo(
    () => ({
      compose,
      add: (n) => add.mutate(n, { onSuccess: () => compose(undefined) }),
      edit: (c, body) => edit.mutate({ comment: c.id, body }),
      remove: (c) => remove.mutate(c.id),
      open,
    }),
    [add.mutate, edit.mutate, remove.mutate, open],
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
  const project = projects.data?.find((p) => p.id === v.projectId);
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
          <h1 className="min-w-0 text-[22px] font-semibold tracking-[-0.02em]">
            {v.title || v.entry || v.branch}
          </h1>
        </div>
        <p className="flex flex-wrap items-center gap-2 text-[14px] text-tx3">
          <Avatar look={lead?.soul.look} size={20} />
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
            </Link>{" "}
            · {statusLabel(v.status)} · {when(v.updatedAt)}
          </span>
        </p>
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
            value={since ? "since" : "main"}
            onChange={(to) => navigate({ search: to === "since" ? { since: true } : {} })}
            options={[
              { value: "main", label: "Against main" },
              { value: "since", label: "Since your last review" },
            ]}
          />
        )}
      </header>
      <div className="flex min-h-0 flex-1">
        <nav className="flex w-[240px] shrink-0 flex-col gap-5 overflow-auto border-r border-line px-3 py-5 text-[13px] max-[900px]:hidden">
          {repos.map((r) => (
            <div key={r.repo} className="flex flex-col gap-0.5">
              <SectionLabel>
                {r.repo} → {r.main}
                {decidedAs(v, r.repo) && ` · ${decidedAs(v, r.repo)}`}
              </SectionLabel>
              {r.files.map((f) => (
                <button
                  key={f.path}
                  type="button"
                  title={f.path}
                  onClick={() => {
                    const id = fileId(r.repo, f.path);
                    fold(id, false);
                    // After the unfold renders, so the file is where it will stay.
                    requestAnimationFrame(() =>
                      document.getElementById(id)?.scrollIntoView({ block: "start" }),
                    );
                  }}
                  className="flex items-baseline gap-2 truncate rounded-md px-2 py-1.5 text-left text-tx2 hover:bg-hover"
                >
                  <span className="min-w-0 flex-1 truncate font-mono text-[12px]">{f.path}</span>
                  <span className="font-mono text-[11px] text-ok">+{f.added}</span>
                  <span className="font-mono text-[11px] text-err">−{f.removed}</span>
                </button>
              ))}
              {r.files.length === 0 && (
                <span className="px-1.5 text-tx4">{r.gone ? "Can't show it." : "No changes."}</span>
              )}
            </div>
          ))}
        </nav>
        {/* The padding is inside, not on the scrolling pane: a pane's own padding
            would hold a stuck file header 20px below its top, lines showing above. */}
        <div className="min-w-0 flex-1 overflow-auto px-8">
          <div className="flex flex-col gap-5 py-5">
            {diff.error && <p className="text-err">{diff.error.message}</p>}
            {diff.isPending && <Loading rows={8} />}
            {diff.data?.length === 0 && <Empty>Nothing to read: the review has no repo.</Empty>}
            {repos
              .filter((r) => r.gone)
              .map((r) => (
                <Empty key={r.repo}>
                  {r.repo}: the change can't be shown. It wasn't recorded when the review was
                  decided, or git no longer has its commits.
                </Empty>
              ))}
            {repos.map((r) =>
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
                    composing?.repo === r.repo && composing.path === f.path ? composing : undefined
                  }
                  folded={folds.get(fileId(r.repo, f.path)) ?? foldedAtFirst(f) !== undefined}
                  onFold={fold}
                />
              )),
            )}
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
          mainOf={(repo) => mainOf(project, repo)}
          forge={forgeOf(project)}
          diffs={diff.data}
          dismissAs={dismissChoices(
            reviewTeam(v, deployments.data ?? [], teams.data ?? []),
            v.projectId,
          )}
          failed={failed}
        />
      )}
    </div>
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
      <SectionLabel>General comments</SectionLabel>
      {general.map((c) => (
        <CommentItem key={c.id} comment={c} comments={comments} />
      ))}
      {comments.open && (
        <Composer
          // A new box once the comment is kept; on a failure the text stays.
          key={general.length}
          placeholder="A comment on the whole change…"
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
          <span className="rounded-[4px] border border-acc px-1.5 text-accTx">draft</span>
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
  onSave,
  onCancel,
}: {
  initial?: string;
  placeholder: string;
  onSave: (body: string) => void;
  onCancel?: () => void;
}) {
  const [body, setBody] = useState(initial);
  return (
    <form
      className="flex flex-col gap-2"
      onSubmit={(e) => {
        e.preventDefault();
        if (body.trim()) onSave(body.trim());
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
        <Button type="submit" variant="raised" disabled={!body.trim()}>
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
  mainOf,
  forge,
  diffs,
  dismissAs,
  failed,
}: {
  review: ReviewData;
  lead: string;
  mainOf: (repo: string) => string;
  forge: string;
  diffs: RepoDiff[] | undefined;
  dismissAs: DismissChoice[];
  failed: (e: Error) => void;
}) {
  const navigate = useNavigate();
  // A decision made, the Inbox's next one is the next thing to do.
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
  const merge = useReviewChange(
    (c, heads: Record<string, string>) => c.reviews.merge(v.id, heads),
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
  const busy = merge.isPending || changes.isPending || dismiss.isPending;
  const repos = openRepos(v, mainOf);
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
  return (
    <footer className="flex shrink-0 flex-col gap-3 border-t border-line bg-side px-6 py-3">
      {v.status === "changes-requested" && (
        <p className="text-[13.5px] text-tx3">
          Changes requested: waiting on {lead} to fix them and ask again.
        </p>
      )}
      <div className="flex flex-wrap items-center gap-x-5 gap-y-2">
        {v.mergeRequests
          .filter((m) => m.state === "open")
          .map((m) => (
            <label key={m.repo} className="flex items-center gap-2 font-mono text-[12.5px]">
              <input
                type="checkbox"
                checked={!unticked.has(m.repo)}
                onChange={(e) => tick(m.repo, e.target.checked)}
              />
              <span className={m.conflicting ? "text-err" : "text-tx2"}>
                {linkLine(m, forge, mainOf(m.repo))}
              </span>
            </label>
          ))}
        <span className="flex-1" />
        <Button
          variant="danger"
          disabled={busy}
          onClick={() => setForm(form === "dismiss" ? "" : "dismiss")}
        >
          Dismiss
        </Button>
        <Button disabled={busy} onClick={() => setForm(form === "changes" ? "" : "changes")}>
          Request changes{draft > 0 && ` (${draft})`}
        </Button>
        <Button
          variant="primary"
          disabled={busy || ticked.length === 0 || !heads}
          onClick={() =>
            heads &&
            merge.mutate(heads, {
              onSuccess: () => {
                toast({
                  title: "Merged",
                  body: ticked.map((r) => `${r.repo} → ${r.main}`).join(", "),
                  color: "var(--ok)",
                  // To watch the lead pick it up.
                  onOpen: () => navigate(sessionOf(v.lead)),
                });
                back();
              },
            })
          }
        >
          {ticked.length > 0 ? mergeLabel(ticked) : "Merge"}
        </Button>
      </div>
      {form && (
        <form
          className="flex flex-col gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            if (form === "changes")
              changes.mutate(text.trim(), {
                onSuccess: () => {
                  done();
                  back();
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
                      body: keptLines(dismissed),
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
    </footer>
  );
}
