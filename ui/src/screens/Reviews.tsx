import { Link } from "@tanstack/react-router";
import { isNotBuilt, type Review, type Team } from "../api/client";
import { agentOf, when } from "../app/model";
import { useReviews, useTeams } from "../app/queries";
import { reviewGroups, statusLabel } from "../app/reviews";
import { Avatar } from "../components/Avatar";
import { Empty, Loading, NotBuilt, PageTitle, Panel, SectionLabel } from "../components/ui";

// Every review across projects (wiki/product/features/reviews.md): what needs
// the CEO, what waits on a lead, and the last ones done. A row opens its page.
export function Reviews() {
  const reviews = useReviews();
  const teams = useTeams();
  const groups = reviewGroups(reviews.data ?? []);
  const sections = [
    { label: "Needs you", list: groups.needsYou },
    { label: "Waiting on the lead", list: groups.waiting },
    { label: "Done", list: groups.done },
  ].filter((s) => s.list.length > 0);
  return (
    <div className="mx-auto flex w-full max-w-[920px] flex-col gap-10 px-12 pt-12 pb-16">
      <PageTitle title="Reviews" subtitle="The leads' branches on their way to main." />
      {isNotBuilt(reviews.error) ? (
        <NotBuilt what="reviews" />
      ) : reviews.error ? (
        <p className="text-err">{reviews.error.message}</p>
      ) : reviews.isPending ? (
        <Loading rows={4} />
      ) : sections.length === 0 ? (
        <Empty>No reviews yet. A lead's request_merge() opens one.</Empty>
      ) : (
        sections.map((s) => (
          <section key={s.label} className="flex flex-col gap-2.5">
            <SectionLabel>{s.label}</SectionLabel>
            <Panel>
              {s.list.map((v) => (
                <ReviewRow key={v.id} review={v} teams={teams.data ?? []} />
              ))}
            </Panel>
          </section>
        ))
      )}
    </div>
  );
}

function ReviewRow({ review: v, teams }: { review: Review; teams: Team[] }) {
  const lead = agentOf(teams, v.lead);
  const repos = v.mergeRequests.map((m) => m.repo).join(", ");
  return (
    <Link
      to="/reviews/$review"
      params={{ review: v.id }}
      className="grid grid-cols-[auto_1fr_auto] items-center gap-3 border-b border-line px-4 py-3 text-tx last:border-b-0 hover:bg-hover"
    >
      <Avatar look={lead?.soul.look} size={26} />
      <span className="flex min-w-0 flex-col">
        <span className="truncate font-medium">{v.title || v.entry || v.branch}</span>
        <span className="truncate text-[12.5px] text-tx3">
          {v.projectId} · {lead?.soul.name ?? v.lead} · {repos} ·{" "}
          <span className="font-mono">{v.branch}</span>
        </span>
      </span>
      <span className="flex flex-col items-end gap-0.5">
        <span className="text-[13px] text-tx2">{statusLabel(v.status)}</span>
        <span className="text-xs text-tx4">{when(v.updatedAt)}</span>
      </span>
    </Link>
  );
}
