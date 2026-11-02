import { Link } from "@tanstack/react-router";
import { isNotBuilt, type Review, type Team } from "../../api/client";
import { agentOf, inbox, when } from "../../app/model";
import { reviewOf } from "../../app/project";
import { useRequests, useReviews, useTeams } from "../../app/queries";
import { reviewGroups, statusLabel } from "../../app/reviews";
import { Empty, Loading, NotBuilt, Panel, SectionTitle } from "../../components/ui";
import { useProject } from "../Project";
import { ProjectRequest } from "./Pipeline";

// A project's Inbox (wiki/product/features/reviews.md#the-inbox): what waits
// on the CEO there (the reviews to decide, then the other requests), what
// waits on the lead, and the last reviews done. A review opens inside it.
export function Inbox() {
  const project = useProject();
  const reviews = useReviews();
  const teams = useTeams();
  const requests = inbox(useRequests().data ?? []).filter((r) => r.projectId === project.id);
  const groups = reviewGroups((reviews.data ?? []).filter((v) => v.projectId === project.id));
  const rows = (shown: Review[]) =>
    shown.length > 0 && (
      <Panel>
        {shown.map((v) => (
          <ReviewRow key={v.id} review={v} teams={teams.data ?? []} />
        ))}
      </Panel>
    );
  const list = (title: string, shown: Review[]) =>
    shown.length > 0 && (
      <section className="flex flex-col gap-3">
        <SectionTitle count={shown.length}>{title}</SectionTitle>
        {rows(shown)}
      </section>
    );
  if (isNotBuilt(reviews.error)) return <NotBuilt what="reviews" />;
  const needsYou = groups.needsYou.length + requests.length;
  return (
    <div className="flex max-w-[920px] flex-col gap-8 px-6 pt-6 pb-12">
      {/* Always shown: what the CEO opens the Inbox for, even when nothing is there. */}
      <section className="flex flex-col gap-3">
        <SectionTitle count={reviews.isPending ? undefined : needsYou}>Needs you</SectionTitle>
        {reviews.error ? (
          <p className="m-0 text-err">{reviews.error.message}</p>
        ) : reviews.isPending ? (
          <Loading rows={3} />
        ) : needsYou === 0 ? (
          <Empty>Nothing needs you yet.</Empty>
        ) : (
          <>
            {rows(groups.needsYou)}
            {requests.map((r) => (
              <ProjectRequest key={r.id} request={r} project={project} teams={teams.data ?? []} />
            ))}
          </>
        )}
      </section>
      {list("Waiting on the lead", groups.waiting)}
      {list("Done", groups.done)}
    </div>
  );
}

function ReviewRow({ review: v, teams }: { review: Review; teams: Team[] }) {
  const lead = agentOf(teams, v.deploymentId, v.lead);
  const repos = v.mergeRequests.map((m) => m.repo).join(", ");
  return (
    <Link
      {...reviewOf(v.projectId, v.id)}
      className="grid grid-cols-[1fr_auto] items-center gap-3 border-b border-line px-4 py-3 text-tx last:border-b-0 hover:bg-hover"
    >
      <span className="flex min-w-0 flex-col">
        <span className="truncate font-medium">{v.title || v.entry || v.branch}</span>
        <span className="truncate text-[12.5px] text-tx3">
          {lead?.soul.name ?? v.lead} · {repos} · <span className="font-mono">{v.branch}</span>
        </span>
      </span>
      <span className="flex flex-col items-end gap-0.5">
        <span className="text-[13px] text-tx2">{statusLabel(v.status)}</span>
        <span className="text-xs text-tx4">{when(v.updatedAt)}</span>
      </span>
    </Link>
  );
}
