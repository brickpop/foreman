import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { useClient } from "../api/client";
import { keys, useDeployments, useProjects, useTeams } from "../app/queries";
import { Modal } from "./Overlays";
import { useToast } from "./Toasts";
import { Button } from "./ui";

// Assigns a team to a project with a goal (wiki/product/features/deploy-team.md).
// Opened with a team preset (Team page) or a project preset (Project settings).

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
  const ready = team && project && goal.trim() && !already(project);
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
          <span className="text-[13px] font-medium text-tx4">Goal on this project</span>
          <textarea
            rows={3}
            value={goal}
            onChange={(e) => setGoal(e.target.value)}
            placeholder="What does done look like?"
            className="resize-y rounded-lg border border-line2 bg-term px-3 py-2 text-tx placeholder:text-tx4"
          />
        </label>
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
