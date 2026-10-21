import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useSearch } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";
import {
  type ForgeAccount,
  type ImageStatus,
  isNotBuilt,
  type Preferences,
  type Settings as SettingsData,
  useClient,
} from "../api/client";
import { harnessLabel, profileName } from "../app/model";
import { permission } from "../app/notifications";
import {
  keys,
  useHarnesses,
  useHomeFolder,
  useMaintenance,
  usePreferences,
  useProfiles,
  useSaveSettings,
  useSaveTerminalFont,
  useSaveTheme,
  useSettings,
} from "../app/queries";
import {
  logCapChoices,
  settingsSearch,
  settingsTabs,
  stuckChoices,
  withCurrent,
} from "../app/settings";
import { asTheme, savedTheme } from "../app/theme";
import { ConnectDialog } from "../components/ConnectDialog";
import { Confirm } from "../components/Overlays";
import { Select } from "../components/Select";
import { useToast } from "../components/Toasts";
import { terminalFont, terminalFontSizes, terminalFonts } from "../components/terminalFonts";
import { Button, NotBuilt, PageTitle, Segmented, Switch } from "../components/ui";

// Settings (wiki/product/features/settings.md), in tabs by topic. Each
// section says where it is kept: the global repo's settings.toml, the same on
// every machine you clone it to, or this machine only. Every change saves at once.

const inGlobalRepo = "Global repo";
const onThisMachine = "This machine";

export function Settings() {
  const tab = useSearch({ from: "/settings" }).tab ?? "agents";
  const file = useSettings();
  const s = file.data?.settings;
  return (
    <div className="mx-auto flex w-full max-w-[760px] flex-col gap-9 px-12 pt-12 pb-16">
      <div className="flex flex-col gap-4">
        <PageTitle title="Settings" />
        <nav className="flex flex-wrap border-b border-line">
          {settingsTabs.map((t) => (
            <Link
              key={t.tab}
              to="/settings"
              search={settingsSearch({ tab: t.tab })}
              className={`-mb-px border-b-2 px-3.5 py-[9px] font-medium ${
                t.tab === tab ? "border-acc text-tx" : "border-transparent text-tx3 hover:text-tx2"
              }`}
            >
              {t.label}
            </Link>
          ))}
        </nav>
      </div>
      {file.data && file.data.problems.length > 0 && (
        <p className="m-0 rounded-lg border border-err px-3 py-2 text-[13px] text-err">
          settings.toml: {file.data.problems.join(" · ")}
        </p>
      )}
      {tab === "agents" && (
        <>
          <ProfilesSection />
          {s && <RunningSection settings={s} />}
        </>
      )}
      {tab === "git" && s && (
        <>
          <ForgeSection settings={s} />
          <GlobalRepoSection settings={s} />
        </>
      )}
      {tab === "look" && (
        <>
          {s && <NotificationsSection settings={s} />}
          <AppearanceSection />
        </>
      )}
      {tab === "system" && (
        <>
          <HomeSection />
          {s && <NetworkSection settings={s} />}
          <MaintenanceSection />
        </>
      )}
    </div>
  );
}

/** Saves a change to the settings read last; a failure says why and the screen refetches. */
function useSave() {
  const save = useSaveSettings();
  const toast = useToast();
  return (change: (s: SettingsData) => SettingsData) =>
    save(change).catch((e: Error) =>
      toast({ title: "Settings not saved", body: e.message, color: "var(--err)" }),
    );
}

function Section({
  label,
  kept,
  intro,
  children,
}: {
  label: string;
  /** Where what it sets is kept; none for what isn't a setting. */
  kept?: string;
  intro?: string;
  children: ReactNode;
}) {
  return (
    <section className="flex flex-col">
      <h2 className="m-0 mb-1 text-[13px] font-medium text-tx4">
        {label}
        {kept && <span className="font-normal"> · {kept}</span>}
      </h2>
      {intro && <p className="m-0 mb-2 text-[13.5px] text-tx3">{intro}</p>}
      {children}
    </section>
  );
}

function Row({
  title,
  explanation,
  children,
}: {
  title: string;
  explanation?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="grid grid-cols-[1fr_auto] items-center gap-6 border-b border-line py-3.5">
      <div className="flex flex-col gap-0.5">
        <span className="font-medium">{title}</span>
        {explanation && <span className="text-[13.5px] leading-[1.5] text-tx3">{explanation}</span>}
      </div>
      <div className="flex items-center gap-2">{children}</div>
    </div>
  );
}

