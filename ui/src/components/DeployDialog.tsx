import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { useClient } from "../api/client";
import { hasTeam, profileChoices, projectChoices, teamChoices } from "../app/deploy";
import { harnessLabel, replacingLine } from "../app/model";
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
import { Select } from "./Select";
import { useToast } from "./Toasts";
import { Button, Field } from "./ui";

// Assigns a team to a project (wiki/product/features/deploy-team.md). A
// project has one team at most: another one there is replaced, and the dialog
// says so first.
// Opened with a team preset (Team page, Dashboard), a project preset (its
// Session and Team sections), or neither (Dashboard). The project's
// profile is picked here too (saved on Deploy), and connected from here.

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
  const already = (p: string) => hasTeam(deployments.data ?? [], p, team);
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
  // Under the profile: what picking it means, or what is missing, and Connect.
  const profileHint = (
    <span className="flex items-center gap-2">
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
    </span>
  );
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
          <Field label="Team">
            {(field) => (
              <Select
                label="Team"
                field={field}
                value={team ?? ""}
                placeholder="Pick a team"
                options={teamChoices(teams.data ?? [])}
                onChange={setTeam}
              />
            )}
          </Field>
        )}
        <Field label="Project">
          {(field) => (
            <Select
              label="Project"
              field={field}
              value={project ?? ""}
              placeholder="Pick a project"
              options={projectChoices(projects.data ?? [], deployments.data ?? [], team, nameOf)}
              onChange={(p) => {
                setProject(p);
                setPicked(undefined);
              }}
            />
          )}
        </Field>
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
          <Field label="Profile" hint={profileHint}>
            {(field) => (
              <Select
                label="Profile"
                field={field}
                mono
                value={profileName}
                options={profileChoices(profiles.data, harnessLabel)}
                onChange={setPicked}
              />
            )}
          </Field>
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
