import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, useSearch } from "@tanstack/react-router";
import { type ReactNode, useEffect, useRef, useState } from "react";
import { type Instance, type Team, useClient } from "../api/client";
import { agentOf, harnessLabel, roleLabel, slotsTaken } from "../app/model";
import {
  keys,
  useDeployments,
  useDraft,
  useHarnesses,
  useInstances,
  useRepos,
  useRequests,
  useSettings,
  useTeams,
} from "../app/queries";
import { branchOf } from "../app/repos";
import {
  activeDeployments,
  applyMention,
  countdown,
  isLineShortcut,
  linePlaceholder,
  lineShortcut,
  mentionAt,
  mentionMatches,
  rosterOrder,
  type SessionsView,
  sessionsView,
  stoppedLine,
} from "../app/sessions";
import { ActivityPanel } from "../components/ActivityPanel";
import { Avatar } from "../components/Avatar";
import { DeployDialog } from "../components/DeployDialog";
import { CheckIcon, CopyIcon, PromptIcon, StopIcon } from "../components/icons";
import { StopDialog } from "../components/StopDialog";
import { Terminal, type TerminalHandle } from "../components/Terminal";
import { useToast } from "../components/Toasts";
import { Button, Dot, LeadTag, Loading, outlineLink } from "../components/ui";

// The terminal multiplexer (wiki/product/features/sessions.md): one deployment
// per page, picked in the sidebar; its agent tabs, the chosen agent's
// terminal, and the Activity panel.

// Each deployment's last view, so its sidebar link comes back to it (while
// the app runs).
const lastView = new Map<string, SessionsView>();

export function Sessions() {
  const search = useSearch({ from: "/sessions" });
  const navigate = useNavigate();
  const instances = useInstances();
  const teams = useTeams();
  const requests = useRequests();
  const deployments = useDeployments();
  // The last agent-tab click, counted: the terminal showing that agent takes
  // the keyboard (also when the click mounted it).
  const [focusAsk, setFocusAsk] = useState({ id: "", n: 0 });
  const [stopping, setStopping] = useState(false);
  const active = activeDeployments(instances.data ?? [], teams.data ?? [], requests.data ?? []);
  const rosterOf = (d: string) => rosterOrder(d, instances.data ?? [], teams.data ?? []);
  // Only once the team files are in: before them, who leads (the default
  // agent) isn't known (found 2026-10-16: a reload opened another agent).
  const ready = instances.isSuccess && teams.isSuccess;
  const view = ready
    ? sessionsView(
        search,
        active.map((d) => d.id),
        rosterOf,
        (d) => lastView.get(d),
      )
    : undefined;
  const show = (next: SessionsView) => navigate({ to: "/sessions", search: next, replace: true });
  // The URL always names what is shown: the sidebar marks it, and a reload or
  // a shared link shows the same.
  const shown = view && `${view.deployment} ${view.agent} ${view.view ?? ""}`;
  const asked = `${search.deployment} ${search.agent} ${search.view ?? ""}`;
  // biome-ignore lint/correctness/useExhaustiveDependencies: `shown` stands for the view
  useEffect(() => {
    if (!view) return;
    lastView.set(view.deployment, view);
    if (shown !== asked) show(view);
  }, [shown, asked]);

  if (!ready) return <Loading rows={5} className="m-12" />;
  if (!view)
    return (
      <Centered>
        <NoSessions />
      </Centered>
    );

  const d = active.find((a) => a.id === view.deployment);
  const team = teams.data.find((t) => t.id === view.deployment.split("@")[0]);
  const roster = rosterOf(view.deployment);
  const instance = roster.find((i) => i.agentId === view.agent);
  const activity = view.view === "activity";
  const toStop = deployments.data?.find((x) => x.id === view.deployment);
  const pick = (id: string) => {
    show({ ...view, agent: id.split("@")[0] });
    setFocusAsk((f) => ({ id, n: f.n + 1 }));
  };
  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex items-center gap-2.5 border-b border-line px-4 py-2.5">
        <Dot status={d?.status} title={d?.status} />
        <h1 className="m-0 truncate text-[15px] font-semibold">
          {d?.team} <span className="font-normal text-tx3">· {d?.project}</span>
        </h1>
        {d?.needsYou && <Dot title="Needs you" />}
        <span className="flex-1" />
        {/* Back to what the team works on: the project's tasks and their status. */}
        <Link
          to="/projects/$project"
          params={{ project: d?.project ?? "" }}
          search={{ tab: "board" }}
          className={outlineLink}
        >
          Board
        </Link>
        <button
          type="button"
          aria-pressed={activity}
          onClick={() => show({ ...view, view: activity ? undefined : "activity" })}
          className={`${outlineLink} ${activity ? "border-line3 bg-raised text-tx" : ""}`}
        >
          Activity
        </button>
        <Button
          variant="danger"
          className="inline-flex items-center gap-1.5"
          onClick={() => setStopping(true)}
        >
          <StopIcon />
          Stop
        </Button>
        {stopping && toStop && (
          <StopDialog
            deployment={toStop}
            teamName={d?.team ?? toStop.teamId}
            onClose={() => setStopping(false)}
          />
        )}
      </div>
      {instance && (
        <Session
          key={instance.id}
          instance={instance}
          teams={teams.data}
          lead={team?.config.lead}
          focusAsk={focusAsk}
          tabs={(controls) => (
            <AgentTabs roster={roster} team={team} shown={instance} onClick={pick}>
              {controls}
            </AgentTabs>
          )}
          aside={
            activity && (
              <ActivityPanel
                deployment={view.deployment}
                roster={roster}
                teams={teams.data}
                onClose={() => show({ ...view, view: undefined })}
                onOpen={pick}
              />
            )
          }
        />
      )}
    </div>
  );
}

