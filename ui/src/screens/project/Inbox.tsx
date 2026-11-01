import { Link } from "@tanstack/react-router";
import { isNotBuilt, type Review, type Team } from "../../api/client";
import { agentOf, inbox, when } from "../../app/model";
import { reviewOf } from "../../app/project";
import { useRequests, useReviews, useTeams } from "../../app/queries";
import { reviewGroups, statusLabel } from "../../app/reviews";
import { Avatar } from "../../components/Avatar";
import { Empty, Loading, NotBuilt, Panel, SectionLabel } from "../../components/ui";
import { useProject } from "../Project";
import { NeedsYou } from "./Pipeline";

// A project's Inbox (wiki/product/features/reviews.md#the-inbox): what waits
// on the CEO there (the reviews to decide, then the other requests), what
// waits on the lead, and the last reviews done. A review opens inside it.
export function Inbox() {
  const project = useProject();
  const reviews = useReviews();
  const teams = useTeams();
  const requests = inbox(useRequests().data ?? []).filter((r) => r.projectId === project.id);
  const groups = reviewGroups((reviews.data ?? []).filter((v) => v.projectId === project.id));
  const list = (label: string, shown: Review[]) =>
    shown.length > 0 && (
      <section className="flex flex-col gap-2.5">
        <SectionLabel>{label}</SectionLabel>
        <Panel>
          {shown.map((v) => (
            <ReviewRow key={v.id} review={v} teams={teams.data ?? []} />
          ))}
        </Panel>
      </section>
    );
  const none =
    groups.needsYou.length + groups.waiting.length + groups.done.length + requests.length === 0 &&
    !reviews.isPending;
  return (
    <div className="flex max-w-[920px] flex-col gap-8 px-6 pt-6 pb-12">
      {isNotBuilt(reviews.error) ? (
        <NotBuilt what="reviews" />
      ) : reviews.error ? (
        <p className="text-err">{reviews.error.message}</p>
      ) : reviews.isPending ? (
        <Loading rows={4} />
      ) : (
        list("Needs your review", groups.needsYou)
      )}
      <NeedsYou project={project} />
      {list("Waiting on the lead", groups.waiting)}
      {list("Done", groups.done)}
      {none && !reviews.error && (
        <Empty>
          Nothing waits on you on {project.id}. A lead's request_merge() opens a review.
        </Empty>
      )}
    </div>
  );
}

function ReviewRow({ review: v, teams }: { review: Review; teams: Team[] }) {
  const lead = agentOf(teams, v.deploymentId, v.lead);
  const repos = v.mergeRequests.map((m) => m.repo).join(", ");
  return (
    <Link
      {...reviewOf(v.projectId, v.id)}
      className="grid grid-cols-[auto_1fr_auto] items-center gap-3 border-b border-line px-4 py-3 text-tx last:border-b-0 hover:bg-hover"
    >
      <Avatar look={lead?.soul.look} size={26} />
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
