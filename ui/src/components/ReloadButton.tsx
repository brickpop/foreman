import { useMutation } from "@tanstack/react-query";
import { type Instance, useClient } from "../api/client";
import { reloadLine, reloadOf } from "../app/model";
import { useTeams } from "../app/queries";
import { ReloadIcon } from "./icons";
import { useToast } from "./Toasts";
import { Button, type ButtonProps } from "./ui";

// A deployment's Reload (wiki/product/models/team-definition.md#reload):
// restarts its agents on an older prompt, each continuing its conversation.
// Disabled while there is nothing to reload or an agent is busy, its title
// saying which.
export function ReloadButton({
  instances,
  variant,
}: {
  /** The deployment's instances. */
  instances: Instance[];
  variant?: ButtonProps["variant"];
}) {
  const client = useClient();
  const toast = useToast();
  const teams = useTeams();
  const reload = reloadOf(instances);
  const restart = useMutation({
    mutationFn: () => Promise.all(reload.behind.map((i) => client.agents.restart(i.id))),
    onError: (e: Error) =>
      toast({ title: "Agents not reloaded", body: e.message, color: "var(--err)" }),
  });
  return (
    <Button
      variant={variant}
      className="relative z-10 inline-flex items-center gap-1.5"
      disabled={reload.behind.length === 0 || !!reload.busy || restart.isPending}
      title={reloadLine(reload, teams.data ?? [])}
      onClick={() => restart.mutate()}
    >
      <ReloadIcon />
      Reload
    </Button>
  );
}