/** The deployment's agents, the lead first; the one shown carries its branch.
 * The shown agent's controls (children) end the row. */
function AgentTabs({
  roster,
  team,
  shown,
  onClick,
  children,
}: {
  roster: Instance[];
  team?: Team;
  shown: Instance;
  onClick: (id: string) => void;
  children: ReactNode;
}) {
  const repos = useRepos(shown.id.split("@")[1]);
  const branch = branchOf(repos.data ?? [], shown.agentId);
  // A worktree is made when its agent starts: one not seen yet is looked for
  // again as the agent's status moves (git actions refresh the rest).
  const hasSession = !["asleep", "stopped", "queued"].includes(shown.status);
  // biome-ignore lint/correctness/useExhaustiveDependencies: on a status change only
  useEffect(() => {
    if (!branch && hasSession && repos.isSuccess) repos.refetch();
  }, [shown.status]);
  const tab = (i: Instance) => {
    const soul = team?.config.agents[i.agentId];
    const on = i.id === shown.id;
    return (
      <button
        key={i.id}
        type="button"
        onClick={() => onClick(i.id)}
        className={`flex shrink-0 items-center gap-2 px-3 py-2.5 font-medium ${on ? "text-tx shadow-[inset_0_-2px_0_var(--acc)]" : "text-tx3 hover:text-tx2"}`}
      >
        <Avatar look={soul?.look} size={24} status={i.status} dotBorder="var(--side)" />
        {soul?.name ?? i.agentId}
        {i.agentId === team?.config.lead && <LeadTag />}
        {on && branch && (
          <span className="font-mono text-[11.5px] font-normal text-tx4">{branch}</span>
        )}
      </button>
    );
  };
  const [lead, ...members] = roster;
  return (
    <nav className="flex items-center gap-2 border-b border-line bg-side px-3">
      <div className="flex min-w-0 flex-1 items-center overflow-x-auto [scrollbar-width:thin]">
        {lead && tab(lead)}
        {members.length > 0 && (
          <>
            <span className="mx-2 h-5 w-px shrink-0 bg-line2" />
            <span className="mr-1 shrink-0 text-xs text-tx4">Direct</span>
            {members.map(tab)}
          </>
        )}
      </div>
      {children}
    </nav>
  );
}

/** The agent shown: its tabs row (via `tabs`, given this agent's controls),
 * its terminal, and `aside` (the Activity panel) beside it. */
