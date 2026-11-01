import { Link, useParams, useSearch } from "@tanstack/react-router";
import { useState } from "react";
import type { Deployment, Team as TeamData } from "../api/client";
import { useOpenChiefOfStaff } from "../app/chiefOfStaff";
import { roleLabel, roster } from "../app/model";
import { useDeployments, useHomeFolder, useInstances, useRequests, useTeams } from "../app/queries";
import { Avatar } from "../components/Avatar";
import { DeployDialog } from "../components/DeployDialog";
import { StopIcon } from "../components/icons";
import { StopDialog } from "../components/StopDialog";
import { TeamPlaybook } from "../components/TeamPlaybook";
import { Button, Dot, Empty, Loading, Panel } from "../components/ui";

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
  if (teams.isPending) return <Loading rows={5} className="m-12" />;
  if (!team) return <p className="p-12 text-tx3">No team {id}.</p>;
  const mine = (deployments.data ?? []).filter((d) => d.teamId === id);
  return (
    <div className="flex flex-col">
      <header className="flex flex-col gap-4 px-6 pt-6">
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
          <Button onClick={() => openChiefOfStaff({ start: "hire", team: id })}>Hire</Button>
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
  const instances = useInstances();
  const requests = useRequests();
  const [stopping, setStopping] = useState<Deployment>();
  const sessions = (d: Deployment) => (instances.data ?? []).filter((i) => i.deploymentId === d.id);
  return (
    <section className="flex w-full max-w-[920px] flex-col gap-3 px-6 pt-6 pb-12">
      {deployments.length === 0 ? (
        <Empty>Not deployed anywhere. Deploy it to a project to start sessions.</Empty>
      ) : (
        deployments.map((d) => {
          const working = sessions(d).filter((i) => i.status === "running").length;
          const waiting = (requests.data ?? []).filter(
            (r) => r.status === "open" && r.deploymentId === d.id,
          ).length;
          return (
            // The whole card opens the deployment's sessions: their link
            // stretches over it, and the project name and Stop sit above it.
            <Panel
              key={d.id}
              className="relative hover:border-line3 has-[[data-card]:focus-visible]:outline-2 has-[[data-card]:focus-visible]:outline-acc"
            >
              <div className="flex items-center gap-4 px-5 py-4">
                <div className="flex flex-1 flex-col items-start gap-0.5">
                  <Link
                    to="/projects/$project/wiki"
                    params={{ project: d.projectId }}
                    className="relative z-10 text-[15px] font-semibold text-tx hover:underline"
                  >
                    {d.projectId}
                  </Link>
                  <Link
                    to="/projects/$project/session"
                    params={{ project: d.projectId }}
                    data-card
                    aria-label={`Open the sessions of ${team.config.name} on ${d.projectId}`}
                    className="text-[13px] text-tx4 outline-none after:absolute after:inset-0"
                  >
                    {working} working{waiting > 0 && ` · ${waiting} waiting on you`}
                  </Link>
                </div>
                <Button
                  variant="danger"
                  className="relative z-10 inline-flex items-center gap-1.5"
                  onClick={() => setStopping(d)}
                >
                  <StopIcon />
                  Stop
                </Button>
              </div>
            </Panel>
          );
        })
      )}
      {stopping && (
        <StopDialog
          deployment={stopping}
          teamName={team.config.name}
          onClose={() => setStopping(undefined)}
        />
      )}
    </section>
  );
}

function Members({ team, deployments }: { team: TeamData; deployments: Deployment[] }) {
  const instances = useInstances();
  const ids = roster(team.config);
  return (
    <section className="flex w-full max-w-[920px] flex-col px-6 pt-5 pb-12">
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
                <span className="font-normal text-tx4"> · {roleLabel(soul.role)}</span>
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
  return (
    <section className="flex w-full max-w-[760px] flex-col gap-6 px-6 pt-7 pb-12">
      <p className="m-0 text-tx2">
        The playbook is part of the team definition, so every deployment uses the same one. It's
        added to each member's session prompt. Each project's own gates and checks apply on top.
      </p>
      <TeamPlaybook team={team.config} />
    </section>
  );
}

function File({ team }: { team: TeamData }) {
  const home = useHomeFolder();
  return (
    <section className="flex w-full max-w-[860px] flex-col gap-3 px-6 pt-7 pb-12">
      <span className="font-mono text-[12.5px] text-tx3">
        {home.data?.path ?? "~/.foreman"}/config/teams/{team.id}.toml
      </span>
      <pre className="m-0 overflow-x-auto rounded-[10px] border border-line bg-term p-3.5 font-mono text-[12.5px] leading-[1.6] text-tx2">
        {team.source}
      </pre>
    </section>
  );
}