function HomeSection() {
  const home = useHomeFolder();
  return (
    <Section label="Home">
      <Row
        title="Home"
        explanation="Foreman’s folder on this machine: the global repo (teams, settings), each project’s repo, the agents’ sandboxes and its state."
      >
        <span className="font-mono text-[13px] text-tx2">{home.data?.path}</span>
      </Row>
    </Section>
  );
}

function NetworkSection({ settings }: { settings: SettingsData }) {
  const save = useSave();
  const [confirm, setConfirm] = useState(false);
  return (
    <Section label="Network" kept={inGlobalRepo}>
      <Row
        title="Listen on"
        explanation="Anyone with the URL Foreman printed at start has full control. Keep this on your own machine unless you trust the network. Takes effect on restart."
      >
        <Segmented
          label="Listen on"
          value={settings.listen}
          onChange={(v) => (v === "lan" ? setConfirm(true) : save((s) => ({ ...s, listen: v })))}
          options={[
            { value: "local", label: "This machine" },
            { value: "lan", label: "Local network" },
          ]}
        />
      </Row>
      <Confirm
        open={confirm}
        onClose={() => setConfirm(false)}
        title="Listen on the local network?"
        body="Anyone on your network who gets the URL can drive your agents, and through them your code and your machine. Only do this on a network you trust. Takes effect when Foreman restarts."
        action="Listen on the network"
        onConfirm={() => save((s) => ({ ...s, listen: "lan" }))}
      />
    </Section>
  );
}

function RunningSection({ settings }: { settings: SettingsData }) {
  const save = useSave();
  const engine = useMaintenance();
  const [confirm, setConfirm] = useState(false);
  return (
    <Section label="Running agents" kept={inGlobalRepo}>
      <Row
        title="Running sessions at once"
        explanation="Extra sessions wait in a queue until a slot frees up."
      >
        <Stepper
          value={settings.maxSessions}
          min={1}
          max={64}
          label="Running sessions at once"
          onChange={(n) => save((s) => ({ ...s, maxSessions: n }))}
        />
      </Row>
      <Row
        title="Stuck after"
        explanation="An agent mid-turn that sends no sign of activity (a prompt, a tool call, a turn ending) for this long gets a card on the Dashboard. It keeps running; you decide."
      >
        <Select
          label="Stuck after"
          value={settings.stuckAfterMinutes}
          options={withCurrent(stuckChoices, settings.stuckAfterMinutes).map((m) => ({
            value: m,
            label: `${m} min`,
          }))}
          onChange={(m) => save((s) => ({ ...s, stuckAfterMinutes: m }))}
        />
      </Row>
      <Row
        title="Session logs per agent"
        explanation="Every byte each agent's terminal printed, kept in Foreman's home. Past this size, the oldest go first."
      >
        <Select
          label="Session logs per agent"
          value={settings.logCapMb}
          options={withCurrent(logCapChoices, settings.logCapMb).map((mb) => ({
            value: mb,
            label: `${mb} MB`,
          }))}
          onChange={(mb) => save((s) => ({ ...s, logCapMb: mb }))}
        />
      </Row>
      <Row
        title="Run agents in"
        explanation={
          <>
            Containers isolate each agent from your machine. Without them, agents run as plain
            processes with full access to it. Takes effect when Foreman restarts.
            {engine.data?.podman && (
              <span className="mt-1.5 block text-tx2">
                Containers aren't available: {engine.data.podman}
              </span>
            )}
            {engine.data && settings.engine !== engine.data.running && (
              <span className="mt-1.5 block font-medium text-accTx">
                Restart Foreman to switch: agents run{" "}
                {engine.data.running === "direct" ? "directly" : "in containers"} until then.
              </span>
            )}
            {engine.data?.running === "direct" && (
              <span className="mt-1.5 block font-medium text-err">
                Agents run with no isolation: they can read and change anything your user can.
              </span>
            )}
          </>
        }
      >
        <Segmented
          label="Run agents in"
          value={settings.engine}
          onChange={(v) => (v === "direct" ? setConfirm(true) : save((s) => ({ ...s, engine: v })))}
          options={[
            // Containers can't be chosen without Podman: Foreman wouldn't start.
            {
              value: "podman",
              label: "Containers (Podman)",
              disabled: !!engine.data?.podman && settings.engine !== "podman",
            },
            { value: "direct", label: "Directly (no isolation)" },
          ]}
        />
      </Row>
      <Confirm
        open={confirm}
        onClose={() => setConfirm(false)}
        title="Run agents with no isolation?"
        body="Agents run as plain processes on this machine, with permission checks off: they can read and change anything your user can. Use this for development only."
        action="Run directly"
        onConfirm={() => save((s) => ({ ...s, engine: "direct" }))}
      />
    </Section>
  );
}

