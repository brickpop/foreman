import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { useClient } from "../api/client";
import { harnessLabel, profileOf } from "../app/model";
import {
  keys,
  useDeployments,
  useProfiles,
  useProjects,
  useSettings,
  useSetup,
  useTeams,
} from "../app/queries";
import { ConnectDialog } from "./ConnectDialog";
import { Modal } from "./Overlays";
import { useToast } from "./Toasts";
import { Button } from "./ui";

// Assigns a team to a project, with an optional goal
// (wiki/product/features/deploy-team.md).
// Opened with a team preset (Team page), a project preset (Project settings),
// or neither (empty Sessions). The project's profile, not connected, is connected from here.

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
  project: presetProject,
}: {
  open: boolean;
  onClose: () => void;
  team?: string;
  project?: string;
}) {
  const client = useClient();
  const queries = useQueryClient();
  const navigate = useNavigate();
  const toast = useToast();
  const teams = useTeams();
  const projects = useProjects();
  const deployments = useDeployments();
  const [team, setTeam] = useState(presetTeam);
  const [project, setProject] = useState(presetProject);
  const [goal, setGoal] = useState("");
  const [connecting, setConnecting] = useState<string>();
  // The project's profile: its agents run on it, so it must be connected first.
  const settings = useSettings().data?.settings;
  const profiles = useProfiles();
  const profileName = project ? profileOf(settings, project) : undefined;
  const profile = profiles.data?.find((p) => p.name === profileName);
  const profileReady = profile?.connected ?? false;
  // Agents start only once Podman runs (and the instances are restored).
  const running = useSetup().data?.find((s) => s.kind === "running");
  const engineDown = running !== undefined && running.state !== "done";
  const already = (p: string) =>
    (deployments.data ?? []).some((d) => d.teamId === team && d.projectId === p);
  const teamName = teams.data?.find((t) => t.id === team)?.config.name ?? team;
  const deploy = useMutation({
    mutationFn: () => client.agents.deploy(team ?? "", project ?? "", goal.trim()),
    onSuccess: (d) => {
      queries.invalidateQueries({ queryKey: keys.instances });
      queries.invalidateQueries({ queryKey: keys.deployments });
      const t = teams.data?.find((x) => x.id === d.teamId);
      const lead = `${t?.config.lead}@${d.projectId}`;
      const members = Object.keys(t?.config.agents ?? {}).length;
      onClose();
      setGoal("");
      navigate({ to: "/sessions", search: { deployment: d.id, agent: lead } });
      toast({
        title: `${t?.config.name ?? d.teamId} deployed to ${d.projectId}`,
        body: `${members} new sessions starting.`,
        color: "var(--ok)",
        onOpen: () => navigate({ to: "/sessions", search: { deployment: d.id, agent: lead } }),
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
          Starts a separate set of sessions for this project. Deployments on other projects aren't
          affected.
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
                {there ? `${p.id} (already there)` : p.id}
              </button>
            );
          })}
        </Group>
        <label className="flex flex-col gap-1.5">
          <span className="text-[13px] font-medium text-tx4">Goal on this project (optional)</span>
          <textarea
            rows={3}
            value={goal}
            onChange={(e) => setGoal(e.target.value)}
            placeholder="What does done look like? Or leave it empty and give it to the lead in its session."
            className="resize-y rounded-lg border border-line2 bg-term px-3 py-2 text-tx placeholder:text-tx4"
          />
        </label>
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
              Setup on Home
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
