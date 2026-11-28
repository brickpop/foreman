import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, useParams, useSearch } from "@tanstack/react-router";
import { type RefObject, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { type Instance, type Team, useClient } from "../../api/client";
import { agentOf, plural, slotsTaken } from "../../app/model";
import { latestStopped } from "../../app/project";
import {
  keys,
  useDraft,
  useHarnesses,
  useInstances,
  useProjectName,
  useRequests,
  useSettings,
  useStoppedDeployments,
  useTeams,
} from "../../app/queries";
import {
  activeDeployments,
  agentStep,
  applyMention,
  countdown,
  draftNotice,
  isLineShortcut,
  linePlaceholder,
  lineShortcut,
  mentionAt,
  mentionMatches,
  rosterOrder,
  sessionAgent,
  stepAgent,
  stoppedLine,
  waitingOnYou,
} from "../../app/sessions";
import { setUnsentLine, useUnsentLine } from "../../app/unsentLines";
import { DeployDialog } from "../../components/DeployDialog";
import { PlayIcon, PromptIcon } from "../../components/icons";
import { ReloadButton } from "../../components/ReloadButton";
import { Terminal, type TerminalHandle } from "../../components/Terminal";
import { useToast } from "../../components/Toasts";
import { Button, CopyButton, Dot, LeadTag, Loading, Spinner } from "../../components/ui";
import { ProjectBar, useProject } from "../Project";
import { AgentBranch, FilesMenu, WorktreesMenu } from "./SessionHeader";

// A project's Session section (wiki/product/features/sessions.md): the team
// on the project, its agent tabs and the chosen agent's terminal. The rest of
// the project is a click away in the rail.

// The agent last shown on each project, so coming back shows it again (while
// the app runs).
const lastAgent = new Map<string, string>();

export function Session() {
  const projectNameOf = useProjectName();
  const { project } = useParams({ from: "/projects/$project" });
  const search = useSearch({ from: "/projects/$project/session" });
  const navigate = useNavigate({ from: "/projects/$project/session" });
  const instances = useInstances();
  const teams = useTeams();
  const requests = useRequests();
  // The last agent-tab click, counted: the terminal showing that agent takes
  // the keyboard (also when the click mounted it).
  const [focusAsk, setFocusAsk] = useState({ id: "", n: 0 });
  const [controls, setControls] = useState<HTMLElement | null>(null);
  const d = activeDeployments(instances.data ?? [], teams.data ?? [], requests.data ?? []).find(
    (x) => x.project === project,
  );
  const roster = d ? rosterOrder(d.id, instances.data ?? [], teams.data ?? []) : [];
  // Only once the team files are in: before them, who leads (the default
  // agent) isn't known (found 2026-10-16: a reload opened another agent).
  const ready = instances.isSuccess && teams.isSuccess;
  const agent = ready ? sessionAgent(search.agent, roster, lastAgent.get(project)) : undefined;
  // The URL always names the agent shown: a reload or a shared link shows the same.
  useEffect(() => {
    if (!agent) return;
    lastAgent.set(project, agent);
    if (agent !== search.agent) navigate({ search: { agent }, replace: true });
  }, [project, agent, search.agent, navigate]);
  // Cmd+Shift+← / → move between the agent tabs, from anywhere on the page
  // (the window's capture phase, before the terminal: the agent never gets them).
  const step = useRef<(by: number) => void>(undefined);
  const lineHeld = useRef(false);
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const by = agentStep(e, lineHeld.current);
      if (!by || !step.current) return;
      e.preventDefault();
      e.stopPropagation();
      step.current(by);
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, []);
  step.current = undefined; // set below, only while a team is on the project

  if (!ready) return <Loading rows={5} className="m-6 max-[760px]:mt-14" />;
  if (!d) return <NoTeam project={project} />;

  const team = teams.data.find((t) => t.id === d.id.split("@")[0]);
  const instance = roster.find((i) => i.agentId === agent);
  const pick = (id: string) => {
    navigate({ search: { agent: id.split("@")[0] }, replace: true });
    setFocusAsk((f) => ({ id, n: f.n + 1 }));
  };
  step.current = (by) => {
    const next =
      instance &&
      stepAgent(
        roster.map((i) => i.id),
        instance.id,
        by,
      );
    if (next) pick(next);
  };
  return (
    <div className="flex h-full min-h-0 flex-col">
      {/* Under 760px, room on the left for the menu button, as the project's bar has. */}
      <div className="flex items-center gap-2.5 border-b border-line px-4 py-2.5 max-[760px]:pl-14">
        <Dot status={d.status} title={d.status} />
        <h1 className="m-0 min-w-16 truncate text-[15px] font-semibold">
          {projectNameOf(project)}{" "}
          <span className="font-normal text-tx3">
            ·{" "}
            <Link
              to="/projects/$project/team"
              params={{ project }}
              className="hover:text-tx hover:underline"
            >
              {d.team}
            </Link>
          </span>
        </h1>
        {instance && <AgentBranch key={instance.id} shown={instance} />}
        {d.needsYou && <Dot title="Needs you" />}
        <span className="ml-auto flex shrink-0 items-center gap-1.5">
          {instance && (
            <WorktreesMenu
              key={instance.id}
              shown={instance}
              name={team?.config.agents[instance.agentId]?.name ?? instance.agentId}
            />
          )}
          <FilesMenu />
        </span>
      </div>
      {instance && (
        <AgentTabs
          roster={roster}
          team={team}
          waiting={waitingOnYou(requests.data ?? [])}
          shown={instance}
          onClick={pick}
          controlsSlot={setControls}
        />
      )}
      {instance && (
        <AgentSession
          key={instance.id}
          instance={instance}
          teams={teams.data}
          lead={team?.config.lead}
          focusAsk={focusAsk}
          controls={controls}
          lineHeld={lineHeld}
        />
      )}
    </div>
  );
}