function Session({
  instance,
  teams,
  lead,
  focusAsk,
  tabs,
  aside,
}: {
  instance: Instance;
  teams: Team[];
  lead?: string;
  /** The last agent-tab click: the terminal showing that agent takes the keyboard. */
  focusAsk: { id: string; n: number };
  tabs: (controls: ReactNode) => ReactNode;
  aside: ReactNode;
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
  // Asleep, stopped or queued: no session runs; the body says why, and what to do.
  const asleep = ["asleep", "stopped", "queued"].includes(instance.status);
  const toggleLine = () => {
    if (asleep) return;
    if (!lineMode) setLineMode(true);
    else if (document.activeElement !== lineInput.current) lineInput.current?.focus();
    else closeLine();
  };
  // A tab click (and the mount it caused) takes the keyboard: Line when open,
  // else the terminal; an agent with no session has nothing to type into.
  // biome-ignore lint/correctness/useExhaustiveDependencies: on a click only (and on mounting, for the agent the click opened); opening or closing Line moves the keyboard itself
  useEffect(() => {
    if (focusAsk.id !== instance.id) return;
    if (lineMode) lineInput.current?.focus();
    else terminal.current?.focus();
  }, [focusAsk]);
  // The shortcut works anywhere on the screen. Listening on the window's
  // capture phase runs before the terminal, so the agent never gets it.
  const toggle = useRef(toggleLine);
  toggle.current = toggleLine;
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!isLineShortcut(e)) return;
      e.preventDefault();
      e.stopPropagation();
      toggle.current();
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, []);
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
  const copy = async () => {
    await navigator.clipboard.writeText(terminal.current?.text() ?? "");
    setCopied(true);
    setTimeout(() => setCopied(false), 1400);
  };
  return (
    <section className="flex min-h-0 min-w-0 flex-1 flex-col">
      {tabs(
        <span className="flex shrink-0 items-center gap-2.5 text-[12.5px] text-tx4">
          {agent ? roleLabel(agent.soul.role) : instance.agentId} · {harnessLabel(instance.harness)}
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
        </span>,
      )}
      <div className="flex min-h-0 flex-1">
        <div className="flex min-h-0 min-w-0 flex-1 flex-col bg-term">
          {instance.status === "queued" ? (
            <Queued name={name} />
          ) : asleep ? (
            <Asleep
              instance={instance.id}
              name={name}
              stopped={instance.status === "stopped"}
              reason={instance.stopReason}
            />
          ) : (
            <div className="flex min-h-0 flex-1">
              <div className="min-h-0 min-w-0 flex-1 px-4 py-3">
                <Terminal instance={instance.id} newline={newline} ref={terminal} />
              </div>
            </div>
          )}
          {draft && !asleep && (
            // A box inset from the edges, so the Activity panel beside it never hides
            // a side of its border.
            <div className="mx-3 my-2 flex items-center gap-3 rounded-lg border-2 border-acc bg-accSoft px-3.5 py-2 text-[12.5px] text-accTx">
              <span className="flex-1">
                {draft.waiting === 1 ? "1 message" : `${draft.waiting} messages`} waiting for your
                line: send or clear it.{" "}
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
                <span className="font-mono text-[13px] leading-[1.6] font-medium text-accTx">
                  ❯
                </span>
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
        </div>
        {aside}
      </div>
    </section>
  );
}

/** An asleep or stopped agent, in place of its terminal: no session runs until it is started,
 * and then it picks up its conversation. */
function Asleep({
  instance,
  name,
  stopped,
  reason,
}: {
  instance: string;
  name: string;
  stopped: boolean;
  reason: string;
}) {
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
          ? stoppedLine(name, reason)
          : `${name} is asleep: no session runs until there's work for them.`}
      </p>
      <Button disabled={wake.isPending} onClick={() => wake.mutate()}>
        {stopped ? `Start ${name} again` : `Start ${name}`}
      </Button>
    </div>
  );
}

/** A queued agent, in place of its terminal: every session slot is taken, idle agents included,
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
