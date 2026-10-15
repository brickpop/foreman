import { Link, useNavigate } from "@tanstack/react-router";
import type { Request, Team } from "../api/client";
import { useOpenChiefOfStaff } from "../app/chiefOfStaff";
import {
  agentOf,
  checksSentence,
  homeSubtitle,
  homeTitle,
  ordered,
  projectRows,
} from "../app/model";
import {
  useAnswer,
  useEntryTitle,
  useInstances,
  usePipeline,
  useProjects,
  useRequests,
  useTeams,
} from "../app/queries";
import { DemoBadge } from "../components/DemoBadge";
import { RequestPanel } from "../components/RequestPanel";
import { Empty, PageTitle, SectionLabel } from "../components/ui";

// The CEO's inbox (wiki/product/features/home.md): every open request across
// projects, then one line per project.
export function Home() {
  const requests = useRequests();
  const instances = useInstances();
  const projects = useProjects();
  const teams = useTeams();
  const open = ordered((requests.data ?? []).filter((r) => r.status === "open"));
  const rows = projectRows(
    projects.data ?? [],
    teams.data ?? [],
    instances.data ?? [],
    requests.data ?? [],
  );
  return (
    <div className="mx-auto flex w-full max-w-[920px] flex-col gap-10 px-12 pt-12 pb-16">
      <PageTitle
        title={
          <span className="flex items-center gap-3">
            {homeTitle(open.length)}
            <DemoBadge areas={["requests", "agents", "workspace"]} />
          </span>
        }
        subtitle={homeSubtitle(instances.data ?? [])}
      />
      <section className="flex flex-col gap-2.5">
        {open.length === 0 ? (
          <Empty>Nothing needs you. You'll get a browser notification when something does.</Empty>
        ) : (
          open.map((r) => <HomeRequest key={r.id} request={r} teams={teams.data ?? []} />)
        )}
      </section>
      <section className="flex flex-col">
        <SectionLabel>Projects</SectionLabel>
        <div className="mt-2">
          {rows.map((row) => (
            <Link
              key={row.id}
              to="/projects/$project"
              params={{ project: row.id }}
              search={{ tab: "overview" }}
              className="grid grid-cols-[1fr_auto] items-center border-t border-line px-1 py-3.5 text-tx hover:bg-hover"
            >
              <span className="flex flex-col gap-0.5">
                <span className="text-[15px] font-semibold">{row.id}</span>
                <span className="text-[13.5px] text-tx3">{row.line}</span>
              </span>
              <span
                className={`text-[13px] ${
                  row.state === "needs you"
                    ? "text-accTx"
                    : row.state === "on track"
                      ? "text-ok"
                      : "text-tx4"
                }`}
              >
                {row.state}
              </span>
            </Link>
          ))}
        </div>
      </section>
    </div>
  );
}

function HomeRequest({ request: r, teams }: { request: Request; teams: Team[] }) {
  const navigate = useNavigate();
  const answer = useAnswer();
  const openChiefOfStaff = useOpenChiefOfStaff();
  const agent = agentOf(teams, r.instanceId);
  // A Chief of Staff change to a team file belongs to no project.
  const where = r.projectId ? `on ${r.projectId}` : "";
  const context = r.source === "cos" || !agent ? where : `${agent.team.config.name} ${where}`;
  return (
    <RequestPanel
      request={r}
      who={agent && { name: agent.soul.name, look: agent.soul.look }}
      context={context}
      reviewText={useReviewText(r, agent?.soul.name)}
      onAnswer={(text) => answer.mutate({ id: r.id, answer: text })}
      onOpenSession={() =>
        navigate({ to: "/sessions", search: { deployment: r.deploymentId, agent: r.instanceId } })
      }
      onReview={
        // A Chief of Staff item shows its options right here (home.md#request-list).
        r.source === "cos"
          ? undefined
          : () =>
              navigate({
                to: "/projects/$project",
                params: { project: r.projectId },
                search: { tab: "changes" },
              })
      }
      onOpenChiefOfStaff={() => openChiefOfStaff()}
    />
  );
}

/** A review's sentence: `<Lead> wants <entry title> merged into main in <repo>. Checks pass.` */
function useReviewText(r: Request, lead?: string): string | undefined {
  const pipeline = usePipeline(r.kind === "review" ? r.projectId : "");
  const mr = pipeline.data?.mergeRequests.find((m) => m.id === r.mergeRequestId);
  const title = useEntryTitle(r.projectId, mr?.entry ?? "");
  if (r.kind !== "review" || !mr) return undefined;
  const what = title.data ?? mr.entry;
  return `${lead ?? "The lead"} wants ${what} merged into main in ${mr.repo}.${checksSentence(mr.checksStatus)}`;
}
