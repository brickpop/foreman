import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { useClient } from "../api/client";
import { harnessLabel, replacingLine } from "../app/model";
import { sessionOf } from "../app/project";
import {
  keys,
  useDeployments,
  useInstances,
  useProfiles,
  useProjects,
  useSetup,
  useTeams,
} from "../app/queries";
import { ConnectDialog } from "./ConnectDialog";
import { Modal } from "./Overlays";
import { useToast } from "./Toasts";
import { Button } from "./ui";

// Assigns a team to a project (wiki/product/features/deploy-team.md). A
// project has one team at most: another one there is replaced, and the dialog
// says so first.
// Opened with a team preset (Team page, Dashboard), a project preset (its
// Session and Deployment sections), or neither (Dashboard). The project's
// profile, not connected, is connected from here.

const option = (on: boolean, disabled = false) =>
  `rounded-[7px] border px-3 py-1.5 text-[13px] ${
    disabled
      ? "border-line2 text-tx4"
      : on
        ? "border-acc bg-accSoft text-tx"
        : "border-line2 text-tx2 hover:border-line3"
  }`;

export function DeployDialog({
  open,
  onClose,
  team: presetTeam,
  initialTeam,
  project: presetProject,
}: {
  open: boolean;
  onClose: () => void;
  /** The team to deploy, not to be changed. */
  team?: string;
  /** The team chosen at first, to be changed (the last one on the project). */
  initialTeam?: string;
  project?: string;
}) {
  const client = useClient();
  const queries = useQueryClient();
  const navigate = useNavigate();
  const toast = useToast();
  const teams = useTeams();
  const projects = useProjects();
  const deployments = useDeployments();
  const instances = useInstances();
  const [team, setTeam] = useState(presetTeam ?? initialTeam);
  const [project, setProject] = useState(presetProject);
  const [connecting, setConnecting] = useState<string>();
  // The project's profile: its agents run on it, so it must be connected first.
  const profiles = useProfiles();
  const chosen = projects.data?.find((p) => p.id === project);
  const profileName = chosen?.profile;
  // An archived project takes no deployment: unarchived from here, as from its settings.
  const unarchive = useMutation({
    mutationFn: () => client.home.setArchived(project ?? "", false),
    onSettled: () => queries.invalidateQueries({ queryKey: keys.projects }),
  });
  const profile = profiles.data?.find((p) => p.name === profileName);
  const profileReady = profile?.connected ?? false;
  // Agents start only once Podman runs (and the instances are restored).
  const running = useSetup().data?.find((s) => s.kind === "running");
  const engineDown = running !== undefined && running.state !== "done";
  const nameOf = (id: string) => teams.data?.find((t) => t.id === id)?.config.name || id;
  // The team on each project now (one at most): deploying another replaces it.
  const on = (p?: string) => (deployments.data ?? []).find((d) => d.projectId === p);
  const already = (p: string) => on(p)?.teamId === team;
  const teamName = team && nameOf(team);
  const replaced = project && !already(project) ? on(project) : undefined;
  const ending = (instances.data ?? []).filter((i) => i.deploymentId === replaced?.id).length;
  const deploy = useMutation({
    mutationFn: () => client.agents.deploy(team ?? "", project ?? ""),
    onSuccess: async (d) => {
      // Its agents loaded first: until then the Session section doesn't know
      // the team, and would offer to deploy one.
      await Promise.all([
        queries.invalidateQueries({ queryKey: keys.instances }),
        queries.invalidateQueries({ queryKey: keys.deployments }),
      ]);
      const t = teams.data?.find((x) => x.id === d.teamId);
      const lead = `${t?.config.lead}@${d.projectId}`;
      const members = Object.keys(t?.config.agents ?? {}).length;
      onClose();
      navigate(sessionOf(lead));
      toast({
        title: `${t?.config.name ?? d.teamId} deployed to ${d.projectId}`,
        body: `${members} new sessions starting.`,
        color: "var(--ok)",
        onOpen: () => navigate(sessionOf(lead)),
      });
    },
  });
  const ready =
    team && project && !already(project) && !chosen?.archived && profileReady && !engineDown;
  return (
    <Modal
      open={open}
      onClose={onClose}
      title={presetTeam ? `Deploy ${teamName}` : "Deploy a team"}
    >
      <div className="flex flex-col gap-4">
        <p className="m-0 text-[13.5px] text-tx3">
          A project has one team at most. The team's work here is separate from its work on other
          projects.
        </p>
        {!presetTeam && (
          <Group label="Team">
            {(teams.data ?? []).map((t) => (
              <button
                key={t.id}
                type="button"
                aria-pressed={t.id === team}
                className={option(t.id === team)}
                onClick={() => setTeam(t.id)}
              >
                {t.config.name}
              </button>
            ))}
          </Group>
        )}
        <Group label="Project">
          {(projects.data ?? []).map((p) => {
            const there = already(p.id);
            return (
              <button
                key={p.id}
                type="button"
                disabled={there}
                aria-pressed={p.id === project}
                className={option(p.id === project, there)}
                onClick={() => setProject(p.id)}
              >
                {there ? `${p.id} (already there)` : p.archived ? `${p.id} (archived)` : p.id}
                {on(p.id) && !there && (
                  <span className="text-tx4"> · {nameOf(on(p.id)?.teamId ?? "")}</span>
                )}
              </button>
            );
          })}
        </Group>
        {replaced && team && (
          <p className="m-0 rounded-lg border border-attnLine bg-attnSoft px-3 py-2.5 text-[13px] text-attnTx">
            {replacingLine(nameOf(replaced.teamId), replaced.projectId, ending)}
          </p>
        )}
        {chosen?.archived && (
          <div className="flex items-center gap-3 rounded-lg border border-line2 px-3 py-2.5 text-[13px] text-tx2">
            <span className="flex-1">
              <span className="font-mono">{chosen.id}</span> is archived, so no team can be deployed
              to it.
              {unarchive.error && <span className="text-err"> {unarchive.error.message}</span>}
            </span>
            <Button
              variant="small"
              disabled={unarchive.isPending}
              onClick={() => unarchive.mutate()}
            >
              Unarchive
            </Button>
          </div>
        )}
        {profileName && profiles.data && (
          <div className="flex items-center gap-3 rounded-lg border border-line2 px-3 py-2.5 text-[13px] text-tx2">
            <span className="flex-1">
              {!profile ? (
                <>
                  The <span className="font-mono">{profileName}</span> profile doesn’t exist: pick
                  another in the project’s settings.
                </>
              ) : (
                <>
                  Its agents run on the <span className="font-mono">{profile.name}</span> profile (
                  {harnessLabel(profile.harness)})
                  {profile.connected ? "." : ", which isn’t connected yet, so they can’t start."}
                </>
              )}
            </span>
            {profile && !profile.connected && (
              <Button variant="small" onClick={() => setConnecting(profile.name)}>
                Connect
              </Button>
            )}
          </div>
        )}
        {engineDown && (
          <div className="flex items-center gap-3 rounded-lg border border-line2 px-3 py-2.5 text-[13px] text-tx2">
            <span className="flex-1">Podman isn’t running yet, so agents can’t start.</span>
            <Link to="/" onClick={onClose} className="text-accTx hover:underline">
              Setup on the Dashboard
            </Link>
          </div>
        )}
        {deploy.error && <p className="m-0 text-[13px] text-err">{deploy.error.message}</p>}
        <div className="flex justify-end gap-2">
          <Button onClick={onClose}>Cancel</Button>
          <Button
            variant="primary"
            disabled={!ready || deploy.isPending}
            onClick={() => deploy.mutate()}
          >
            Deploy
          </Button>
        </div>
      </div>
      {connecting && (
        <ConnectDialog profile={connecting} onClose={() => setConnecting(undefined)} />
      )}
    </Modal>
  );
}

function Group({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <fieldset className="m-0 flex flex-col gap-1.5 border-0 p-0">
      <legend className="mb-1.5 text-[13px] font-medium text-tx4">{label}</legend>
      <div className="flex flex-wrap gap-1.5">{children}</div>
    </fieldset>
  );
}
