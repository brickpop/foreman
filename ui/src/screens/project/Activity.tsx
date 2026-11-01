import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import type { Deployment, Instance, Team } from "../../api/client";
import { type Filter, pageEntries, shows } from "../../app/activity";
import { readable, when } from "../../app/model";
import { sessionOf } from "../../app/project";
import {
  useActivity,
  useDeployments,
  useInstances,
  useMessages,
  useStoppedDeployments,
  useTeams,
} from "../../app/queries";
import { rosterOrder } from "../../app/sessions";
import { Chevron } from "../../components/icons";
import { ActionRow, CallMap, FilterStrip, namesOf, ThreadRow } from "../../components/TeamActivity";
import { Button, Empty, Loading, Panel, SectionLabel } from "../../components/ui";
import { useProject } from "../Project";

// A project's Activity section (wiki/product/features/project-activity.md):
// its team's messages and git actions as one timeline, newest first, 50 at a
// time, with the map of who calls whom beside it, filtering it; the stopped
// deployments' timelines below, in Older sessions.

const teamName = (teams: Team[], d: Deployment) =>
  teams.find((t) => t.id === d.teamId)?.config.name ?? d.teamId;

export function Activity() {
  const project = useProject().id;
  const navigate = useNavigate();
  const teams = useTeams().data ?? [];
  const instances = useInstances().data ?? [];
  const d = useDeployments().data?.find((x) => x.projectId === project);
  const stopped = (useStoppedDeployments().data ?? []).filter((x) => x.projectId === project);
  const [filter, setFilter] = useState<Filter>(null);
  const name = namesOf(teams, d?.id ?? "");
  return (
    <div className="flex flex-col gap-8 px-6 pt-6 pb-12">
      {d ? (
        <div className="flex items-start gap-6 max-[1100px]:flex-col-reverse">
          <section className="flex min-w-0 flex-1 flex-col gap-2.5">
            {filter && <FilterStrip filter={filter} name={name} onClear={() => setFilter(null)} />}
            <Timeline
              deployment={d.id}
              teams={teams}
              filter={filter}
              onOpen={(instance) => navigate(sessionOf(instance))}
            />
          </section>
          <TeamMap
            deployment={d.id}
            roster={rosterOrder(d.id, instances, teams)}
            filter={filter}
            onFilter={setFilter}
            name={name}
          />
        </div>
      ) : (
        <Empty>No team is on {readable(project)}. Deploy one from its Deployment section.</Empty>
      )}
      {stopped.length > 0 && <OlderSessions deployments={stopped} teams={teams} />}
    </div>
  );
}

/** Who calls whom on the team, beside the timeline (above it on a narrow
 * window), staying in view as the timeline scrolls. */
function TeamMap({
  deployment,
  roster,
  filter,
  onFilter,
  name,
}: {
  deployment: string;
  roster: Instance[];
  filter: Filter;
  onFilter: (f: Filter) => void;
  name: ReturnType<typeof namesOf>;
}) {
  const messages = useMessages(deployment).data ?? [];
  return (
    <aside className="sticky top-4 shrink-0 self-start max-[1100px]:static max-[1100px]:self-stretch">
      <Panel>
        <CallMap
          roster={roster}
          messages={messages}
          filter={filter}
          onFilter={onFilter}
          name={name}
        />
      </Panel>
    </aside>
  );
}

/** A deployment's timeline a page at a time, filtered as the map says.
 * onOpen opens the session a thread came from (none for an older session:
 * it has none). */
function Timeline({
  deployment,
  teams,
  filter = null,
  onOpen,
}: {
  deployment: string;
  teams: Team[];
  filter?: Filter;
  onOpen?: (instance: string) => void;
}) {
  const activity = useActivity(deployment);
  const name = namesOf(teams, deployment);
  const all = pageEntries(activity.data?.pages ?? []);
  const list = all.filter((e) => shows(e, filter));
  if (activity.isPending) return <Loading rows={5} />;
  if (activity.isError) {
    return <p className="m-0 text-[13px] text-err">{activity.error.message}</p>;
  }
  return (
    <div className="flex flex-col gap-3">
      {list.length === 0 ? (
        // Filtered, the pages not loaded yet may hold more.
        <Empty>{all.length === 0 ? "Nothing yet." : "Nothing here in what's loaded."}</Empty>
      ) : (
        <Panel>
          {list.map((e) =>
            "thread" in e ? (
              <ThreadRow
                key={`m${e.thread.message.id}`}
                thread={e.thread}
                name={name}
                onOpen={onOpen}
              />
            ) : (
              <ActionRow key={`a${e.action.id}`} action={e.action} name={name} />
            ),
          )}
        </Panel>
      )}
      {activity.hasNextPage && (
        <Button
          className="self-start"
          disabled={activity.isFetchingNextPage}
          onClick={() => activity.fetchNextPage()}
        >
          Show more
        </Button>
      )}
    </div>
  );
}

/** The stopped deployments, collapsed: each opens its own timeline. */
function OlderSessions({ deployments, teams }: { deployments: Deployment[]; teams: Team[] }) {
  const [open, setOpen] = useState(false);
  const [shown, setShown] = useState<string>();
  return (
    <section className="flex flex-col gap-2.5">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        aria-expanded={open}
        className="flex items-center gap-1.5 self-start"
      >
        <Chevron open={open} />
        <SectionLabel>Older sessions · {deployments.length}</SectionLabel>
      </button>
      {open &&
        deployments.map((d) => (
          <div key={d.id} className="flex flex-col gap-2.5">
            <button
              type="button"
              onClick={() => setShown(shown === d.id ? undefined : d.id)}
              aria-expanded={shown === d.id}
              className="flex items-center gap-2 self-start text-left"
            >
              <Chevron open={shown === d.id} />
              <span className="font-semibold">{teamName(teams, d)}</span>
              <span className="text-[13px] text-tx4">stopped {when(d.stoppedAt)}</span>
              {d.stopReason && (
                <span className="text-[13px] text-err">by Foreman: {d.stopReason}</span>
              )}
            </button>
            {shown === d.id && <Timeline deployment={d.id} teams={teams} />}
          </div>
        ))}
    </section>
  );
}
