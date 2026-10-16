import { useState } from "react";
import type { Instance, Team, TeamAction, TeamMessage } from "../api/client";
import { agentOf } from "../app/model";
import { useActions, useMessages } from "../app/queries";
import { entries, type Filter, links, shows, type Thread } from "../app/traffic";
import { CodeText } from "./CodeText";
import { Dot } from "./ui";

// The Team traffic panel (wiki/product/features/sessions.md#team-traffic): a
// small map of who calls whom, then the exchanges, newest first: one row per
// thread, which opens into a chat. One per deployment; the panes stay
// terminals only.

// The map: name boxes on an ellipse, compact.
const width = 328;
const height = 150;
const rx = 118;
const ry = 50;

type Names = (id: string) => string;

export function TeamTraffic({
  deployment,
  roster,
  teams,
  onClose,
  onOpen,
}: {
  deployment: string;
  roster: Instance[];
  teams: Team[];
  onClose: () => void;
  onOpen: (instance: string) => void;
}) {
  const messages = useMessages(deployment).data ?? [];
  const [filter, setFilter] = useState<Filter>(null);
  const [teamId, project] = deployment.split("@");
  const teamName = teams.find((t) => t.id === teamId)?.config.name ?? teamId;
  const name: Names = (id) => {
    if (id === "foreman") return "Foreman";
    if (id === "ceo") return "You";
    return agentOf(teams, id)?.soul.name ?? id.split("@")[0];
  };
  const actions = useActions(deployment).data ?? [];
  const shown = entries(messages, actions).filter((e) => shows(e, filter));
  return (
    <aside className="flex w-[360px] shrink-0 flex-col border-l border-line bg-panel">
      <header className="flex items-start justify-between border-b border-line px-4 py-3">
        <span className="flex flex-col">
          <span className="font-semibold">Team traffic</span>
          <span className="text-[12.5px] text-tx4">
            {teamName} on {project} only
          </span>
        </span>
        <button
          type="button"
          onClick={onClose}
          aria-label="Close team traffic"
          className="text-[18px] text-tx3 hover:text-tx"
        >
          ×
        </button>
      </header>
      <CallMap
        roster={roster}
        messages={messages}
        filter={filter}
        onFilter={setFilter}
        name={name}
      />
      {filter && (
        <div className="flex items-center gap-2 border-b border-line px-4 py-2 text-[12.5px] text-tx3">
          <span className="flex flex-1 items-center gap-1.5">
            {"agent" in filter ? (
              <>
                Only <NameTag id={filter.agent} name={name} />
              </>
            ) : (
              <>
                Only <NameTag id={filter.a} name={name} /> and <NameTag id={filter.b} name={name} />
              </>
            )}
          </span>
          <button type="button" onClick={() => setFilter(null)} className="text-accTx">
            Show all
          </button>
        </div>
      )}
      <div className="min-h-0 flex-1 overflow-auto">
        {shown.length === 0 && (
          <p className="m-0 px-4 py-6 text-center text-[13px] text-tx4">
            {messages.length + actions.length === 0 ? "Nothing yet." : "Nothing here."}
          </p>
        )}
        {shown.map((e) =>
          "thread" in e ? (
            <ThreadRow
              key={`m${e.thread.message.id}`}
              thread={e.thread}
              name={name}
              onOpen={onOpen}
            />
          ) : (
            <ActionRow key={`a${e.action.id}`} action={e.action} name={name} />
          ),
        )}
      </div>
    </aside>
  );
}

/** An agent's name as a label, in the primary colour; the CEO and Foreman
 * stand apart. */
function NameTag({ id, name }: { id: string; name: Names }) {
  const tone =
    id === "ceo"
      ? "border-line3 bg-raised text-tx"
      : id === "foreman"
        ? "border-line2 bg-raised text-tx3"
        : "border-accLine bg-accSoft text-accTx";
  return (
    <span
      className={`inline-flex items-center rounded-[5px] border px-1.5 text-[12px] leading-[18px] font-semibold ${tone}`}
    >
      {name(id)}
    </span>
  );
}

// What each kind of message does, as a verb, and its colour.
const verbs: Record<string, { verb: string; tone: string }> = {
  ask: { verb: "asked", tone: "text-info" },
  reply: { verb: "replied to", tone: "text-ok" },
  handoff: { verb: "handed off to", tone: "text-tx2" },
  contribute: { verb: "wrote to", tone: "text-tx3" },
  notice: { verb: "notified", tone: "text-tx3" },
};

/** The action between two names: a verb in the past tense, in the kind's colour. */
function Action({ kind }: { kind: string }) {
  const a = verbs[kind] ?? { verb: kind, tone: "text-tx3" };
  return <span className={`text-[12px] font-medium ${a.tone}`}>{a.verb}</span>;
}

/** Who did what to whom: <agent> <action> → <agent>. */
function Exchange({ message: m, name }: { message: TeamMessage; name: Names }) {
  return (
    <span className="flex min-w-0 items-center gap-1.5">
      <NameTag id={m.fromInstance} name={name} />
      <Action kind={m.kind} />
      <NameTag id={m.toInstance} name={name} />
    </span>
  );
}