function Stepper({
  value,
  min,
  max,
  label,
  onChange,
}: {
  value: number;
  min: number;
  max: number;
  label: string;
  onChange: (n: number) => void;
}) {
  return (
    <fieldset className="m-0 flex items-center gap-1 border-0 p-0">
      <legend className="sr-only">{label}</legend>
      <Button
        className="!size-7 !p-0"
        aria-label="Fewer"
        disabled={value <= min}
        onClick={() => onChange(value - 1)}
      >
        −
      </Button>
      <span className="min-w-7 text-center font-mono text-sm font-medium" aria-live="polite">
        {value}
      </span>
      <Button
        className="!size-7 !p-0"
        aria-label="More"
        disabled={value >= max}
        onClick={() => onChange(value + 1)}
      >
        +
      </Button>
    </fieldset>
  );
}

/** The CEO's profiles: each a harness and its login, on this machine. A
 * project picks one in its settings. */
function ProfilesSection() {
  const client = useClient();
  const queries = useQueryClient();
  const toast = useToast();
  const profiles = useProfiles();
  const harnesses = useHarnesses();
  const [connecting, setConnecting] = useState<string>();
  const [adding, setAdding] = useState(false);
  const [deleting, setDeleting] = useState<string>();
  const refetch = () => {
    queries.invalidateQueries({ queryKey: keys.profiles });
    queries.invalidateQueries({ queryKey: keys.setup });
  };
  const remove = useMutation({
    mutationFn: (name: string) => client.profiles.remove(name),
    onSettled: refetch,
    onError: (e: Error) =>
      toast({ title: "Profile not deleted", body: e.message, color: "var(--err)" }),
  });
  return (
    <Section
      label="Profiles"
      kept={onThisMachine}
      intro="Each profile is a harness and its credentials, yours on this machine: Claude Code's login, or the model server Pi runs on. Every project can use it; a project uses default unless its settings pick another."
    >
      {profiles.isError && <p className="m-0 text-[13px] text-err">{profiles.error.message}</p>}
      {(profiles.data ?? []).map((p) => (
        <div
          key={p.name}
          className="grid grid-cols-[1fr_auto_auto] items-center gap-4 border-b border-line py-3"
        >
          <span className="flex min-w-0 items-baseline gap-2">
            <span className="truncate font-mono font-semibold">{p.name}</span>
            <span className="text-[13px] text-tx4">{harnessLabel(p.harness)}</span>
          </span>
          <span className={`text-[13px] ${p.connected ? "text-ok" : "text-tx4"}`}>
            {p.connected ? "connected" : "not connected"}
          </span>
          <span className="flex items-center gap-2">
            <Button variant="small" onClick={() => setConnecting(p.name)}>
              {p.connected ? "Reconnect" : "Connect"}
            </Button>
            {p.name !== "default" && (
              <Button variant="small" onClick={() => setDeleting(p.name)}>
                Delete
              </Button>
            )}
          </span>
        </div>
      ))}
      {adding ? (
        <NewProfile
          harnesses={(harnesses.data ?? []).map((h) => h.id)}
          onCancel={() => setAdding(false)}
          onCreated={(name) => {
            setAdding(false);
            refetch();
            setConnecting(name); // its login next
          }}
        />
      ) : (
        <div className="pt-3">
          <Button variant="small" onClick={() => setAdding(true)}>
            New profile
          </Button>
        </div>
      )}
      {connecting && (
        <ConnectDialog profile={connecting} onClose={() => setConnecting(undefined)} />
      )}
      <Confirm
        open={deleting !== undefined}
        title={`Delete the ${deleting} profile?`}
        body="Its credentials go with it. Projects that use it can't start agents until they pick another profile."
        action="Delete"
        onConfirm={() => deleting && remove.mutate(deleting)}
        onClose={() => setDeleting(undefined)}
      />
    </Section>
  );
}

