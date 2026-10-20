import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, useSearch } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";
import { type Instance, type Team, useClient } from "../api/client";
import { agentOf, harnessLabel, slotsTaken } from "../app/model";
import {
  keys,
  useDeployments,
  useDraft,
  useHarnesses,
  useInstances,
  useRequests,
  useSettings,
  useTeams,
} from "../app/queries";
import {
  applyMention,
  clickTab,
  closePane,
  countdown,
  deploymentTabs,
  inOrder,
  isLineShortcut,
  linePlaceholder,
  lineShortcut,
  mentionAt,
  mentionMatches,
  moved,
  type Panes,
  panesFrom,
  rosterOrder,
  type SessionsSearch,
  searchFor,
  toggleSplit,
} from "../app/sessions";
import { ActivityPanel } from "../components/ActivityPanel";
import { Avatar } from "../components/Avatar";
import { DeployDialog } from "../components/DeployDialog";
import { CheckIcon, CopyIcon, PromptIcon } from "../components/icons";
import { StopDialog } from "../components/StopDialog";
import { Terminal, type TerminalHandle } from "../components/Terminal";
import { useToast } from "../components/Toasts";
import { Button, Dot, LeadTag, outlineLink, Segmented } from "../components/ui";

// The terminal multiplexer (wiki/product/features/sessions.md): deployment
// tabs, agent tabs, one or two panes of real terminals, and the Activity
// panel.

// Each deployment's last view, so switching deployment tabs comes back to it
// (while the app runs; across browsers, with the UI state API).
const lastView = new Map<string, SessionsSearch>();
// The deployment tabs' order as dragged, while the app runs (the CEO, 2026-10-17).
let tabOrder: string[] = [];

export function Sessions() {
  const search = useSearch({ from: "/sessions" });
  const navigate = useNavigate();
  const instances = useInstances();
  const teams = useTeams();
  const requests = useRequests();
  const [order, setOrder] = useState(tabOrder);
  // The agent whose tab was clicked last, and how many times: its pane takes
  // the keyboard once it shows it (the URL changes after the click, so only
  // the pane showing that agent may take it, never the one it replaces).
  const [focusAsk, setFocusAsk] = useState({ id: "", n: 0 });
  const tabs = inOrder(
    deploymentTabs(instances.data ?? [], teams.data ?? [], requests.data ?? []),
    order,
  );
  // A deployment that is gone (stopped, mistyped) falls back to the first one.
  const asked = tabs.some((t) => t.id === search.deployment) ? search.deployment : undefined;
  const deployment = asked ?? tabs[0]?.id ?? "";
  const roster = rosterOrder(deployment, instances.data ?? [], teams.data ?? []);
  // The URL is the view: a reload, a link or Back shows the same panes.
  const panes = panesFrom(search, roster);
  const activity = search.view === "activity";
  const [stopping, setStopping] = useState<string>(); // a deployment id
  const deployments = useDeployments();
  const toStop = deployments.data?.find((d) => d.id === stopping);
  const show = (p: Panes, withActivity: boolean) => {
    const next = searchFor(deployment, p, withActivity);
    lastView.set(deployment, next);
    navigate({ to: "/sessions", search: next, replace: true });
  };
  const setPanes = (p: Panes) => show(p, activity);
  const setActivity = (on: boolean) => show(panes, on);

  if (instances.isPending) return <Centered>…</Centered>;
  if (tabs.length === 0)
    return (
      <Centered>
        <NoSessions />
      </Centered>
    );

  const team = teams.data?.find((t) => t.id === deployment.split("@")[0]);
  const project = deployment.split("@")[1] ?? "";
  const byId = new Map(roster.map((i) => [i.id, i]));
  const shown = panes.open.filter((id) => byId.has(id));
  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex items-center gap-3 border-b border-line px-4 py-2.5">
        <Segmented
          label="Deployments"
          value={deployment}
          onChange={(d) =>
            navigate({ to: "/sessions", search: lastView.get(d) ?? { deployment: d } })
          }
          onClose={{ label: "Stop", close: setStopping }}
          onReorder={(dragged, target) => {
            tabOrder = moved(
              tabs.map((t) => t.id),
              dragged,
              target,
            );
            setOrder(tabOrder);
          }}
          options={tabs.map((t) => ({
            value: t.id,
            label: (
              <span className="flex items-center gap-1.5">
                <Dot status={t.status} title={t.status} />
                {t.label}
                {t.needsYou && <Dot title="Needs you" />}
              </span>
            ),
          }))}
        />
        <span className="flex-1" />
        {/* Back to what the team works on: the project's tasks and their status. */}
        <Link
          to="/projects/$project"
          params={{ project }}
          search={{ tab: "board" }}
          className={outlineLink}
        >
          {project} · Board
        </Link>
        {/* One view at a time: one terminal, two, or one beside the activity. */}
        <span className="rounded-lg border border-line p-0.5">
          <Segmented
            label="View"
            value={activity ? "activity" : panes.open.length === 2 ? "split" : "single"}
            options={[
              { value: "single", label: "Single" },
              { value: "split", label: "Split" },
              { value: "activity", label: "Activity" },
            ]}
            onChange={(view) => {
              const split = panes.open.length === 2;
              show(
                (view === "split") !== split ? toggleSplit(panes, roster) : panes,
                view === "activity",
              );
            }}
          />
        </span>
        {toStop && (
          <StopDialog
            deployment={toStop}
            teamName={teams.data?.find((t) => t.id === toStop.teamId)?.config.name ?? toStop.teamId}
            onClose={() => setStopping(undefined)}
          />
        )}
      </div>
      <AgentTabs
        roster={roster}
        team={team}
        panes={panes}
        onClick={(id) => {
          setPanes(clickTab(panes, id));
          setFocusAsk((f) => ({ id, n: f.n + 1 }));
        }}
      />
      <div className="flex min-h-0 flex-1">
        <div
          className={`grid min-h-0 min-w-0 flex-1 ${shown.length === 2 ? "grid-cols-2 divide-x divide-line" : "grid-cols-1"}`}
        >
          {shown.map((id, index) => (
            <Pane
              key={id}
              instance={byId.get(id) as Instance}
              teams={teams.data ?? []}
              lead={team?.config.lead}
              active={index === panes.active}
              focusAsk={focusAsk}
              split={shown.length === 2}
              onActivate={() => setPanes({ ...panes, active: index })}
              onClose={() => setPanes(closePane(panes, index))}
            />
          ))}
        </div>
        {activity && (
          <ActivityPanel
            deployment={deployment}
            roster={roster}
            teams={teams.data ?? []}
            onClose={() => setActivity(false)}
            onOpen={(id) => setPanes(clickTab(panes, id))}
          />
        )}
      </div>
    </div>
  );
}

