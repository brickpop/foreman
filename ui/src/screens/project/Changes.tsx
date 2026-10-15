import { useNavigate } from "@tanstack/react-router";
import type { MergeRequest, Project, Request, Team, WorkItem } from "../../api/client";
import { useOpenChiefOfStaff } from "../../app/chiefOfStaff";
import { agentOf, ordered } from "../../app/model";
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
import { ReviewPanel } from "../../components/ReviewPanel";
import { Button, Empty, Panel, SectionLabel } from "../../components/ui";

// The project's actionable tab (wiki/product/features/changes.md): what needs
// the CEO, work moving towards main, and what recently merged.

/** Where a request came from, as the panel's header says it. */
export function sourceLabel(r: Request): string {
  if (r.source === "cos") return "Chief of Staff";
  if (r.kind === "blocked") return "raised by Foreman";
  if (r.kind === "review") return "request_merge()";
  return `escalate(${r.kind})`;
}

const stageLabels = ["in worktree", "handed to lead", "lead reviewing", "integrated"];

export function Changes({ project }: { project: Project }) {
  const requests = useRequests();
  const pipeline = usePipeline(project.id);
  const teams = useTeams();
  const instances = useInstances();
  const open = ordered(
    (requests.data ?? []).filter((r) => r.status === "open" && r.projectId === project.id),
  );
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
  return (
    <div className="mx-auto flex w-full max-w-[860px] flex-col gap-9 px-10 pt-7 pb-16">
      <section className="flex flex-col gap-2.5">
        <SectionLabel>Needs you</SectionLabel>
        {open.length === 0 ? (
          <Empty>Nothing needs you on this project.</Empty>
        ) : (
          open.map((r) => (
            <ChangesRequest
              key={r.id}
              request={r}
              project={project}
              teams={teams.data ?? []}
              mr={mrs.find((m) => m.id === r.mergeRequestId)}
            />
          ))
        )}
      </section>
      <section className="flex flex-col gap-2.5">
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
                teams={teams.data ?? []}
                items={items.filter((w) => w.mergeRequestId === m.id)}
                live={live.has(m.deploymentId)}
              />
            ))}
          </Panel>
        )}
      </section>
      {merged.length > 0 && (
        <section className="flex flex-col gap-2.5">
          <SectionLabel>Merged</SectionLabel>
          {merged.map((m) => (
            <MergedRow
              key={m.id}
              mr={m}
              project={project.id}
              teams={teams.data ?? []}
              items={items.filter((w) => w.mergeRequestId === m.id)}
            />
          ))}
        </section>
      )}
    </div>
  );
}

function ChangesRequest({
  request: r,
  project,
  teams,
  mr,
}: {
  request: Request;
  project: Project;
  teams: Team[];
  mr?: MergeRequest;
}) {
  const navigate = useNavigate();
  const answer = useAnswer();
  const openChiefOfStaff = useOpenChiefOfStaff();
  const agent = agentOf(teams, r.instanceId);
  const title = useEntryTitle(project.id, mr?.entry ?? "");
  const openSession = () =>
    navigate({ to: "/sessions", search: { deployment: r.deploymentId, agent: r.instanceId } });
  const fromChiefOfStaff = r.source === "cos";
  const context =
    fromChiefOfStaff || !agent ? `on ${project.id}` : `${agent.team.config.name} on ${project.id}`;
  const who = agent && { name: agent.soul.name, look: agent.soul.look };
  if (r.kind === "review" && mr) {
    return (
      <ReviewPanel
        request={r}
        mr={mr}
        // The project repo's own forge for a config change, the code repo's otherwise.
        forge={
          mr.repo === "project" && !project.config.forge.projectRepo
            ? "none"
            : project.config.forge.kind
        }
        who={who}
        context={context}
        // A Chief of Staff change says what it proposes in its own words.
        reviewText={
          fromChiefOfStaff
            ? undefined
            : `${agent?.soul.name ?? "The lead"} wants ${title.data ?? mr.entry} merged into main in ${mr.repo}.`
        }
        author={fromChiefOfStaff ? "the Chief of Staff" : (agent?.soul.name ?? "the lead")}
        variant="changes"
        source={sourceLabel(r)}
        onOpenSession={openSession}
        onOpenChiefOfStaff={() => openChiefOfStaff()}
      />
    );
  }
  return (
    <RequestPanel
      request={r}
      variant="changes"
      source={sourceLabel(r)}
      who={who}
      context={context}
      onAnswer={(text) => answer.mutate({ request: r, answer: text })}
      answering={answer.isPending}
      onOpenSession={openSession}
      onOpenChiefOfStaff={() => openChiefOfStaff()}
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
  teams,
  items,
  live,
}: {
  mr: MergeRequest;
  project: string;
  teams: Team[];
  items: WorkItem[];
  live: boolean;
}) {
  const navigate = useNavigate();
  const lead = agentOf(teams, mr.authorInstance);
  const title = useEntryTitle(project, mr.entry);
  return (
    <div className="flex flex-col gap-2 border-b border-line px-4 py-3 last:border-b-0">
      <div className="grid grid-cols-[auto_1fr_auto_auto] items-center gap-3">
        <Avatar look={lead?.soul.look} size={26} />
        <span className="flex min-w-0 flex-col">
          <span className="truncate font-medium">
            {title.data ?? mr.entry}
            {mr.forgePrNumber > 0 && <span className="text-tx3"> PR #{mr.forgePrNumber}</span>}
          </span>
          <span className="truncate text-[12.5px] text-tx3">
            {lead?.soul.name ?? mr.authorInstance} · {mr.repo} ·{" "}
            <span className="font-mono">{mr.branch}</span> → main · checks{" "}
            {mr.checksStatus || "running"}
            {mr.note && ` · ${mr.note}`}
          </span>
        </span>
        <span className="rounded-[5px] bg-raised px-2 py-[3px] text-[12.5px] text-tx2">
          {mr.forgePrNumber ? "PR open" : "merge requested"}
        </span>
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
  teams,
  items,
}: {
  mr: MergeRequest;
  project: string;
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
        <span className="text-[13px] text-tx4">Merged to main at {mr.mergedAt.slice(11, 16)}</span>
      </span>
      {items.map((w) => (
        <CarriedItem key={w.id} item={w} project={project} teams={teams} />
      ))}
    </div>
  );
}
