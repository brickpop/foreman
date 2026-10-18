import { Link, useNavigate } from "@tanstack/react-router";
import { isNotBuilt, type Request, type Team } from "../api/client";
import {
  agentOf,
  checksSentence,
  forgeOf,
  homeSubtitle,
  homeTitle,
  ordered,
  projectRows,
} from "../app/model";
import {
  useAnswer,
  useEntryTitle,
  useInstances,
  useMergeRequest,
  usePipeline,
  useProjects,
  useRequests,
  useSetup,
  useTeams,
} from "../app/queries";
import { setupPending } from "../app/setup";
import { RequestPanel } from "../components/RequestPanel";
import { ReviewPanel } from "../components/ReviewPanel";
import { SetupChecklist } from "../components/SetupChecklist";
import { Empty, NotBuilt, PageTitle, SectionLabel } from "../components/ui";

// The CEO's inbox (wiki/product/features/home.md): every open request across
// projects, then one line per project.
export function Home() {
  const requests = useRequests();
  const instances = useInstances();
  const projects = useProjects();
  const teams = useTeams();
  const setup = useSetup();
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
        title={isNotBuilt(requests.error) ? "Home" : homeTitle(open.length)}
        subtitle={homeSubtitle(instances.data ?? [])}
      />
      {setup.data && setupPending(setup.data) && <SetupChecklist steps={setup.data} />}
      <section className="flex flex-col gap-2.5">
        {isNotBuilt(requests.error) ? (
          <NotBuilt what="requests" />
        ) : open.length === 0 ? (
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
  const projects = useProjects();
  const agent = agentOf(teams, r.instanceId);
  const main = projects.data?.find((p) => p.id === r.projectId)?.config.main ?? "main";
  const reviewText = useReviewText(r, main, agent?.soul.name);
  // A review is decided right here (home.md#request-list): its merge request
  // carries the change to look at.
  const mr = useMergeRequest(r.kind === "review" ? r.mergeRequestId : "");
  const where = `on ${r.projectId}`;
  const context = agent ? `${agent.team.config.name} ${where}` : where;
  const openSession = () =>
    navigate({ to: "/sessions", search: { deployment: r.deploymentId, agent: r.instanceId } });
  if (r.kind === "review" && mr.data) {
    const project = projects.data?.find((p) => p.id === r.projectId);
    return (
      <ReviewPanel
        request={r}
        mr={mr.data}
        main={main}
        forge={forgeOf(project)}
        who={agent && { name: agent.soul.name, look: agent.soul.look }}
        context={context}
        reviewText={reviewText}
        author={agent?.soul.name ?? "the lead"}
        variant="home"
        onOpenSession={openSession}
      />
    );
  }
  return (
    <RequestPanel
      request={r}
      who={agent && { name: agent.soul.name, look: agent.soul.look }}
      context={context}
      reviewText={reviewText}
      onAnswer={(text) => answer.mutate({ request: r, answer: text })}
      answering={answer.isPending}
      onOpenSession={openSession}
      onReview={() =>
        navigate({
          to: "/projects/$project",
          params: { project: r.projectId },
          search: { tab: "overview" },
        })
      }
    />
  );
}

/** A review's sentence: `<Lead> wants <entry title> merged into <main> in <repo>. Checks pass.` */
function useReviewText(r: Request, main: string, lead?: string): string | undefined {
  const pipeline = usePipeline(r.kind === "review" ? r.projectId : "");
  const mr = pipeline.data?.mergeRequests.find((m) => m.id === r.mergeRequestId);
  const title = useEntryTitle(r.projectId, mr?.entry ?? "");
  if (r.kind !== "review" || !mr) return undefined;
  const what = title.data ?? mr.entry;
  return `${lead ?? "The lead"} wants ${what} merged into ${main} in ${mr.repo}.${checksSentence(mr.checksStatus)}`;
}
