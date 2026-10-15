import { Link, useNavigate, useSearch } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";
import type { Instance, Team, TeamMessage } from "../api/client";
import { agentOf, harnessLabel } from "../app/model";
import {
  useHarnesses,
  useInstances,
  useMessages,
  useRequests,
  useSettings,
  useTeams,
} from "../app/queries";
import {
  clickTab,
  closePane,
  deploymentTabs,
  initialPanes,
  isLineShortcut,
  linePlaceholder,
  lineShortcut,
  type Panes,
  rosterOrder,
  toggleSplit,
  typedLine,
} from "../app/sessions";
import { Avatar } from "../components/Avatar";
import { DemoBadge } from "../components/DemoBadge";
import { Terminal, type TerminalHandle } from "../components/Terminal";
import { Button, Dot, LeadTag, Segmented } from "../components/ui";

// The terminal multiplexer (wiki/product/features/sessions.md): deployment
// tabs, agent tabs, one or two panes of real terminals, markers and the
// Messages drawer.

// The panes of each deployment, kept while the app runs (the server-side
// ui_prefs that keep them across browsers come with the UI state API).
const remembered = new Map<string, Panes>();

export function Sessions() {
  const search = useSearch({ from: "/sessions" });
  const navigate = useNavigate();
  const instances = useInstances();
  const teams = useTeams();
  const requests = useRequests();
  const tabs = deploymentTabs(instances.data ?? [], teams.data ?? [], requests.data ?? []);
  const deployment = search.deployment ?? tabs[0]?.id ?? "";
  const roster = rosterOrder(deployment, instances.data ?? [], teams.data ?? []);
  const [panes, setPanesState] = useState<Panes>(
    () => remembered.get(deployment) ?? initialPanes(roster),
  );
  const [messages, setMessages] = useState(false);
  const setPanes = (p: Panes) => {
    remembered.set(deployment, p);
    setPanesState(p);
  };

  // A deployment switch restores its panes; ?agent= opens that agent.
  // biome-ignore lint/correctness/useExhaustiveDependencies: on deployment, agent or data changes only
  useEffect(() => {
    let p = remembered.get(deployment) ?? initialPanes(roster);
    if (p.open.length === 0 && roster.length > 0) p = initialPanes(roster);
    if (search.agent && roster.some((i) => i.id === search.agent)) p = clickTab(p, search.agent);
    remembered.set(deployment, p);
    setPanesState(p);
  }, [deployment, search.agent, roster.length]);

  if (instances.isPending) return <Centered>…</Centered>;
  if (tabs.length === 0)
    return <Centered>No team is deployed yet, so there are no sessions.</Centered>;

  const team = teams.data?.find((t) => t.id === deployment.split("@")[0]);
  const byId = new Map(roster.map((i) => [i.id, i]));
  const shown = panes.open.filter((id) => byId.has(id));
  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex items-center gap-3 border-b border-line px-4 py-2.5">
        <Segmented
          label="Deployments"
          value={deployment}
          onChange={(d) => navigate({ to: "/sessions", search: { deployment: d } })}
          options={tabs.map((t) => ({
            value: t.id,
            label: (
              <span className="flex items-center gap-1.5">
                {t.label}
                {t.needsYou && <Dot title="Needs you" />}
              </span>
            ),
          }))}
        />
        <DemoBadge areas={["agents", "messages"]} />
        <span className="flex-1" />
        <Button
          variant={panes.open.length === 2 ? "raised" : "secondary"}
          onClick={() => setPanes(toggleSplit(panes, roster))}
        >
          {panes.open.length === 2 ? "Single" : "Split"}
        </Button>
        <Button variant={messages ? "raised" : "secondary"} onClick={() => setMessages(!messages)}>
          Messages
        </Button>
      </div>
      <AgentTabs
        roster={roster}
        team={team}
        panes={panes}
        onClick={(id) => setPanes(clickTab(panes, id))}
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
              split={shown.length === 2}
              onActivate={() => setPanes({ ...panes, active: index })}
              onClose={() => setPanes(closePane(panes, index))}
            />
          ))}
        </div>
        {messages && (
          <MessagesDrawer
            deployment={deployment}
            teams={teams.data ?? []}
            onClose={() => setMessages(false)}
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
  split,
  onActivate,
  onClose,
}: {
  instance: Instance;
  teams: Team[];
  lead?: string;
  active: boolean;
  split: boolean;
  onActivate: () => void;
  onClose: () => void;
}) {
  const terminal = useRef<TerminalHandle>(null);
  const settings = useSettings();
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
  const toggleLine = () => {
    if (!lineMode) setLineMode(true);
    else if (document.activeElement !== lineInput.current) lineInput.current?.focus();
    else closeLine();
  };
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
  const agent = agentOf(teams, instance.id);
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
        <Button
          variant="tiny"
          onClick={() => (lineMode ? closeLine() : setLineMode(true))}
          aria-pressed={lineMode}
          title={`Line input (${lineShortcut()})`}
        >
          Line
        </Button>
        <Button variant="tiny" onClick={copy}>
          {copied ? "Copied" : "Copy"}
        </Button>
        {split && (
          <Button variant="tiny" onClick={onClose} aria-label={`Close ${name}'s pane`}>
            ×
          </Button>
        )}
      </header>
      {!isLead && (
        <div className="border-b border-line bg-side px-4 py-[7px] text-[12.5px] text-tx3">
          Direct line to {name}. They'll tell {leadName}, the lead, if it matters.
        </div>
      )}
      <div className="flex min-h-0 flex-1">
        <div className="min-h-0 min-w-0 flex-1 px-4 py-3">
          <Terminal instance={instance.id} newline={newline} ref={terminal} />
        </div>
        {settings.data?.settings.teamMessageMarkers !== false && (
          <Markers deployment={instance.deploymentId} instance={instance.id} teams={teams} />
        )}
      </div>
      {lineMode && (
        <div className="flex items-start gap-2 border-t border-line px-[18px] py-[11px]">
          <span className="font-mono text-[13px] leading-[1.5] font-medium text-accTx">❯</span>
          {/* Enter sends; Shift+Enter starts a new line, typed as the harness's newline. */}
          <textarea
            ref={lineInput}
            // biome-ignore lint/a11y/noAutofocus: opening Line is asking to type in it
            autoFocus
            value={line}
            rows={Math.min(line.split("\n").length, 8)}
            onChange={(e) => setLine(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Escape") return closeLine();
              if (e.key !== "Enter" || e.nativeEvent.isComposing) return;
              if (e.shiftKey && newline) return; // the textarea adds the line
              e.preventDefault();
              // An empty line still sends Enter (it answers a harness's prompt).
              // Without a known newline, each pasted line is sent on its own.
              terminal.current?.send(typedLine(line, newline ?? "\r"));
              setLine("");
            }}
            placeholder={linePlaceholder(name, instance.status, isLead)}
            aria-label={`Line input to ${name}`}
            className="flex-1 resize-none bg-transparent leading-[1.5] text-tx placeholder:text-tx4 focus:outline-none"
          />
        </div>
      )}
    </section>
  );
}