/** Who calls whom: a box per agent, a line per pair that talked, with its
 * count (amber while a message between them waits). A click filters. */
function CallMap({
  roster,
  messages,
  filter,
  onFilter,
  name,
}: {
  roster: Instance[];
  messages: TeamMessage[];
  filter: Filter;
  onFilter: (f: Filter) => void;
  name: Names;
}) {
  const ids = roster.map((i) => i.id);
  // Foreman and the CEO appear only when they sent something.
  for (const m of messages)
    for (const id of [m.fromInstance, m.toInstance]) if (!ids.includes(id)) ids.push(id);
  const at = new Map(
    ids.map((id, i) => {
      const angle = -Math.PI / 2 + (2 * Math.PI * i) / Math.max(ids.length, 1);
      return [id, { x: width / 2 + rx * Math.cos(angle), y: height / 2 + ry * Math.sin(angle) }];
    }),
  );
  const pairs = links(messages);
  const selected = (a: string, b: string) =>
    filter !== null && "a" in filter && [filter.a, filter.b].sort().join() === [a, b].join();
  const status = (id: string) => roster.find((i) => i.id === id)?.status;
  return (
    <div className="flex justify-center border-b border-line py-1.5">
      <div className="relative" style={{ width, height }}>
        <svg width={width} height={height} className="absolute inset-0" aria-hidden="true">
          {pairs.map((l) => {
            const p = at.get(l.a);
            const q = at.get(l.b);
            if (!p || !q) return null;
            return (
              <line
                key={`${l.a} ${l.b}`}
                x1={p.x}
                y1={p.y}
                x2={q.x}
                y2={q.y}
                stroke={l.waiting ? "var(--acc)" : "var(--line3)"}
                strokeWidth={Math.min(1 + (l.ab + l.ba) / 3, 4)}
                strokeOpacity={!filter || selected(l.a, l.b) ? 0.8 : 0.3}
              />
            );
          })}
        </svg>
        {pairs.map((l) => {
          const p = at.get(l.a);
          const q = at.get(l.b);
          if (!p || !q) return null;
          const on = selected(l.a, l.b);
          // The pair's count is its button: it filters the list to the two of them.
          return (
            <button
              type="button"
              key={`${l.a} ${l.b}`}
              onClick={() => onFilter(on ? null : { a: l.a, b: l.b })}
              aria-pressed={on}
              title={`${name(l.a)} → ${name(l.b)}: ${l.ab} · ${name(l.b)} → ${name(l.a)}: ${l.ba}`}
              className={`absolute -translate-1/2 rounded-full border px-1.5 font-mono text-[10.5px] leading-[16px] ${
                l.waiting ? "border-acc bg-accSoft text-accTx" : "border-line2 bg-panel text-tx3"
              } ${on ? "ring-1 ring-tx" : ""}`}
              style={{ left: (p.x + q.x) / 2, top: (p.y + q.y) / 2 }}
            >
              {l.ab + l.ba}
            </button>
          );
        })}
        {ids.map((id) => {
          const p = at.get(id);
          if (!p) return null;
          const on = filter !== null && "agent" in filter && filter.agent === id;
          const s = status(id);
          return (
            <button
              key={id}
              type="button"
              onClick={() => onFilter(on ? null : { agent: id })}
              aria-pressed={on}
              className={`absolute flex -translate-1/2 items-center gap-1.5 rounded-md border px-2 py-[3px] text-[12px] font-semibold ${
                on
                  ? "border-acc bg-acc text-accInk"
                  : "border-accLine bg-accSoft text-accTx hover:border-acc"
              }`}
              style={{ left: p.x, top: p.y }}
            >
              {s && <Dot status={s} />}
              {name(id)}
            </button>
          );
        })}
      </div>
    </div>
  );
}

const time = (m: TeamMessage) => m.createdAt.slice(11, 16);

/** A thread as one row: who asked whom, and how it stands. A click opens it
 * as a chat: the first message, then the replies. */
