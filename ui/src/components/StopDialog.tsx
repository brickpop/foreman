import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import type { Deployment } from "../api/client";
import { useClient } from "../api/client";
import { sessionsEnd } from "../app/model";
import { keys, useInstances, useProjectName } from "../app/queries";
import { StopIcon } from "./icons";
import { Modal } from "./Overlays";
import { useToast } from "./Toasts";
import { Button } from "./ui";

// Stopping a deployment, after a confirmation (wiki/product/features/team-page.md):
// from the Team page and the project's Team section. Mount it only while open, so each
// opening starts without the last one's error.

export function StopDialog({
  deployment: d,
  teamName,
  onClose,
}: {
  deployment: Deployment;
  teamName: string;
  onClose: () => void;
}) {
  const projectNameOf = useProjectName();
  const client = useClient();
  const queries = useQueryClient();
  const navigate = useNavigate();
  const toast = useToast();
  const instances = useInstances();
  const sessions = (instances.data ?? []).filter((i) => i.deploymentId === d.id).length;
  const stop = useMutation({
    mutationFn: () => client.agents.stop(d.id),
    onSuccess: () => {
      queries.invalidateQueries({ queryKey: keys.instances });
      queries.invalidateQueries({ queryKey: keys.deployments });
      onClose();
      toast({
        title: `${teamName} stopped on ${projectNameOf(d.projectId)}`,
        body: "Worktrees are kept. Other deployments keep running.",
        color: "var(--tx4)",
        onOpen: () => navigate({ to: "/projects/$project/team", params: { project: d.projectId } }),
      });
    },
  });
  return (
    <Modal open onClose={onClose} title={`Stop ${teamName} on ${projectNameOf(d.projectId)}?`}>
      <div className="flex flex-col gap-4">
        <p className="m-0 text-tx2">{sessionsEnd(sessions)}. Worktrees and branches are kept.</p>
        {stop.error && <p className="m-0 text-[13px] text-err">{stop.error.message}</p>}
        <div className="flex justify-end gap-2">
          <Button onClick={onClose}>Cancel</Button>
          <Button
            variant="danger"
            className="inline-flex items-center gap-1.5"
            disabled={stop.isPending}
            onClick={() => stop.mutate()}
          >
            <StopIcon />
            Stop
          </Button>
        </div>
      </div>
    </Modal>
  );
}
