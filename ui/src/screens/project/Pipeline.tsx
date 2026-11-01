import { Link, useNavigate } from "@tanstack/react-router";
import {
  isNotBuilt,
  type MergeRequest,
  type Project,
  type Request,
  type Team,
  type WorkItem,
} from "../../api/client";
import { agentOf, inbox, mainOf, when } from "../../app/model";
import { reviewOf, sessionOf } from "../../app/project";
import {
  useAnswer,
  useEntryTitle,
  useInstances,
  usePipeline,
  useRepos,
  useRequests,
  useTeams,
} from "../../app/queries";
import { reposWith } from "../../app/repos";
import { Avatar } from "../../components/Avatar";
import { RequestPanel } from "../../components/RequestPanel";
import { Button, Empty, Loading, NotBuilt, Panel, SectionLabel } from "../../components/ui";

// A project's pipeline (wiki/product/features/project-pipeline.md): what needs
// the CEO, in its Inbox (a review opens its page there); the work moving
// towards main, and what recently merged, in its Deployment section.

/** Where a request came from, as the panel's header says it. */
export function sourceLabel(r: Request): string {
  if (r.kind === "blocked") return "raised by Foreman";
  if (r.kind === "review") return "request_merge()";
  return `escalate(${r.kind})`;
}

const stageLabels = ["in worktree", "handed to lead", "lead reviewing", "integrated"];

/** What waits on the CEO on this project but its reviews, in its Inbox;
 * nothing when nothing does. */
export function NeedsYou({ project }: { project: Project }) {
  const requests = useRequests();
  const teams = useTeams();
  const open = inbox(requests.data ?? []).filter((r) => r.projectId === project.id);
  if (open.length === 0) return null;
  return (
    <section className="flex flex-col gap-2.5">
      <SectionLabel>Needs you</SectionLabel>
      {open.map((r) => (
        <ProjectRequest key={r.id} request={r} project={project} teams={teams.data ?? []} />
      ))}
    </section>
  );
}

/** The work on its way to main, and what merged in the last 7 days: on the
 * Deployment section, under the agents' worktrees. Each branch links to its
 * code in Repos. */
export function InFlight({ project }: { project: Project }) {
  const pipeline = usePipeline(project.id);
  const teams = useTeams();
  const instances = useInstances();
  const repos = useRepos(project.id).data ?? [];
  const mrs = pipeline.data?.mergeRequests ?? [];
  const items = pipeline.data?.workItems ?? [];
  const live = new Set((instances.data ?? []).map((i) => i.deploymentId));
  const weekAgo = Date.now() - 7 * 24 * 3600 * 1000;
  const merged = mrs
    .filter((m) => m.state === "merged" && Date.parse(m.mergedAt) >= weekAgo)
    .sort((a, b) => b.mergedAt.localeCompare(a.mergedAt))
    .slice(0, 20);
  const openMrs = mrs.filter((m) => m.state === "open");
  // In flight on their own: every work item no merge request carries, integrated
  // ones included (they wait for the lead's request_merge). A dismissed review's
  // items stay with it, out of the way, until a new hand-off frees them.
  const carried = (w: (typeof items)[number]) => mrs.some((m) => m.id === w.mergeRequestId);
  const loose = items.filter((w) => !carried(w));
  if (isNotBuilt(pipeline.error)) return <NotBuilt what="the pipeline" />;
  // Side by side when both fit (the CEO, 2026-10-17); one alone takes the width.
  return (
    <div className="grid grid-cols-[repeat(auto-fit,minmax(min(440px,100%),1fr))] items-start gap-9">
      <section className="flex min-w-0 flex-col gap-2.5">
        <div className="flex items-baseline gap-2">
          <SectionLabel>In flight</SectionLabel>
          <span className="text-xs text-tx4">worktree → lead → main</span>
        </div>
        {pipeline.isPending ? (
          <Loading />
        ) : loose.length === 0 && openMrs.length === 0 ? (
          <Empty>No work in flight.</Empty>
        ) : (
          <Panel>
            {loose.map((w) => (
              <WorkRow
                key={w.id}
                item={w}
                project={project.id}
                repo={reposWith(repos, w.branch)[0]}
                teams={teams.data ?? []}
                live={live.has(w.deploymentId)}
              />
            ))}
            {openMrs.map((m) => (
              <MergeRow
                key={m.id}
                mr={m}
                project={project.id}
                main={mainOf(project, m.repo)}
                teams={teams.data ?? []}
                items={items.filter((w) => w.mergeRequestId === m.id)}
                live={live.has(m.deploymentId)}
              />
            ))}
          </Panel>
        )}
      </section>
      {merged.length > 0 && (
        <section className="flex min-w-0 flex-col gap-2.5">
          <SectionLabel>Merged</SectionLabel>
          {merged.map((m) => (
            <MergedRow
              key={m.id}
              mr={m}
              project={project.id}
              main={mainOf(project, m.repo)}
              teams={teams.data ?? []}
              items={items.filter((w) => w.mergeRequestId === m.id)}
            />
          ))}
        </section>
      )}
    </div>
  );
}

