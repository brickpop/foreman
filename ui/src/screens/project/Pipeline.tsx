import { useNavigate } from "@tanstack/react-router";
import {
  isNotBuilt,
  type MergeRequest,
  type Project,
  type Request,
  type Team,
  type WorkItem,
} from "../../api/client";
import { agentOf, inbox, mainOf, when } from "../../app/model";
import {
  useAnswer,
  useEntryTitle,
  useInstances,
  usePipeline,
  useRequests,
  useTeams,
} from "../../app/queries";
import { Avatar } from "../../components/Avatar";
import { RequestPanel } from "../../components/RequestPanel";
import { Button, Empty, NotBuilt, Panel, SectionLabel } from "../../components/ui";

// A project's pipeline (wiki/product/features/changes.md): what needs the CEO
// opens its Overview (a review links to its page, wiki/product/features/reviews.md); the work moving towards main, and what recently merged,
// open its Repositories tab.

/** Where a request came from, as the panel's header says it. */
export function sourceLabel(r: Request): string {
  if (r.kind === "blocked") return "raised by Foreman";
  if (r.kind === "review") return "request_merge()";
  return `escalate(${r.kind})`;
}

const stageLabels = ["in worktree", "handed to lead", "lead reviewing", "integrated"];

/** What waits on the CEO on this project, first on its Overview, but its
 * reviews (in Reviews); nothing when nothing does. */
export function NeedsYou({ project }: { project: Project }) {
  const requests = useRequests();
  const teams = useTeams();
  const open = inbox(requests.data ?? []).filter((r) => r.projectId === project.id);
  if (open.length === 0) return null;
  return (
    <section className="flex flex-col gap-2.5 px-12 pt-6">
      <SectionLabel>Needs you</SectionLabel>
      {open.map((r) => (
        <ProjectRequest key={r.id} request={r} project={project} teams={teams.data ?? []} />
      ))}
    </section>
  );
}

/** The work on its way to main, and what merged in the last 7 days: first on
 * the Repositories tab. */
export function InFlight({ project }: { project: Project }) {
  const pipeline = usePipeline(project.id);
  const teams = useTeams();
  const instances = useInstances();
  const mrs = pipeline.data?.mergeRequests ?? [];
  const items = pipeline.data?.workItems ?? [];
  const live = new Set((instances.data ?? []).map((i) => i.deploymentId));
  const weekAgo = Date.now() - 7 * 24 * 3600 * 1000;
  const merged = mrs
    .filter((m) => m.state === "merged" && Date.parse(m.mergedAt) >= weekAgo)
    .sort((a, b) => b.mergedAt.localeCompare(a.mergedAt))
    .slice(0, 20);
  const openMrs = mrs.filter((m) => m.state === "open");
  // In flight on their own: every work item not carried by an open or merged merge
  // request, integrated ones included (they wait for the lead's request_merge).
  const carried = (w: (typeof items)[number]) =>
    mrs.some((m) => m.id === w.mergeRequestId && (m.state === "open" || m.state === "merged"));
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
        {loose.length === 0 && openMrs.length === 0 ? (
          <Empty>No work in flight.</Empty>
        ) : (
          <Panel>
            {loose.map((w) => (
              <WorkRow
                key={w.id}
                item={w}
                project={project.id}
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
  const agent = agentOf(teams, r.instanceId);
  const openSession = () =>
    navigate({ to: "/sessions", search: { deployment: r.deploymentId, agent: r.instanceId } });
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

function WorkRow({
  item: w,
  project,
  teams,
  live,
}: {
  item: WorkItem;
  project: string;
  teams: Team[];
  live: boolean;
}) {
  const navigate = useNavigate();
  const author = agentOf(teams, w.authorInstance);
  const title = useEntryTitle(project, w.cardRef);
  const tone =
    w.checksStatus === "fail" ? "text-err" : w.checksStatus === "pass" ? "text-ok" : "text-tx3";
  return (
    <div className="grid grid-cols-[auto_1fr_auto_auto] items-center gap-3 border-b border-line px-4 py-3 last:border-b-0">
      <Avatar look={author?.soul.look} size={26} />
      <span className="flex min-w-0 flex-col">
        <span className="truncate font-medium">{title.data ?? (w.cardRef || w.branch)}</span>
        <span className="truncate text-[12.5px] text-tx3">
          {author?.soul.name ?? w.authorInstance} · <span className="font-mono">{w.branch}</span>
          {w.note && <span className={tone}> · {w.note}</span>}
        </span>
      </span>
      <span className="rounded-[5px] bg-raised px-2 py-[3px] text-[12.5px] text-tx2">
        {stageLabels[w.stage] ?? w.stage}
      </span>
      {live ? (
        <Button
          variant="small"
          onClick={() =>
            navigate({
              to: "/sessions",
              search: { deployment: w.deploymentId, agent: w.authorInstance },
            })
          }
        >
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
  const lead = agentOf(teams, mr.authorInstance);
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
            <span className="font-mono">{mr.branch}</span> → {main}
            {mr.checksStatus && ` · checks ${mr.checksStatus}`}
            {mr.note && <span className={mr.conflicting ? "text-err" : ""}> · {mr.note}</span>}
          </span>
        </span>
        <span className="rounded-[5px] bg-raised px-2 py-[3px] text-[12.5px] text-tx2">
          {mr.forgePrNumber ? "PR open" : "merge requested"}
        </span>
        <Button
          variant="small"
          onClick={() => navigate({ to: "/reviews/$review", params: { review: mr.reviewId } })}
        >
          Review
        </Button>
        {mr.forgePrUrl ? (
          <Button variant="small" onClick={() => window.open(mr.forgePrUrl, "_blank", "noopener")}>
            Open PR ↗
          </Button>
        ) : live ? (
          <Button
            variant="small"
            onClick={() =>
              navigate({
                to: "/sessions",
                search: { deployment: mr.deploymentId, agent: mr.authorInstance },
              })
            }
          >
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
  const author = agentOf(teams, w.authorInstance);
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
