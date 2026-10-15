import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, useParams, useSearch } from "@tanstack/react-router";
import { useState } from "react";
import { type Deployment, type Team as TeamData, useClient } from "../api/client";
import { useOpenChiefOfStaff } from "../app/chiefOfStaff";
import { harnessLabel, plural, resolveWho, roleLabel, roster } from "../app/model";
import {
  keys,
  useDeployments,
  useInstances,
  useRequests,
  useTeams,
  useWorkspace,
} from "../app/queries";
import { Avatar } from "../components/Avatar";
import { DeployDialog } from "../components/DeployDialog";
import { Modal } from "../components/Overlays";
import { useToast } from "../components/Toasts";
import { Button, Dot, Empty, Panel } from "../components/ui";

export type TeamTab = "deployments" | "members" | "playbook" | "file";

const tabs: { tab: TeamTab; label: string }[] = [
  { tab: "deployments", label: "Deployments" },
  { tab: "members", label: "Members" },
  { tab: "playbook", label: "Playbook" },
  { tab: "file", label: "File" },
];

// One team definition (wiki/product/features/team-page.md): where it is
// deployed, its members, its playbook and its file.
export function Team() {
  const { team: id } = useParams({ from: "/teams/$team" });
  const { tab } = useSearch({ from: "/teams/$team" });
  const teams = useTeams();
  const deployments = useDeployments();
  const openChiefOfStaff = useOpenChiefOfStaff();
  const [deploy, setDeploy] = useState(false);
  const team = teams.data?.find((t) => t.id === id);
  if (teams.isPending) return null;
  if (!team) return <p className="p-12 text-tx3">No team {id} in this workspace.</p>;
  const mine = (deployments.data ?? []).filter((d) => d.teamId === id);
  return (
    <div className="flex flex-col">
      <header className="flex flex-col gap-4 px-12 pt-10">
        <div className="flex items-end gap-4">
          <div className="flex flex-1 flex-col gap-1">
            <h1 className="m-0 text-[30px] font-semibold tracking-[-0.02em]">{team.config.name}</h1>
            {team.config.purpose && (
              <p className="m-0 text-[15px] text-tx3">{team.config.purpose}</p>
            )}
            <p className="m-0 text-[13.5px] text-tx4">
              {mine.length === 0
                ? "Not deployed"
                : `Deployed on ${mine.length} project${mine.length === 1 ? "" : "s"}`}
            </p>
          </div>
          <Button onClick={() => openChiefOfStaff({ flow: "hire", team: id })}>Hire</Button>
          <Button variant="primary" onClick={() => setDeploy(true)}>
            Deploy to a project
          </Button>
        </div>
        <nav className="flex border-b border-line">
          {tabs.map((t) => (
            <Link
              key={t.tab}
              to="/teams/$team"
              params={{ team: id }}
              search={{ tab: t.tab }}
              className={`-mb-px border-b-2 px-3.5 py-[9px] font-medium ${
                t.tab === tab ? "border-acc text-tx" : "border-transparent text-tx3 hover:text-tx2"
              }`}
            >
              {t.label}
            </Link>
          ))}
        </nav>
      </header>
      {tab === "deployments" && <Deployments team={team} deployments={mine} />}
      {tab === "members" && <Members team={team} deployments={mine} />}
      {tab === "playbook" && <Playbook team={team} />}
      {tab === "file" && <File team={team} />}
      {/* Mounted only while open: each opening starts fresh, on this team. */}
      {deploy && <DeployDialog open onClose={() => setDeploy(false)} team={id} />}
    </div>
  );
}

function Deployments({ team, deployments }: { team: TeamData; deployments: Deployment[] }) {
  const navigate = useNavigate();
  const client = useClient();
  const queries = useQueryClient();
  const toast = useToast();
  const instances = useInstances();
  const requests = useRequests();
  const [stopping, setStopping] = useState<Deployment>();
  const stop = useMutation({
    mutationFn: (d: Deployment) => client.agents.stop(d.id),
    onSuccess: (_r, d) => {
      queries.invalidateQueries({ queryKey: keys.instances });
      queries.invalidateQueries({ queryKey: keys.deployments });
      setStopping(undefined);
      toast({
        title: `${team.config.name} stopped on ${d.projectId}`,
        body: "Worktrees are kept. Other deployments keep running.",
        color: "var(--tx4)",
        onOpen: () =>
          navigate({
            to: "/projects/$project",
            params: { project: d.projectId },
            search: { tab: "settings" },
          }),
      });
    },
  });
  const sessions = (d: Deployment) => (instances.data ?? []).filter((i) => i.deploymentId === d.id);
  return (
    <section className="flex w-full max-w-[920px] flex-col gap-3 px-12 pt-6 pb-16">
      {deployments.length === 0 ? (
        <Empty>Not deployed anywhere. Deploy it to a project to start sessions.</Empty>
      ) : (
        deployments.map((d) => {
          const working = sessions(d).filter((i) => i.status === "running").length;
          const waiting = (requests.data ?? []).filter(
            (r) => r.status === "open" && r.deploymentId === d.id,
          ).length;
          return (
            <Panel key={d.id} className="hover:border-line3">
              {/* biome-ignore lint/a11y/noStaticElementInteractions: the whole card opens the project; the title link is the keyboard path */}
              {/* biome-ignore lint/a11y/useKeyWithClickEvents: the title link is the keyboard path */}
              <div
                className="flex cursor-pointer items-center gap-4 px-5 py-4"
                onClick={(e) => {
                  // The buttons and the link inside do their own thing.
                  if ((e.target as HTMLElement).closest("a, button")) return;
                  navigate({
                    to: "/projects/$project",
                    params: { project: d.projectId },
                    search: { tab: "overview" },
                  });
                }}
              >
                <div className="flex flex-1 flex-col gap-0.5">
                  <Link
                    to="/projects/$project"
                    params={{ project: d.projectId }}
                    search={{ tab: "overview" }}
                    className="text-[15px] font-semibold text-tx"
                  >
                    {d.projectId}
                  </Link>
                  <span className="text-tx2">{d.goal}</span>
                  <span className="text-[13px] text-tx4">
                    {working} working{waiting > 0 && ` · ${waiting} waiting on you`}
                  </span>
                </div>
                <Button
                  variant="raised"
                  onClick={() => navigate({ to: "/sessions", search: { deployment: d.id } })}
                >
                  Sessions
                </Button>
                <Button
                  variant="danger"
                  onClick={() => {
                    stop.reset(); // no error left from another deployment
                    setStopping(d);
                  }}
                >
                  Stop
                </Button>
              </div>
            </Panel>
          );
        })
      )}
      <Modal
        open={stopping !== undefined}
        onClose={() => setStopping(undefined)}
        title={`Stop ${team.config.name} on ${stopping?.projectId ?? ""}?`}
      >
        <div className="flex flex-col gap-4">
          <p className="m-0 text-tx2">
            {plural(stopping ? sessions(stopping).length : 0, "session")} end. Worktrees and
            branches are kept.
          </p>
          {stop.error && <p className="m-0 text-[13px] text-err">{stop.error.message}</p>}
          <div className="flex justify-end gap-2">
            <Button onClick={() => setStopping(undefined)}>Cancel</Button>
            <Button
              variant="danger"
              disabled={stop.isPending}
              onClick={() => stopping && stop.mutate(stopping)}
            >
              Stop
            </Button>
          </div>
        </div>
      </Modal>
    </section>
  );
}