function ProjectRequest({
  request: r,
  project,
  teams,
}: {
  request: Request;
  project: Project;
  teams: Team[];
}) {
  const navigate = useNavigate();
  const answer = useAnswer();
  const agent = agentOf(teams, r.deploymentId, r.instanceId);
  const openSession = () => navigate(sessionOf(r.instanceId));
  const context = agent ? `${agent.team.config.name} on ${project.id}` : `on ${project.id}`;
  const who = agent && { name: agent.soul.name, look: agent.soul.look };
  return (
    <RequestPanel
      request={r}
      variant="project"
      source={sourceLabel(r)}
      who={who}
      context={context}
      onAnswer={(text) => answer.mutate({ request: r, answer: text })}
      answering={answer.isPending}
      onOpenSession={openSession}
    />
  );
}

/** A branch, linked to its code in Repos when a repo has it. */
function BranchLink({ project, repo, branch }: { project: string; repo?: string; branch: string }) {
  if (!repo) return <span className="font-mono">{branch}</span>;
  return (
    <Link
      to="/projects/$project/repos/$repo"
      params={{ project, repo }}
      search={{ branch }}
      title={`Read ${branch} in ${repo}`}
      className="font-mono text-accTx hover:underline"
    >
      {branch}
    </Link>
  );
}

function WorkRow({
  item: w,
  project,
  repo,
  teams,
  live,
}: {
  item: WorkItem;
  project: string;
  /** Where its branch is read: the first repo with commits on it. */
  repo?: string;
  teams: Team[];
  live: boolean;
}) {
  const navigate = useNavigate();
  const author = agentOf(teams, w.deploymentId, w.authorInstance);
  const title = useEntryTitle(project, w.cardRef);
  const tone =
    w.checksStatus === "fail" ? "text-err" : w.checksStatus === "pass" ? "text-ok" : "text-tx3";
  return (
    <div className="grid grid-cols-[auto_1fr_auto_auto] items-center gap-3 border-b border-line px-4 py-3 last:border-b-0">
      <Avatar look={author?.soul.look} size={26} />
      <span className="flex min-w-0 flex-col">
        <span className="truncate font-medium">{title.data ?? (w.cardRef || w.branch)}</span>
        <span className="truncate text-[12.5px] text-tx3">
          {author?.soul.name ?? w.authorInstance} ·{" "}
          <BranchLink project={project} repo={repo} branch={w.branch} />
          {w.note && <span className={tone}> · {w.note}</span>}
        </span>
      </span>
      <span className="rounded-[5px] bg-raised px-2 py-[3px] text-[12.5px] text-tx2">
        {stageLabels[w.stage] ?? w.stage}
      </span>
      {live ? (
        <Button variant="small" onClick={() => navigate(sessionOf(w.authorInstance))}>
          Session
        </Button>
      ) : (
        <span className="text-xs text-tx4">stopped</span>
      )}
    </div>
  );
}