function ThreadRow({
  thread,
  name,
  onOpen,
}: {
  thread: Thread;
  name: Names;
  onOpen: (instance: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const m = thread.message;
  const last = thread.replies.at(-1) ?? m;
  const replies = thread.replies.length;
  let state: string | undefined;
  if (m.state === "queued") state = "waiting";
  else if (m.state !== "delivered") state = m.state;
  else if (m.kind === "ask")
    state = replies === 0 ? "no reply yet" : `${replies} ${replies === 1 ? "reply" : "replies"}`;
  return (
    <div className="border-b border-line">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        aria-expanded={open}
        className="flex w-full flex-col gap-1 px-4 py-2.5 text-left hover:bg-hover"
      >
        <span className="flex items-center gap-1.5">
          <Exchange message={m} name={name} />
          <span className="flex-1" />
          {state && (
            <span className={`text-[11.5px] ${m.state === "queued" ? "text-accTx" : "text-tx4"}`}>
              {state}
            </span>
          )}
          <span className="text-[11.5px] text-tx4">{time(last)}</span>
        </span>
        {!open && <span className="truncate text-[12.5px] text-tx3">{m.body}</span>}
      </button>
      {open && (
        <div className="flex flex-col gap-2 px-4 pb-3">
          {[m, ...thread.replies].map((x) => (
            <Bubble key={x.id} message={x} name={name} first={x.fromInstance === m.fromInstance} />
          ))}
          {m.fromInstance.includes("@") && (
            <button
              type="button"
              onClick={() => onOpen(m.fromInstance)}
              className="self-start text-[12px] text-accTx"
            >
              Open {name(m.fromInstance)}’s pane
            </button>
          )}
        </div>
      )}
    </div>
  );
}

/** One message of an open thread, as a chat bubble: the asker on the left,
 * the other side on the right. */
function Bubble({
  message: m,
  name,
  first,
}: {
  message: TeamMessage;
  name: Names;
  first: boolean;
}) {
  return (
    <div
      className={`flex max-w-[88%] flex-col gap-1 rounded-xl border px-3 py-2 ${
        first
          ? "self-start rounded-tl-sm border-line bg-raised"
          : "self-end rounded-tr-sm border-line2 bg-panel"
      }`}
    >
      <span className="flex items-center gap-1.5">
        <NameTag id={m.fromInstance} name={name} />
        <span className="font-mono text-[10.5px] text-tx4">
          {m.kind} · {m.ref}
        </span>
        <span className="flex-1" />
        <span className="text-[11px] text-tx4">{time(m)}</span>
      </span>
      <div className="text-[13px] leading-[1.5] whitespace-pre-wrap text-tx2 select-text">
        <CodeText text={m.body} />
      </div>
      {m.artifactRef && (
        <span className="font-mono text-[11.5px] text-tx3">Artifact: {m.artifactRef}</span>
      )}
      {m.state === "queued" && (
        <span className="text-[11.5px] text-accTx">Waiting to be delivered</span>
      )}
      {m.reason && <span className="text-[11.5px] text-err">{m.reason}</span>}
    </div>
  );
}

/** A branch or a commit, as git shows it. */
function Ref({ children }: { children: React.ReactNode }) {
  return <span className="font-mono text-[11.5px] text-tx2">{children}</span>;
}

/** What a git tool did, in a few words: the verb and its object. */
function ActionText({ action: a }: { action: TeamAction }) {
  const refused = a.error !== "";
  switch (a.tool) {
    case "commit":
      return (
        <span className="text-[12.5px] text-tx2">
          {refused ? "tried to commit" : "committed"} <span className="text-tx">“{a.message}”</span>
        </span>
      );
    case "switch":
      return (
        <span className="text-[12.5px] text-tx2">
          {refused ? "tried to switch to" : "switched to"} <Ref>{a.branch}</Ref>
          {a.repos.some((r) => r.created) && <span className="text-tx4"> (new)</span>}
        </span>
      );
    case "merge":
      return (
        <span className="text-[12.5px] text-tx2">
          {refused ? "tried to merge" : "merged"} <Ref>{a.branch}</Ref>
        </span>
      );
    case "abort_merge":
      return (
        <span className="text-[12.5px] text-tx2">
          {refused ? "tried to abort a merge" : "aborted a merge"}
        </span>
      );
    default:
      return <span className="text-[12.5px] text-tx2">{a.tool}</span>;
  }
}

/** One git tool call: who did what, then what happened in each repo (or why
 * it was refused). */
function ActionRow({ action: a, name }: { action: TeamAction; name: Names }) {
  const time = a.createdAt.slice(11, 16);
  return (
    <div className="flex flex-col gap-1 border-b border-line bg-side/40 px-4 py-2">
      <span className="flex min-w-0 items-center gap-1.5">
        <NameTag id={a.instance} name={name} />
        <span className="min-w-0 truncate">
          <ActionText action={a} />
        </span>
        <span className="flex-1" />
        {a.error !== "" && <span className="text-[11.5px] text-err">refused</span>}
        <span className="text-[11.5px] text-tx4">{time}</span>
      </span>
      {a.error !== "" ? (
        <span className="line-clamp-2 text-[12px] text-tx3">{a.error}</span>
      ) : (
        a.repos.map((r) => (
          <span key={r.repo} className="flex flex-wrap items-center gap-x-2 text-[11.5px] text-tx3">
            <span className="font-mono">{r.repo}</span>
            {r.branch && <Ref>{r.branch}</Ref>}
            {r.commit && <Ref>{r.commit.slice(0, 7)}</Ref>}
            {(r.conflicts ?? []).length > 0 && (
              <span className="text-accTx">
                {r.conflicts.length === 1 ? "1 conflict" : `${r.conflicts.length} conflicts`}:{" "}
                {r.conflicts.join(", ")}
              </span>
            )}
          </span>
        ))
      )}
    </div>
  );
}