/** No team on the project: a quiet box in the middle of the screen, to deploy one,
 * the last one here preset. */
function NoTeam({ project }: { project: string }) {
  const projectNameOf = useProjectName();
  const [deploy, setDeploy] = useState(false);
  const stopped = useStoppedDeployments().data ?? [];
  return (
    <div className="flex h-full min-h-0 flex-col">
      <ProjectBar section="Session" />
      <div className="grid flex-1 place-items-center p-6">
        <div className="flex min-h-[220px] w-full max-w-[440px] flex-col items-center justify-center gap-2 rounded-xl border border-line2 px-6 text-center">
          <h2 className="m-0 text-[15px] font-semibold">No team on {projectNameOf(project)} yet</h2>
          <p className="m-0 text-[13.5px] text-tx3">Deploy a team to start its sessions.</p>
          <Button variant="primary" className="mt-3" onClick={() => setDeploy(true)}>
            Deploy a team
          </Button>
        </div>
      </div>
      {/* Mounted only while open: each opening starts fresh. */}
      {deploy && (
        <DeployDialog
          open
          onClose={() => setDeploy(false)}
          project={project}
          initialTeam={latestStopped(project, stopped)?.teamId}
        />
      )}
    </div>
  );
}

/** The deployment's agents, the lead first. It outlives the agent shown,
 * so picking a tab keeps the row's scroll; the shown agent's controls end
 * it, in the slot it hands over. */
function AgentTabs({
  roster,
  team,
  waiting,
  shown,
  onClick,
  controlsSlot,
}: {
  roster: Instance[];
  team?: Team;
  /** The agents waiting on the CEO's answer. */
  waiting: Set<string>;
  shown: Instance;
  onClick: (id: string) => void;
  controlsSlot: (el: HTMLElement | null) => void;
}) {
  const row = useRef<HTMLDivElement>(null);
  // A tab picked with the keyboard may be scrolled out of sight: bring it in.
  // biome-ignore lint/correctness/useExhaustiveDependencies: on a change of the agent shown only
  useEffect(() => {
    row.current
      ?.querySelector("[aria-current]")
      ?.scrollIntoView({ block: "nearest", inline: "nearest" });
  }, [shown.id]);
  const tab = (i: Instance) => {
    const soul = team?.config.agents[i.agentId];
    const on = i.id === shown.id;
    return (
      <button
        key={i.id}
        type="button"
        aria-current={on || undefined}
        onClick={() => onClick(i.id)}
        className={`flex shrink-0 items-center gap-2 px-3 py-2.5 font-medium ${on ? "text-tx shadow-[inset_0_-2px_0_var(--acc)]" : "text-tx3 hover:text-tx2"}`}
      >
        <Dot status={i.status} size={8} title={i.status} />
        {soul?.name ?? i.agentId}
        {i.agentId === team?.config.lead && <LeadTag name={soul?.name ?? i.agentId} />}
        {waiting.has(i.id) && <Dot title="Waiting on you" />}
      </button>
    );
  };
  const [lead, ...members] = roster;
  return (
    <nav className="flex items-center gap-2 border-b border-line bg-side px-3">
      <div
        ref={row}
        className="flex min-w-0 flex-1 items-center overflow-x-auto [scrollbar-width:thin]"
      >
        {lead && tab(lead)}
        {members.length > 0 && (
          <>
            <span className="mx-2 h-5 w-px shrink-0 bg-line2" />
            <span className="mr-1 shrink-0 text-xs text-tx4">Direct</span>
            {members.map(tab)}
          </>
        )}
      </div>
      {/* Only while some agent runs on an older prompt: the Team header always has it. */}
      {roster.some((i) => i.behind) && <ReloadButton instances={roster} variant="small" />}
      <span ref={controlsSlot} className="flex shrink-0" />
    </nav>
  );
}