function MergeRow({
  mr,
  project,
  main,
  teams,
  items,
  live,
}: {
  mr: MergeRequest;
  project: string;
  /** The project's protected branch. */
  main: string;
  teams: Team[];
  items: WorkItem[];
  live: boolean;
}) {
  const navigate = useNavigate();
  const lead = agentOf(teams, mr.deploymentId, mr.authorInstance);
  const title = useEntryTitle(project, mr.entry);
  return (
    <div className="flex flex-col gap-2 border-b border-line px-4 py-3 last:border-b-0">
      <div className="grid grid-cols-[auto_1fr_auto_auto_auto] items-center gap-3">
        <Avatar look={lead?.soul.look} size={26} />
        <span className="flex min-w-0 flex-col">
          <span className="truncate font-medium">
            {title.data ?? mr.entry}
            {mr.forgePrNumber > 0 && <span className="text-tx3"> PR #{mr.forgePrNumber}</span>}
          </span>
          <span className="truncate text-[12.5px] text-tx3">
            {lead?.soul.name ?? mr.authorInstance} · {mr.repo} ·{" "}
            <BranchLink project={project} repo={mr.repo} branch={mr.branch} /> → {main}
            {mr.checksStatus && ` · checks ${mr.checksStatus}`}
            {mr.note && <span className={mr.conflicting ? "text-err" : ""}> · {mr.note}</span>}
          </span>
        </span>
        <span className="rounded-[5px] bg-raised px-2 py-[3px] text-[12.5px] text-tx2">
          {mr.forgePrNumber ? "PR open" : "merge requested"}
        </span>
        <Button variant="small" onClick={() => navigate(reviewOf(project, mr.reviewId))}>
          Review
        </Button>
        {mr.forgePrUrl ? (
          <Button variant="small" onClick={() => window.open(mr.forgePrUrl, "_blank", "noopener")}>
            Open PR ↗
          </Button>
        ) : live ? (
          <Button variant="small" onClick={() => navigate(sessionOf(mr.authorInstance))}>
            Session
          </Button>
        ) : (
          <span className="text-xs text-tx4">stopped</span>
        )}
      </div>
      {items.map((w) => (
        <CarriedItem key={w.id} item={w} project={project} teams={teams} />
      ))}
    </div>
  );
}

function CarriedItem({
  item: w,
  project,
  teams,
}: {
  item: WorkItem;
  project: string;
  teams: Team[];
}) {
  const author = agentOf(teams, w.deploymentId, w.authorInstance);
  const title = useEntryTitle(project, w.cardRef);
  return (
    <span className="ml-9 flex items-center gap-2 text-[13px] text-tx2">
      <Avatar look={author?.soul.look} size={20} />
      {title.data ?? (w.cardRef || w.branch)}
      <span className="font-mono text-xs text-tx4">{w.branch}</span>
    </span>
  );
}

function MergedRow({
  mr,
  project,
  main,
  teams,
  items,
}: {
  mr: MergeRequest;
  project: string;
  /** The main it merged into: its repo's. */
  main: string;
  teams: Team[];
  items: WorkItem[];
}) {
  const title = useEntryTitle(project, mr.entry);
  return (
    <div className="flex flex-col gap-1.5">
      <span className="flex items-center gap-2">
        <span className="text-ok">✓</span>
        <span className="font-medium">{title.data ?? mr.entry}</span>
        {mr.forgePrNumber > 0 && <span className="text-tx3">PR #{mr.forgePrNumber}</span>}
        <span className="text-[13px] text-tx4">
          {mr.repo} · merged to {main} · {when(mr.mergedAt)}
        </span>
      </span>
      {items.map((w) => (
        <CarriedItem key={w.id} item={w} project={project} teams={teams} />
      ))}
    </div>
  );
}