function AgentTabs({
  roster,
  team,
  panes,
  onClick,
}: {
  roster: Instance[];
  team?: Team;
  panes: Panes;
  onClick: (id: string) => void;
}) {
  const tab = (i: Instance) => {
    const soul = team?.config.agents[i.agentId];
    const at = panes.open.indexOf(i.id);
    const underline =
      at < 0
        ? ""
        : at === panes.active
          ? "shadow-[inset_0_-2px_0_var(--acc)]"
          : "shadow-[inset_0_-2px_0_var(--line3)]";
    return (
      <button
        key={i.id}
        type="button"
        onClick={() => onClick(i.id)}
        className={`flex shrink-0 items-center gap-2 px-3 py-2.5 font-medium ${underline} ${at >= 0 ? "text-tx" : "text-tx3 hover:text-tx2"}`}
      >
        <Avatar look={soul?.look} size={24} status={i.status} dotBorder="var(--side)" />
        {soul?.name ?? i.agentId}
        {i.agentId === team?.config.lead && <LeadTag />}
      </button>
    );
  };
  const [lead, ...members] = roster;
  return (
    <nav className="flex items-center overflow-x-auto border-b border-line bg-side px-3 [scrollbar-width:thin]">
      {lead && tab(lead)}
      {members.length > 0 && (
        <>
          <span className="mx-2 h-5 w-px shrink-0 bg-line2" />
          <span className="mr-1 shrink-0 text-xs text-tx4">Direct</span>
          {members.map(tab)}
        </>
      )}
    </nav>
  );
}