/** The agent shown: its controls (into the tabs row's slot), its terminal. */
function AgentSession({
  instance,
  teams,
  lead,
  focusAsk,
  controls,
  lineHeld,
}: {
  instance: Instance;
  teams: Team[];
  lead?: string;
  /** The last agent-tab click: the terminal showing that agent takes the keyboard. */
  focusAsk: { id: string; n: number };
  /** Where its controls go: the end of the tabs row. */
  controls: HTMLElement | null;
  /** Set while its Line holds text: the agent keys select in it, not switch away. */
  lineHeld: RefObject<boolean>;
}) {
  const terminal = useRef<TerminalHandle>(null);
  const client = useClient();
  const draft = useDraft(instance.id);
  const newline = useHarnesses().data?.find((h) => h.id === instance.harness)?.newline;
  const line = useUnsentLine(instance.id);
  const setLine = (text: string) => setUnsentLine(instance.id, text);
  // Back on an agent with a line kept, Line opens with it.
  const [lineMode, setLineMode] = useState(line !== "");
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
  lineHeld.current = lineMode && !asleep && line !== "";
  // Gone (the agent left the roster), it holds nothing back.
  useEffect(() => {
    return () => {
      lineHeld.current = false;
    };
  }, [lineHeld]);
  // The line goes through Foreman: typed, then sent after a pause, like a
  // delivery (never mixed with one), so the harness takes the Enter as sending.
  const sendLine = useMutation({
    mutationFn: (text: string) => client.agents.line(instance.id, text),
    onSuccess: () => setLine(""),
  });
  const agent = agentOf(teams, instance.deploymentId, instance.id);
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
  return (
    <section className="flex min-h-0 min-w-0 flex-1 flex-col">
      {controls &&
        createPortal(
          <span className="flex items-center gap-2.5 text-[12.5px] text-tx4">
            {!asleep && (
              <CopyButton
                text={() => terminal.current?.text() ?? ""}
                label="Copy the whole buffer"
                className="size-7"
              />
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
          controls,
        )}
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
          <div className="flex min-h-0 flex-1 flex-col">
            {instance.status === "starting" && <Starting name={name} />}
            <div className="min-h-0 min-w-0 flex-1 px-4 py-3">
              <Terminal instance={instance.id} newline={newline} ref={terminal} />
            </div>
          </div>
        )}
        {draft && !asleep && (
          // A box inset from the edges, apart from the terminal.
          <div className="mx-3 my-2 flex items-center gap-3 rounded-lg border-2 border-attn bg-attnSoft px-3.5 py-2 text-[12.5px] text-attnTx">
            {draft.onDraft ? (
              <>
                <span className="flex-1">
                  {draftNotice(draft.waiting, name, true)}{" "}
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
                <Button variant="tiny" onClick={() => client.agents.sendNow(instance.id)}>
                  Send now
                </Button>
              </>
            ) : (
              // The line went out: they follow at the agent's next idle, never dropped.
              <span className="flex-1">{draftNotice(draft.waiting, name, false)}</span>
            )}
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
      <Button variant="outline" disabled={wake.isPending} onClick={() => wake.mutate()}>
        <PlayIcon />
        {stopped ? `Start ${name} again` : `Start ${name}`}
      </Button>
    </div>
  );
}

/** Over a starting agent's terminal, which stays (its harness may show something before it
 * reports in): Foreman prepares a worktree per code repo first, one at a time, its submodules
 * included, which a first start may take minutes over. */
function Starting({ name }: { name: string }) {
  const repos = useProject().repos.length;
  return (
    <p className="m-0 flex items-center gap-2.5 border-b border-line px-4 py-2.5 text-[13px] text-tx3">
      <span className="text-acc">
        <Spinner />
      </span>
      Starting {name}:{" "}
      {repos > 0 ? `its worktrees in ${plural(repos, "code repo")}, submodules too, then ` : ""}its
      session. A first start can take a few minutes.
    </p>
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
      <Link to="/settings/$tab" params={{ tab: "agents" }} className="text-[13.5px] text-accTx">
        Raise the limit in Settings
      </Link>
    </div>
  );
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
