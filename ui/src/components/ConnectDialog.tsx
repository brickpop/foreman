import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { useClient } from "../api/client";
import { harnessLabel } from "../app/model";
import { keys } from "../app/queries";
import { Modal } from "./Overlays";
import { Terminal } from "./Terminal";
import { Button } from "./ui";

// A profile's harness login, in a terminal (wiki/product/features/settings.md#profiles):
// from Settings, the setup checklist and the Deploy dialog. Mount it only
// while open: opening starts the login, closing ends it, logged in or not.

export function ConnectDialog({ profile, onClose }: { profile: string; onClose: () => void }) {
  const client = useClient();
  const queries = useQueryClient();
  const login = useMutation({ mutationFn: () => client.profiles.connect(profile) });
  // biome-ignore lint/correctness/useExhaustiveDependencies: one login per opening
  useEffect(() => login.mutate(), []);
  // Followed while open, so the dialog says when the login landed.
  const status = useQuery({
    queryKey: keys.profiles,
    queryFn: () => client.profiles.list(),
    refetchInterval: 2000,
  });
  const found = status.data?.find((p) => p.name === profile);
  const connected = found?.connected ?? false;
  const close = () => {
    client.profiles.endLogin(profile).catch(() => {}); // already gone is fine
    queries.invalidateQueries({ queryKey: keys.profiles });
    queries.invalidateQueries({ queryKey: keys.setup });
    onClose();
  };
  const label = harnessLabel(found?.harness ?? "");
  return (
    <Modal open onClose={close} title={`Connect ${profile}`} width={760} buttonsOnly>
      <div className="flex flex-col gap-3">
        <p className="m-0 text-[13.5px] text-tx3">
          {label}’s own login, in its container. For Claude Code, type{" "}
          <span className="font-mono text-tx2">/login</span> and follow it. Every workspace’s
          projects on the <span className="font-mono text-tx2">{profile}</span> profile use it.
        </p>
        <div className="h-[380px] overflow-hidden rounded-[10px] border border-line bg-term">
          {login.data && <Terminal instance={login.data} />}
        </div>
        {login.error && <p className="m-0 text-[13px] text-err">{login.error.message}</p>}
        <div className="flex items-center justify-end gap-3">
          <span className={`flex-1 text-[13px] ${connected ? "text-ok" : "text-tx4"}`}>
            {connected ? `${profile} is connected.` : "Not connected yet."}
          </span>
          <Button variant={connected ? "primary" : "secondary"} onClick={close}>
            {connected ? "Done" : "Cancel"}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