/** A new profile: its name and its harness; its credentials come after (Connect). */
function NewProfile({
  harnesses,
  onCancel,
  onCreated,
}: {
  harnesses: string[];
  onCancel: () => void;
  onCreated: (name: string) => void;
}) {
  const client = useClient();
  const [typed, setTyped] = useState("");
  const [harness, setHarness] = useState(harnesses[0] ?? "claude-code");
  const name = profileName(typed);
  const create = useMutation({
    mutationFn: () => client.profiles.create(name, harness),
    onSuccess: (p) => onCreated(p.name),
  });
  return (
    <form
      className="flex flex-wrap items-center gap-2 border-b border-line py-3 text-[13px]"
      onSubmit={(e) => {
        e.preventDefault();
        if (name) create.mutate();
      }}
    >
      <label htmlFor="new-profile" className="text-tx3">
        Name
      </label>
      <input
        id="new-profile"
        // biome-ignore lint/a11y/noAutofocus: opened on purpose, to type its name
        autoFocus
        value={typed}
        onChange={(e) => setTyped(e.target.value)}
        placeholder="work"
        className="w-36 rounded-md border border-line2 bg-panel px-2 py-1 font-mono text-[13px]"
      />
      {typed && name !== typed && <span className="font-mono text-tx4">{name || "—"}</span>}
      <span className="ml-2 text-tx3">Harness</span>
      <Select
        label="Harness"
        value={harness}
        onChange={setHarness}
        options={harnesses.map((h) => ({ value: h, label: harnessLabel(h) }))}
      />
      <span className="flex-1" />
      <Button variant="small" onClick={onCancel}>
        Cancel
      </Button>
      <Button variant="small" type="submit" disabled={!name || create.isPending}>
        Create
      </Button>
      {create.error && <p className="m-0 w-full text-err">{create.error.message}</p>}
    </form>
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
    <div className="flex flex-col gap-3 border-b border-line py-3">
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
      <div className="flex flex-col gap-2.5 py-1.5">
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

const permissionColor = {
  on: "text-ok",
  blocked: "text-err",
  "turn on": "text-accTx",
  "n/a": "text-tx4",
};

function NotificationsSection({ settings }: { settings: SettingsData }) {
  const save = useSave();
  const toast = useToast();
  const [state, setState] = useState(permission);
  const toggles: { key: keyof SettingsData["notifications"]; label: string }[] = [
    { key: "needsMe", label: "An agent needs me" },
    { key: "finishes", label: "An agent finishes" },
    { key: "mergeRequested", label: "A merge is requested" },
  ];
  return (
    <Section label="Notifications" kept={inGlobalRepo}>
      <Row
        title="Browser notifications"
        explanation="Shown while Foreman isn’t the tab you’re looking at. In-app toasts always show."
      >
        <span className={`text-[13px] ${permissionColor[state]}`}>{state}</span>
        {state === "turn on" && (
          <Button
            variant="small"
            onClick={async () => {
              await Notification.requestPermission();
              setState(permission());
              if (permission() === "on")
                toast({
                  title: "Notifications on",
                  body: "You'll hear from agents when they finish or need you.",
                  color: "var(--ok)",
                });
            }}
          >
            Request
          </Button>
        )}
      </Row>
      {toggles.map((t) => (
        <Row key={t.key} title={t.label}>
          <Switch
            label={t.label}
            checked={settings.notifications[t.key]}
            onChange={(on) =>
              save((s) => ({ ...s, notifications: { ...s.notifications, [t.key]: on } }))
            }
          />
        </Row>
      ))}
    </Section>
  );
}

const imageState: Record<string, { label: string; color: string }> = {
  ready: { label: "ready", color: "text-ok" },
  building: { label: "building…", color: "text-team" },
  failed: { label: "failed", color: "text-err" },
  missing: { label: "not built yet", color: "text-tx4" },
};

function MaintenanceSection() {
  const client = useClient();
  const queries = useQueryClient();
  const status = useMaintenance();
  const rebuild = useMutation({
    mutationFn: () => client.maintenance.rebuild(),
    onSettled: () => queries.invalidateQueries({ queryKey: keys.maintenance }),
  });
  const building = status.data?.images.some((i) => i.state === "building") ?? false;
  return (
    <Section
      label="Maintenance"
      kept={onThisMachine}
      intro="The image agent sessions run in. Rebuild picks up the latest harness versions; running sessions keep their image, new and restarted ones use the new one."
    >
      <Row title="Engine">
        <span
          className={`text-[13px] ${status.data?.engine.includes("not reachable") ? "text-err" : "text-ok"}`}
        >
          {status.data?.engine ?? "…"}
        </span>
      </Row>
      {status.data?.images.length === 0 && (
        <p className="m-0 border-b border-line py-3.5 text-[13.5px] text-tx3">
          Agents run directly on this machine: no images to build.
        </p>
      )}
      {status.data?.images.map((image) => (
        <ImageRow
          key={image.name}
          image={image}
          onRebuild={() => rebuild.mutate()}
          busy={building || rebuild.isPending}
        />
      ))}
      {rebuild.error && <p className="m-0 pt-2 text-[13px] text-err">{rebuild.error.message}</p>}
    </Section>
  );
}

function ImageRow({
  image,
  onRebuild,
  busy,
}: {
  image: ImageStatus;
  onRebuild: () => void;
  busy: boolean;
}) {
  const [log, setLog] = useState(false);
  const state = imageState[image.state] ?? { label: image.state, color: "text-tx3" };
  return (
    <div className="flex flex-col gap-2.5 border-b border-line py-3.5">
      <div className="grid grid-cols-[1fr_auto] items-center gap-4">
        <span className="flex flex-col gap-0.5">
          <span className="font-mono text-[13px]">{image.name}</span>
          <span className={`text-[13px] ${state.color}`}>{state.label}</span>
        </span>
        <span className="flex gap-2">
          <Button variant="small" disabled={image.log === ""} onClick={() => setLog(!log)}>
            {log ? "Hide log" : "Log"}
          </Button>
          <Button variant="small" disabled={busy} onClick={onRebuild}>
            Rebuild
          </Button>
        </span>
      </div>
      {log && (
        <pre className="m-0 max-h-80 overflow-auto rounded-[10px] border border-line bg-term p-3.5 font-mono text-[12px] leading-[1.6] whitespace-pre-wrap text-tx2 select-text">
          {image.log}
        </pre>
      )}
    </div>
  );
}

/** The theme, one of the CEO's preferences. */
function AppearanceSection() {
  const prefs = usePreferences();
  const save = useSaveTheme();
  const toast = useToast();
  const theme =
    save.isPending && save.variables
      ? save.variables
      : prefs.data
        ? asTheme(prefs.data.theme)
        : savedTheme();
  return (
    <Section label="Appearance" kept={onThisMachine}>
      <Row title="Theme" explanation="System follows your operating system.">
        <Segmented
          label="Theme"
          value={theme}
          onChange={(t) =>
            save.mutate(t, {
              onError: (e) =>
                toast({ title: "Theme not saved", body: e.message, color: "var(--err)" }),
            })
          }
          options={[
            { value: "system", label: "System" },
            { value: "dark", label: "Dark" },
            { value: "light", label: "Light" },
          ]}
        />
      </Row>
      {prefs.data && <TerminalFontRow prefs={prefs.data} />}
    </Section>
  );
}

function TerminalFontRow({ prefs }: { prefs: Preferences }) {
  const save = useSaveTerminalFont();
  const toast = useToast();
  const change = (p: Partial<Preferences>) =>
    save.mutate(p, {
      onError: (e) => toast({ title: "Font not saved", body: e.message, color: "var(--err)" }),
    });
  return (
    <Row
      title="Terminal font"
      explanation="The agents’ terminals. System uses your operating system’s monospace font."
    >
      <span className="flex gap-2">
        <Select
          label="Terminal font"
          value={terminalFont(prefs.terminalFont).id}
          options={terminalFonts.map((f) => ({ value: f.id, label: f.label }))}
          onChange={(terminalFont) => change({ terminalFont })}
        />
        <Select
          label="Terminal font size"
          value={prefs.terminalFontSize}
          options={terminalFontSizes.map((s) => ({ value: s, label: `${s} px` }))}
          onChange={(terminalFontSize) => change({ terminalFontSize })}
        />
      </span>
    </Row>
  );
}
