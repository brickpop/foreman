import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
  type ForgeAccount,
  isNotBuilt,
  type Settings as SettingsData,
  useClient,
} from "../../api/client";
import { keys } from "../../app/queries";
import { useToast } from "../../components/Toasts";
import { Button, NotBuilt, Segmented } from "../../components/ui";
import { inGlobalRepo, Section, useSave } from "./parts";

/** Settings → Git & forges: forge accounts, and where the global repo is hosted. */
export function GitTab({ settings }: { settings: SettingsData }) {
  return (
    <>
      <ForgeSection settings={settings} />
      <GlobalRepoSection settings={settings} />
    </>
  );
}

const forges = [
  { kind: "github", label: "GitHub", host: "https://github.com/" },
  { kind: "forgejo", label: "Forgejo", host: "https://" },
  { kind: "gitea", label: "Gitea", host: "https://" },
];

const forgeLabel = (kind: string) => forges.find((f) => f.kind === kind)?.label ?? kind;

function ForgeSection({ settings }: { settings: SettingsData }) {
  const client = useClient();
  const accounts = useQuery({ queryKey: keys.forge, queryFn: () => client.forge.accounts() });
  const [editing, setEditing] = useState<string>();
  return (
    <Section
      label="Forge accounts"
      kept="Global repo + this machine"
      intro="Tokens Foreman uses when you ask it to create a PR or sync a repo, and to watch those PRs. They stay on this machine. Each project’s config names its forge."
    >
      {isNotBuilt(accounts.error) ? (
        <NotBuilt what="forge accounts" />
      ) : (
        forges.map((f) => (
          <ForgeRow
            key={f.kind}
            forge={f}
            account={accounts.data?.find((a) => a.kind === f.kind)}
            url={settings.forges?.[f.kind]?.url ?? ""}
            editing={editing === f.kind}
            onEdit={(on) => setEditing(on ? f.kind : undefined)}
          />
        ))
      )}
    </Section>
  );
}

function ForgeRow({
  forge,
  account,
  url,
  editing,
  onEdit,
}: {
  forge: (typeof forges)[number];
  account?: ForgeAccount;
  url: string;
  editing: boolean;
  onEdit: (on: boolean) => void;
}) {
  const client = useClient();
  const queries = useQueryClient();
  const toast = useToast();
  const [host, setHost] = useState(url || forge.host);
  const [token, setToken] = useState("");
  const connect = useMutation({
    mutationFn: () => client.forge.connect(forge.kind, host.trim(), token),
    onSuccess: () => {
      setToken("");
      onEdit(false);
      queries.invalidateQueries({ queryKey: keys.forge });
      queries.invalidateQueries({ queryKey: keys.settings });
      toast({ title: `${forge.label} connected`, body: host.trim(), color: "var(--ok)" });
    },
  });
  return (
    <div className="flex flex-col gap-3">
      <div className="grid grid-cols-[130px_1fr_auto] items-center gap-4">
        <span className="font-medium">{forge.label}</span>
        <span className="truncate font-mono text-[13px] text-tx3">{url || "—"}</span>
        <span className="flex items-center gap-3">
          <span className={`text-[13px] ${account?.tokenSet ? "text-ok" : "text-tx4"}`}>
            {account?.tokenSet ? "token set" : "not connected"}
          </span>
          <Button variant="small" onClick={() => onEdit(!editing)}>
            {editing ? "Cancel" : account?.tokenSet ? "Change" : "Connect"}
          </Button>
        </span>
      </div>
      {editing && (
        <form
          className="flex flex-col gap-2.5 rounded-[10px] border border-line2 p-3.5"
          onSubmit={(e) => {
            e.preventDefault();
            connect.mutate();
          }}
        >
          <label className="flex flex-col gap-1 text-[13px] text-tx3">
            Host and owner
            <input
              value={host}
              onChange={(e) => setHost(e.target.value)}
              className="rounded-[7px] border border-line2 bg-transparent px-2.5 py-1.5 font-mono text-[13px] text-tx"
            />
          </label>
          <label className="flex flex-col gap-1 text-[13px] text-tx3">
            Token (stored on this machine, never shown again)
            <input
              type="password"
              autoComplete="off"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              className="rounded-[7px] border border-line2 bg-transparent px-2.5 py-1.5 font-mono text-[13px] text-tx"
            />
          </label>
          {connect.error && <p className="m-0 text-[13px] text-err">{connect.error.message}</p>}
          <div className="flex justify-end">
            <Button
              type="submit"
              variant="primary"
              disabled={connect.isPending || token.trim() === "" || host.trim() === ""}
            >
              Save
            </Button>
          </div>
        </form>
      )}
    </div>
  );
}

/** Where the global repo is hosted, so Sync can push its main there. */
function GlobalRepoSection({ settings }: { settings: SettingsData }) {
  const client = useClient();
  const save = useSave();
  const accounts = useQuery({ queryKey: keys.forge, queryFn: () => client.forge.accounts() });
  const connected = (accounts.data ?? []).filter((a) => a.tokenSet).map((a) => a.kind);
  const current = settings.globalForge;
  // A forge needs its repo URL before it can be saved (settings validation).
  const [kind, setKind] = useState(current.kind);
  const [url, setUrl] = useState(current.url);
  const kinds = ["none", ...connected];
  if (current.kind !== "none" && !kinds.includes(current.kind)) kinds.push(current.kind);
  const choose = (k: string) => {
    setKind(k);
    if (k === "none") save((s) => ({ ...s, globalForge: { kind: "none", url: "" } }));
  };
  const commitUrl = () => {
    const next = url.trim();
    if (kind === "none" || next === "" || (kind === current.kind && next === current.url)) return;
    save((s) => ({ ...s, globalForge: { kind, url: next } }));
  };
  return (
    <Section
      label="Global repo"
      kept={inGlobalRepo}
      intro="Where your team definitions and settings are hosted, so Sync can push them there. Without
          a forge they stay on this machine. Nothing is pushed automatically."
    >
      <div className="flex flex-col gap-2.5">
        <Segmented
          label="Global repo forge"
          value={kind}
          onChange={choose}
          options={kinds.map((k) => ({
            value: k,
            label: k === "none" ? "No forge" : forgeLabel(k),
          }))}
        />
        {kind !== "none" && (
          <span className="flex items-center gap-2">
            <input
              aria-label="Global repo URL"
              placeholder="https://…/foreman-config"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              onBlur={commitUrl}
              onKeyDown={(e) => e.key === "Enter" && e.currentTarget.blur()}
              className="flex-1 rounded-[7px] border border-line2 bg-transparent px-2.5 py-1.5 font-mono text-[13px] text-tx"
            />
            <Button variant="small" disabled title="Comes with the Git epic">
              Sync
            </Button>
          </span>
        )}
      </div>
    </Section>
  );
}