function Pane({
  instance,
  teams,
  lead,
  active,
  focusAsk,
  split,
  onActivate,
  onClose,
}: {
  instance: Instance;
  teams: Team[];
  lead?: string;
  active: boolean;
  /** The last agent-tab click: the pane showing that agent takes the keyboard. */
  focusAsk: { id: string; n: number };
  split: boolean;
  onActivate: () => void;
  onClose: () => void;
}) {
  const terminal = useRef<TerminalHandle>(null);
  const client = useClient();
  const draft = useDraft(instance.id);
  const newline = useHarnesses().data?.find((h) => h.id === instance.harness)?.newline;
  const [copied, setCopied] = useState(false);
  const [lineMode, setLineMode] = useState(false);
  const lineInput = useRef<HTMLTextAreaElement>(null);
  // Line opens with the keyboard in it (autoFocus), and closes back to the terminal.
  const closeLine = () => {
    setLineMode(false);
    terminal.current?.focus();
  };
  // The shortcut: open Line; from the terminal, back into Line; from Line, close it.
  // Asleep, stopped or queued: no session runs; the pane says why, and what to do.
  const asleep = ["asleep", "stopped", "queued"].includes(instance.status);
  const toggleLine = () => {
    if (asleep) return;
    if (!lineMode) setLineMode(true);
    else if (document.activeElement !== lineInput.current) lineInput.current?.focus();
    else closeLine();
  };
  // A tab click (or a pane it opened) takes the keyboard: Line when open, else
  // the terminal; an agent with no session has nothing to type into.
  // biome-ignore lint/correctness/useExhaustiveDependencies: on a click only (and on mounting, for a pane the click opened); opening or closing Line moves the keyboard itself
  useEffect(() => {
    if (focusAsk.id !== instance.id) return;
    if (lineMode) lineInput.current?.focus();
    else terminal.current?.focus();
  }, [focusAsk]);
  // The shortcut works anywhere on the screen, for the active pane. Listening on
  // the window's capture phase runs before the terminal, so the agent never gets it.
  const toggle = useRef(toggleLine);
  toggle.current = toggleLine;
  useEffect(() => {
    if (!active) return;
    const onKey = (e: KeyboardEvent) => {
      if (!isLineShortcut(e)) return;
      e.preventDefault();
      e.stopPropagation();
      toggle.current();
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [active]);
  const [line, setLine] = useState("");
  // The line goes through Foreman: typed, then sent after a pause, like a
  // delivery (never mixed with one), so the harness takes the Enter as sending.
  const sendLine = useMutation({
    mutationFn: (text: string) => client.agents.line(instance.id, text),
    onSuccess: () => setLine(""),
  });
  const agent = agentOf(teams, instance.id);
  // @ in the line offers the teammates' names (the agent's own left out).
  const teammates = Object.entries(agent?.team.config.agents ?? {})
    .filter(([id]) => id !== instance.agentId)
    .map(([, soul]) => soul.name);
  const [mention, setMention] = useState<{ start: number; query: string; index: number } | null>(
    null,
  );
  const options = mention ? mentionMatches(teammates, mention.query) : [];
  const pick = (choice: string) => {
    const el = lineInput.current;
    if (!mention || !el) return;
    const next = applyMention(line, el.selectionStart, mention.start, choice);
    setLine(next.text);
    setMention(null);
    requestAnimationFrame(() => el.setSelectionRange(next.caret, next.caret));
  };
  const name = agent?.soul.name ?? instance.agentId;
  const isLead = instance.agentId === lead;
  const leadName = lead ? (agentOf(teams, `${lead}@`)?.soul.name ?? lead) : "the lead";
  const project = instance.id.split("@")[1];
  const copy = async () => {
    await navigator.clipboard.writeText(terminal.current?.text() ?? "");
    setCopied(true);
    setTimeout(() => setCopied(false), 1400);
  };
  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: a click anywhere in a pane activates it (the spec); the keyboard does it through focus
    <section
      onMouseDown={onActivate}
      onFocusCapture={onActivate}
      className={`flex min-h-0 min-w-0 flex-col bg-term ${split && active ? "shadow-[inset_0_2px_0_var(--acc)]" : ""}`}
    >
      <header className="flex items-center gap-2.5 border-b border-line px-4 py-[9px] text-[13px]">
        {/* Single: the active tab already names the agent. Split: each pane says whose it is. */}
        {split && (
          <>
            <Avatar
              look={agent?.soul.look}
              size={26}
              status={instance.status}
              dotBorder="var(--term)"
            />
            {agent ? (
              <Link
                to="/teams/$team/agents/$agent"
                params={{ team: agent.team.id, agent: agent.id }}
                className="font-semibold text-tx"
              >
                {name}
              </Link>
            ) : (
              <span className="font-semibold">{name}</span>
            )}
          </>
        )}
        <span className="text-tx4">
          {harnessLabel(instance.harness)} · {project}
        </span>
        <span className="flex-1" />
        {!asleep && (
          <IconButton onClick={copy} label={copied ? "Copied" : "Copy the whole buffer"}>
            {copied ? <CheckIcon /> : <CopyIcon />}
          </IconButton>
        )}
        {!asleep && (
          <IconButton
            onClick={() => (lineMode ? closeLine() : setLineMode(true))}
            pressed={lineMode}
            label={`Line input (${lineShortcut()})`}
          >
            <PromptIcon />
          </IconButton>
        )}
        {/* Always last, where a close is expected. */}
        {split && (
          <IconButton onClick={onClose} label={`Close ${name}'s pane`}>
            <span className="text-[16px] leading-4">×</span>
          </IconButton>
        )}
      </header>
      {!isLead && (
        <div className="border-b border-line bg-side px-4 py-[7px] text-[12.5px] text-tx3">
          Direct line to {name}. They'll tell {leadName}, the lead, if it matters.
        </div>
      )}
      {instance.status === "queued" ? (
        <Queued name={name} />
      ) : asleep ? (
        <Asleep instance={instance.id} name={name} stopped={instance.status === "stopped"} />
      ) : (
        <div className="flex min-h-0 flex-1">
          <div className="min-h-0 min-w-0 flex-1 px-4 py-3">
            <Terminal instance={instance.id} newline={newline} ref={terminal} />
          </div>
        </div>
      )}
      {draft && !asleep && (
        // A box inset from the pane's edges, so a neighbour (the Activity panel, the
        // other pane) never hides a side of its border.
        <div className="mx-3 my-2 flex items-center gap-3 rounded-lg border-2 border-acc bg-accSoft px-3.5 py-2 text-[12.5px] text-accTx">
          <span className="flex-1">
            {draft.waiting === 1 ? "1 message" : `${draft.waiting} messages`} waiting for your line:
            send or clear it.{" "}
            <span className="font-mono text-tx">
              {draft.paused ? "Paused" : countdown(draft.leftMs)}
            </span>
            {!draft.paused && !draft.counting && " (waits while you type)"}
          </span>
          <Button
            variant="tiny"
            onClick={() => client.agents.pauseCountdown(instance.id, !draft.paused)}
          >
            {draft.paused ? "Resume" : "Pause"}
          </Button>
        </div>
      )}
      {lineMode && !asleep && (
        // A light box with room to breathe; its 1px border brightens on focus,
        // instead of the heavier focus ring.
        <div className="relative px-3 pt-1 pb-3">
          {options.length > 0 && mention && (
            <div
              role="listbox"
              aria-label="Teammates"
              className="absolute bottom-full left-6 z-10 mb-1 flex min-w-40 flex-col rounded-lg border border-line3 bg-panel p-1 shadow-overlay"
            >
              {options.map((o, i) => (
                <button
                  key={o}
                  type="button"
                  role="option"
                  aria-selected={i === mention.index}
                  // mousedown, not click: the textarea keeps the keyboard.
                  onMouseDown={(e) => {
                    e.preventDefault();
                    pick(o);
                  }}
                  className={`rounded-md px-2.5 py-1.5 text-left text-[13px] font-semibold ${
                    i === mention.index ? "bg-accSoft text-accTx" : "text-tx2 hover:bg-hover"
                  }`}
                >
                  {o}
                </button>
              ))}
            </div>
          )}
          <div className="flex items-start gap-2.5 rounded-lg border border-line2 bg-term px-4 py-3 focus-within:border-line3">
            <span className="font-mono text-[13px] leading-[1.6] font-medium text-accTx">❯</span>
            {/* Enter sends; Shift+Enter starts a new line, typed as the harness's newline. */}
            <textarea
              ref={lineInput}
              // biome-ignore lint/a11y/noAutofocus: opening Line is asking to type in it
              autoFocus
              value={line}
              rows={Math.min(line.split("\n").length, 8)}
              onChange={(e) => {
                setLine(e.target.value);
                const at = mentionAt(e.target.value, e.target.selectionStart);
                setMention(at && { ...at, index: 0 });
              }}
              onKeyDown={(e) => {
                // While names are offered, the arrows choose, Enter or Tab takes one,
                // Escape closes the list (not the line).
                if (options.length > 0 && mention) {
                  const step = e.key === "ArrowDown" ? 1 : e.key === "ArrowUp" ? -1 : 0;
                  if (step) {
                    e.preventDefault();
                    const index = (mention.index + step + options.length) % options.length;
                    setMention({ ...mention, index });
                    return;
                  }
                  if (e.key === "Enter" || e.key === "Tab") {
                    e.preventDefault();
                    return pick(options[mention.index]);
                  }
                  if (e.key === "Escape") {
                    e.preventDefault();
                    return setMention(null);
                  }
                }
                if (e.key === "Escape") return closeLine();
                if (e.key !== "Enter" || e.nativeEvent.isComposing) return;
                if (e.shiftKey) return; // the textarea adds the line
                e.preventDefault();
                // An empty line still sends Enter (it answers a harness's prompt).
                if (!sendLine.isPending) sendLine.mutate(line);
              }}
              placeholder={linePlaceholder(name, instance.status, isLead)}
              aria-autocomplete="list"
              aria-label={`Line input to ${name}`}
              className="flex-1 resize-none bg-transparent leading-[1.6] text-tx placeholder:text-tx4 focus-visible:outline-none"
            />
          </div>
          {sendLine.error && (
            <p className="m-0 mt-1.5 px-1 text-[12.5px] text-err">{sendLine.error.message}</p>
          )}
        </div>
      )}
    </section>
  );
}

/** An asleep or stopped agent's pane: no session runs until it is started,
 * and then it picks up its conversation. */
function Asleep({ instance, name, stopped }: { instance: string; name: string; stopped: boolean }) {
  const client = useClient();
  const queries = useQueryClient();
  const toast = useToast();
  const wake = useMutation({
    mutationFn: () => client.agents.wake(instance),
    onSuccess: () => queries.invalidateQueries({ queryKey: keys.instances }),
    onError: (e) => toast({ title: `${name} didn't start`, body: e.message, color: "var(--err)" }),
  });
  return (
    <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-3 px-6 text-center">
      <p className="m-0 max-w-[380px] text-[13.5px] text-tx3">
        {stopped
          ? `${name} stopped: it crashed twice, or couldn't start. Messages for them wait until you start them again; they pick up their conversation.`
          : `${name} is asleep: no session runs until there's work for them.`}
      </p>
      <Button disabled={wake.isPending} onClick={() => wake.mutate()}>
        {stopped ? `Start ${name} again` : `Start ${name}`}
      </Button>
    </div>
  );
}

/** A queued agent's pane: every session slot is taken, idle agents included,
 * so it starts when one frees; the limit is in settings.toml. */
function Queued({ name }: { name: string }) {
  const instances = useInstances().data ?? [];
  const max = useSettings().data?.settings.maxSessions ?? 0;
  return (
    <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-3 px-6 text-center">
      <p className="m-0 max-w-[420px] text-[13.5px] text-tx3">
        {name} is waiting for a session slot: {slotsTaken(instances)} of {max} are taken, idle
        agents included. They start as soon as one frees, and get their messages then.
      </p>
      <Link to="/settings" className="text-[13.5px] text-accTx">
        Raise the limit in Settings
      </Link>
    </div>
  );
}

/** Nothing deployed: deploy from here, choosing the team and the project. */
function NoSessions() {
  const [deploy, setDeploy] = useState(false);
  return (
    <div className="flex flex-col items-center gap-4">
      <span>No team is deployed yet, so there are no sessions.</span>
      <Button variant="primary" onClick={() => setDeploy(true)}>
        Deploy a team
      </Button>
      {/* Mounted only while open: each opening starts fresh. */}
      {deploy && <DeployDialog open onClose={() => setDeploy(false)} />}
    </div>
  );
}

function Centered({ children }: { children: React.ReactNode }) {
  return <div className="grid h-full place-items-center bg-bg p-12 text-tx3">{children}</div>;
}

/** A small square button holding an icon, labelled for screen readers and on hover. */
function IconButton({
  onClick,
  label,
  pressed,
  children,
}: {
  onClick: () => void;
  label: string;
  pressed?: boolean;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      aria-pressed={pressed}
      title={label}
      className={`flex size-7 items-center justify-center rounded-md ${
        pressed ? "bg-accSoft text-accTx" : "text-tx3 hover:bg-hover hover:text-tx"
      }`}
    >
      {children}
    </button>
  );
}
