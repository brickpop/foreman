import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import type { ActivityScope, Deployment, Team } from "../../api/client";
import { pageEntries } from "../../app/activity";
import { when } from "../../app/model";
import { useActivity, useDeployments, useStoppedDeployments, useTeams } from "../../app/queries";
import { ActionRow, namesOf, ThreadRow } from "../../components/ActivityPanel";
import { Chevron } from "../../components/icons";
import { Button, Empty, Panel, SectionLabel, Segmented } from "../../components/ui";

// The project's Activity tab (wiki/product/features/project-activity.md): its
// teams' messages and git actions as one timeline, newest first, 50 at a
// time; the stopped deployments' below, in Older sessions.

const teamName = (teams: Team[], d: Deployment) =>
  teams.find((t) => t.id === d.teamId)?.config.name ?? d.teamId;

export function Activity({ project }: { project: string }) {
  const navigate = useNavigate();
  const teams = useTeams().data ?? [];
  const active = (useDeployments().data ?? []).filter((d) => d.projectId === project);
  const stopped = (useStoppedDeployments().data ?? []).filter((d) => d.projectId === project);
  const [only, setOnly] = useState("all");
  // One team's chip pages that deployment alone, so each page stays full.
  const shown = active.find((d) => d.id === only);
  const scope: ActivityScope = shown ? { deployment: shown.id } : { project };
  return (
    <div className="flex flex-col gap-9 px-12 pt-6 pb-16">
      <section className="flex flex-col gap-2.5">
        {active.length > 1 && (
          <Segmented
            label="Team"
            value={shown ? only : "all"}
            onChange={setOnly}
            options={[
              { value: "all", label: "All teams" },
              ...active.map((d) => ({ value: d.id, label: teamName(teams, d) })),
            ]}
          />
        )}
        {active.length === 0 ? (
          <Empty>No team is deployed on {project}. Deploy one from its Settings tab.</Empty>
        ) : (
          <Timeline
            scope={scope}
            teams={teams}
            label={(deployment) => {
              const d = active.find((x) => x.id === deployment);
              return d && teamName(teams, d);
            }}
            onOpen={(deployment, instance) =>
              navigate({ to: "/sessions", search: { deployment, agent: instance } })
            }
          />
        )}
      </section>
      {stopped.length > 0 && <OlderSessions deployments={stopped} teams={teams} />}
    </div>
  );
}

/** A timeline a page at a time. label names a row's team; onOpen opens the
 * session a thread came from (none for an older session: it has none). */
function Timeline({
  scope,
  teams,
  label,
  onOpen,
}: {
  scope: ActivityScope;
  teams: Team[];
  label?: (deployment: string) => string | undefined;
  onOpen?: (deployment: string, instance: string) => void;
}) {
  const activity = useActivity(scope);
  const name = namesOf(teams);
  const list = pageEntries(activity.data?.pages ?? []);
  if (activity.isPending) return null;
  if (activity.isError) {
    return <p className="m-0 text-[13px] text-err">{activity.error.message}</p>;
  }
  if (list.length === 0) return <Empty>Nothing yet.</Empty>;
  return (
    <div className="flex flex-col gap-3">
      <Panel>
        {list.map((e) => {
          const deployment = "thread" in e ? e.thread.message.deploymentId : e.action.deploymentId;
          return "thread" in e ? (
            <ThreadRow
              key={`m${e.thread.message.id}`}
              thread={e.thread}
              name={name}
              team={label?.(deployment)}
              onOpen={onOpen && ((instance) => onOpen(deployment, instance))}
            />
          ) : (
            <ActionRow
              key={`a${e.action.id}`}
              action={e.action}
              name={name}
              team={label?.(deployment)}
            />
          );
        })}
      </Panel>
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
              <span className="text-[13px] text-tx4">
                stopped {when(d.stoppedAt)}
                {d.goal && ` · ${d.goal}`}
              </span>
            </button>
            {shown === d.id && <Timeline scope={{ deployment: d.id }} teams={teams} />}
          </div>
        ))}
    </section>
  );
}