function Members({ team, deployments }: { team: TeamData; deployments: Deployment[] }) {
  const instances = useInstances();
  const ids = roster(team.config);
  return (
    <section className="flex w-full max-w-[920px] flex-col px-12 pt-5 pb-16">
      {ids.map((agent) => {
        const soul = team.config.agents[agent];
        if (!soul) return null;
        return (
          <Link
            key={agent}
            to="/teams/$team/agents/$agent"
            params={{ team: team.id, agent }}
            className="grid grid-cols-[auto_1fr_auto] items-center gap-3.5 border-b border-line px-1.5 py-3.5 text-tx hover:bg-hover"
          >
            <Avatar look={soul.look} size={38} />
            <span className="flex min-w-0 flex-col">
              <span className="font-semibold">
                {soul.name}
                <span className="font-normal text-tx4">
                  {" "}
                  · {roleLabel(soul.role)} · {harnessLabel(soul.harness)}
                </span>
              </span>
              <span className="truncate text-[13.5px] text-tx3">{soul.personality}</span>
            </span>
            <span className="flex gap-3">
              {deployments.map((d) => {
                const inst = instances.data?.find((i) => i.id === `${agent}@${d.projectId}`);
                return (
                  <span key={d.id} className="flex items-center gap-1.5 text-[13px] text-tx3">
                    <Dot status={inst?.status ?? "stopped"} />
                    {d.projectId}
                  </span>
                );
              })}
            </span>
          </Link>
        );
      })}
    </section>
  );
}

function Playbook({ team }: { team: TeamData }) {
  const who = (w: string) => {
    const r = resolveWho(team, w);
    return r.known ? (
      r.text
    ) : (
      <span className="flex items-center gap-1" title="No member matches this; check the team file">
        {r.text} <span className="text-acc">⚠</span>
      </span>
    );
  };
  return (
    <section className="flex w-full max-w-[760px] flex-col gap-6 px-12 pt-7 pb-16">
      <p className="m-0 text-tx2">
        The playbook is part of the team definition, so every deployment uses the same one. It's
        added to each member's session prompt. Each project's own gates and checks apply on top.
      </p>
      <div className="grid grid-cols-[28px_110px_1fr] gap-x-3 gap-y-2.5">
        {team.config.playbook.flatMap((step, i) =>
          Object.entries(step).map(([w, text]) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: a step has no id; its place in the playbook is its identity
            <div key={`${i}-${w}`} className="contents">
              <span className="font-mono text-tx4">{i + 1}</span>
              <span className="font-semibold">{who(w)}</span>
              <span className="text-tx2">{text}</span>
            </div>
          )),
        )}
      </div>
      {team.config.escalate.length > 0 && (
        <div className="flex flex-col gap-2">
          <span className="text-[13px] font-medium text-tx4">Escalate</span>
          {team.config.escalate.map((e) => (
            <span key={e.when} className="flex gap-2">
              <span className="text-tx2">{e.when}</span>
              <span>→</span>
              <span className={`font-medium ${e.to === "ceo" ? "text-accTx" : ""}`}>
                {who(e.to)}
              </span>
            </span>
          ))}
        </div>
      )}
    </section>
  );
}

function File({ team }: { team: TeamData }) {
  const workspace = useWorkspace();
  return (
    <section className="flex w-full max-w-[860px] flex-col gap-3 px-12 pt-7 pb-16">
      <span className="font-mono text-[12.5px] text-tx3">
        {workspace.data?.path ?? "~"}/.foreman/teams/{team.id}.toml
      </span>
      <pre className="m-0 overflow-x-auto rounded-[10px] border border-line bg-term p-3.5 font-mono text-[12.5px] leading-[1.6] text-tx2">
        {team.source}
      </pre>
    </section>
  );
}
