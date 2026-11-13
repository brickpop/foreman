import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { useClient } from "../api/client";
import { byName, harnessLabel, projectName, replacingLine } from "../app/model";
import { sessionOf } from "../app/project";
import {
  keys,
  useContinuing,
  useDeployments,
  useInstances,
  useProfiles,
  useProjectName,
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
// Session and Team sections), or neither (Dashboard). The project's
// profile is picked here too (saved on Deploy), and connected from here.

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
  const projectNameOf = useProjectName();
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
  // Another one picked here becomes the project's on Deploy.
  const profiles = useProfiles();
  const chosen = projects.data?.find((p) => p.id === project);
  const [picked, setPicked] = useState<string>();
  const profileName = picked ?? chosen?.profile;
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
  // Agents that worked here before continue their conversations, unless the
  // CEO starts fresh; offered only when there are some.
  // None continues on another harness: its conversations are the old one's.
  const saved = profiles.data?.find((p) => p.name === chosen?.profile);
  const sameHarness = !saved || saved.harness === profile?.harness;
  const known = useContinuing(team, project).data ?? [];
  const continuing = sameHarness ? known : [];
  const [fresh, setFresh] = useState(false);
  const deploy = useMutation({
    mutationFn: async () => {
      if (picked && picked !== chosen?.profile) {
        await client.home.setProfile(project ?? "", picked);
        await queries.invalidateQueries({ queryKey: keys.projects });
      }
      return client.agents.deploy(team ?? "", project ?? "", fresh && continuing.length > 0);
    },
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
        title: `${t?.config.name ?? d.teamId} deployed to ${projectNameOf(d.projectId)}`,
        body: `${members} new sessions starting.`,
        color: "var(--ok)",
        onOpen: () => navigate(sessionOf(lead)),
      });
    },
  });
  const ready = team && project && !already(project) && profileReady && !engineDown;
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
          {byName(projects.data ?? []).map((p) => {
            const there = already(p.id);
            return (
              <button
                key={p.id}
                type="button"
                disabled={there}
                aria-pressed={p.id === project}
                className={option(p.id === project, there)}
                onClick={() => {
                  setProject(p.id);
                  setPicked(undefined);
                }}
              >
                {there ? `${projectName([p], p.id)} (already there)` : projectName([p], p.id)}
                {on(p.id) && !there && (
                  <span className="text-tx4"> · {nameOf(on(p.id)?.teamId ?? "")}</span>
                )}
              </button>
            );
          })}
        </Group>
        {replaced && team && (
          <p className="m-0 rounded-lg border border-attnLine bg-attnSoft px-3 py-2.5 text-[13px] text-attnTx">
            {replacingLine(nameOf(replaced.teamId), projectNameOf(replaced.projectId), ending)}
          </p>
        )}
        {project && !already(project) && continuing.length > 0 && (
          <label className="flex items-start gap-2 text-[13px] text-tx2">
            <input
              type="checkbox"
              className="mt-0.5"
              checked={fresh}
              onChange={(e) => setFresh(e.target.checked)}
            />
            <span>
              Start fresh: the agents don’t continue their earlier conversations here (kept on
              disk).
            </span>
          </label>
        )}
        {profileName && profiles.data && (
          <Group label="Profile">
            {profiles.data.map((p) => (
              <button
                key={p.name}
                type="button"
                aria-pressed={p.name === profileName}
                className={option(p.name === profileName)}
                onClick={() => setPicked(p.name)}
              >
                <span className="font-mono">{p.name}</span>
                <span className="text-tx4"> · {harnessLabel(p.harness)}</span>
              </button>
            ))}
            {/* The one the project names, even when that profile is gone. */}
            {!profile && (
              <button type="button" aria-pressed className={`${option(true)} !border-err`}>
                <span className="font-mono">{profileName}</span>
              </button>
            )}
            <p className="m-0 flex w-full items-center gap-2 pt-0.5 text-[13px] text-tx3">
              {!profile
                ? "That profile doesn’t exist: pick another."
                : profile.connected
                  ? picked && picked !== chosen?.profile
                    ? "It becomes the project’s profile on Deploy."
                    : "The project’s profile: its agents run on it."
                  : "Not connected yet, so its agents can’t start."}
              {profile && !profile.connected && (
                <Button variant="small" onClick={() => setConnecting(profile.name)}>
                  Connect
                </Button>
              )}
            </p>
          </Group>
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