/** The ⇄ markers of this agent's traffic, beside its terminal (newest last). */
function Markers({
  deployment,
  instance,
  teams,
}: {
  deployment: string;
  instance: string;
  teams: Team[];
}) {
  const messages = useMessages(deployment);
  const mine = (messages.data ?? [])
    .filter((m) => m.fromInstance === instance || m.toInstance === instance)
    .slice(-8);
  if (mine.length === 0) return null;
  return (
    <aside className="flex w-[220px] shrink-0 flex-col justify-end gap-1 border-l border-line px-3 py-3 font-mono text-[11.5px] text-team">
      {mine.map((m) => (
        <div key={m.id} className="truncate" title={m.body}>
          ⇄ {markerText(m, teams)}
        </div>
      ))}
    </aside>
  );
}

function markerText(m: TeamMessage, teams: Team[]): string {
  const name = (id: string) => agentOf(teams, id)?.soul.name ?? id;
  if (m.kind === "handoff")
    return `${name(m.fromInstance)} handed off ${m.artifactRef} to ${name(m.toInstance)}`;
  return `${name(m.fromInstance)} → ${name(m.toInstance)} · ${m.kind}${m.requestRef ? ` · ${m.requestRef}` : ""}`;
}

/** The team-message traffic of the deployment, newest first. It narrows the
 * panes rather than covering them. */
function MessagesDrawer({
  deployment,
  teams,
  onClose,
  onOpen,
}: {
  deployment: string;
  teams: Team[];
  onClose: () => void;
  onOpen: (instance: string) => void;
}) {
  const messages = useMessages(deployment);
  const [teamId, project] = deployment.split("@");
  const teamName = teams.find((t) => t.id === teamId)?.config.name ?? teamId;
  const name = (id: string) => agentOf(teams, id)?.soul.name ?? id;
  const rows = [...(messages.data ?? [])].reverse();
  return (
    <aside className="flex w-[340px] shrink-0 flex-col border-l border-line bg-panel">
      <header className="flex items-start justify-between border-b border-line px-4 py-3">
        <span className="flex flex-col">
          <span className="font-semibold">Messages between agents</span>
          <span className="text-[12.5px] text-tx4">
            {teamName} on {project} only
          </span>
        </span>
        <button
          type="button"
          onClick={onClose}
          aria-label="Close messages"
          className="text-[18px] text-tx3 hover:text-tx"
        >
          ×
        </button>
      </header>
      <div className="min-h-0 flex-1 overflow-auto">
        {rows.map((m) => (
          <button
            key={m.id}
            type="button"
            onClick={() => onOpen(m.fromInstance)}
            className="flex w-full flex-col gap-1 border-b border-line px-4 py-3 text-left hover:bg-hover"
          >
            <span className="flex items-center gap-1.5 text-[13px]">
              <span className="font-semibold">{name(m.fromInstance)}</span>
              <span className="text-team">→</span>
              <span className="font-semibold">{name(m.toInstance)}</span>
              <span className="font-mono text-[11.5px] text-tx4">{m.kind}</span>
              <span className="flex-1" />
              <span className="text-xs text-tx4">{m.createdAt.slice(11, 16)}</span>
            </span>
            <span className="text-[13px] leading-[1.45] text-tx2">{m.body}</span>
          </button>
        ))}
      </div>
    </aside>
  );
}

function Centered({ children }: { children: React.ReactNode }) {
  return <div className="grid h-full place-items-center bg-bg p-12 text-tx3">{children}</div>;
}
