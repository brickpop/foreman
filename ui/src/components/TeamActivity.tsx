import { Link } from "@tanstack/react-router";
import { useState } from "react";
import type { Instance, Team, TeamAction, TeamMessage } from "../api/client";
import { type Filter, links, type Thread } from "../app/activity";
import { agentOf, waitingReason, when } from "../app/model";
import { useDraft, useInstances, useSettings } from "../app/queries";
import { CodeText } from "./CodeText";
import { Dot } from "./ui";

// A team's activity, the parts of a project's Activity section
// (wiki/product/features/project-activity.md): the map of who calls whom,
// which filters, and the rows: one per thread, which opens into a chat, and
// one per git action.

// The map: name boxes on an ellipse, tall enough that lines between the
// boxes stay apart (the CEO, 2026-11-02).
const width = 328;
const height = 260;
const rx = 118;
const ry = 100;

export type Names = (id: string) => string;

/** Who an id is, as the rows name them: an agent by its soul's name, the CEO
 * as You, Foreman as itself. */
export function namesOf(teams: Team[]): Names {
  return (id) => {
    if (id === "foreman") return "Foreman";
    if (id === "ceo") return "You";
    return agentOf(teams, id)?.soul.name ?? id.split("@")[0];
  };
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
export function CallMap({
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
  // The map is the team at work: Foreman's notices (a merge, a conflict, main
  // moving) stay in the list below, never as a teammate here. The CEO appears
  // when they sent something.
  const team = messages.filter((m) => m.fromInstance !== "foreman" && m.toInstance !== "foreman");
  for (const m of team)
    for (const id of [m.fromInstance, m.toInstance]) if (!ids.includes(id)) ids.push(id);
  const at = new Map(
    ids.map((id, i) => {
      const angle = -Math.PI / 2 + (2 * Math.PI * i) / Math.max(ids.length, 1);
      return [id, { x: width / 2 + rx * Math.cos(angle), y: height / 2 + ry * Math.sin(angle) }];
    }),
  );
  const pairs = links(team);
  const selected = (a: string, b: string) =>
    filter !== null && "a" in filter && [filter.a, filter.b].sort().join() === [a, b].join();
  const status = (id: string) => roster.find((i) => i.id === id)?.status;
  return (
    <div className="flex justify-center py-1.5">
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
              title={s ? `${name(id)} · ${s}` : undefined}
            >
              {s && <Dot status={s} />}
              {name(id)}
              {/* A queued dot is hard to tell from idle at this size: said in words. */}
              {s === "queued" && <span className="font-normal opacity-70">queued</span>}
            </button>
          );
        })}
      </div>
    </div>
  );
}

/** A thread as one row: who asked whom, and how it stands. A click opens it
 * as a chat: the first message, then the replies. Without onOpen, it opens
 * nowhere else (an older session has no sessions). */
export function ThreadRow({
  thread,
  name,
  onOpen,
}: {
  thread: Thread;
  name: Names;
  onOpen?: (instance: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const m = thread.message;
  const last = thread.replies.at(-1) ?? m;
  const replies = thread.replies.length;
  const waiting = useWaiting(m.state === "queued" ? m.toInstance : "", name);
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
            <span
              title={waiting}
              className={`text-[11.5px] ${m.state === "queued" ? "text-accTx" : "text-tx4"}`}
            >
              {state}
            </span>
          )}
          <span className="text-[11.5px] text-tx4">{when(last.createdAt)}</span>
        </span>
        {!open && <span className="truncate text-[12.5px] text-tx3">{m.body}</span>}
      </button>
      {open && (
        <div className="flex flex-col gap-2 px-4 pb-3">
          {[m, ...thread.replies].map((x) => (
            <Bubble key={x.id} message={x} name={name} first={x.fromInstance === m.fromInstance} />
          ))}
          {onOpen && m.fromInstance.includes("@") && (
            <button
              type="button"
              onClick={() => onOpen(m.fromInstance)}
              className="self-start text-[12px] text-accTx"
            >
              Open {name(m.fromInstance)}’s session
            </button>
          )}
        </div>
      )}
    </div>
  );
}

/** Why a message to instance still waits ("" for none): messages are
 * delivered when their recipient is idle. */
function useWaiting(instance: string, name: Names): string {
  const instances = useInstances().data ?? [];
  const max = useSettings().data?.settings.maxSessions ?? 0;
  const draft = useDraft(instance)?.onDraft === true;
  if (!instance) return "";
  const to = instances.find((i) => i.id === instance);
  return waitingReason(to, name(instance), instances, max, draft);
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
  const waiting = useWaiting(m.state === "queued" ? m.toInstance : "", name);
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
        <span className="text-[11px] text-tx4">{when(m.createdAt)}</span>
      </span>
      <div className="text-[13px] leading-[1.5] whitespace-pre-wrap text-tx2 select-text">
        <CodeText text={m.body} />
      </div>
      {m.artifactRef && (
        <span className="font-mono text-[11.5px] text-tx3">Artifact: {m.artifactRef}</span>
      )}
      {m.state === "queued" && <span className="text-[11.5px] text-accTx">Waiting: {waiting}</span>}
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
    // One repo: its message here. Several: each repo's line has its own.
    case "commit":
      return (
        <span className="text-[12.5px] text-tx2">
          {refused ? "tried to commit" : "committed"}{" "}
          {a.repos.length === 1 ? (
            <span className="text-tx">“{a.repos[0].message}”</span>
          ) : (
            <span className="font-mono text-[11.5px]">{a.repos.map((r) => r.repo).join(", ")}</span>
          )}
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
    case "set_submodule":
      return (
        <span className="text-[12.5px] text-tx2">
          {refused ? (
            "tried to set a submodule"
          ) : (
            <span className="text-tx">“{a.repos[0]?.message}”</span>
          )}
        </span>
      );
    case "request_merge":
      return (
        <span className="text-[12.5px] text-tx2">
          {refused ? "tried to request a merge of" : "requested a merge of"} <Ref>{a.branch}</Ref>
          {!refused && (
            <>
              {" "}
              <Link
                to="/projects/$project/inbox"
                params={{ project: a.instance.split("@")[1] ?? "" }}
                className="font-medium text-accTx hover:underline"
              >
                Review it
              </Link>
            </>
          )}
        </span>
      );
    default:
      return <span className="text-[12.5px] text-tx2">{a.tool}</span>;
  }
}

/** One git tool call: who did what, then what happened in each repo (or why
 * it was refused). */
export function ActionRow({ action: a, name }: { action: TeamAction; name: Names }) {
  return (
    <div className="flex flex-col gap-1 border-b border-line bg-side/40 px-4 py-2">
      <span className="flex min-w-0 items-center gap-1.5">
        <NameTag id={a.instance} name={name} />
        <span className="min-w-0 truncate">
          <ActionText action={a} />
        </span>
        <span className="flex-1" />
        {a.error !== "" && <span className="text-[11.5px] text-err">refused</span>}
        <span className="text-[11.5px] text-tx4">{when(a.createdAt)}</span>
      </span>
      {a.error !== "" ? (
        <span className="line-clamp-2 text-[12px] text-tx3">{a.error}</span>
      ) : (
        a.repos.map((r) => (
          <span key={r.repo} className="flex flex-wrap items-center gap-x-2 text-[11.5px] text-tx3">
            <span className="font-mono">{r.repo}</span>
            {r.branch && <Ref>{r.branch}</Ref>}
            {r.commit && <Ref>{r.commit.slice(0, 7)}</Ref>}
            {r.mergeRequest && <Ref>{r.mergeRequest}</Ref>}
            {a.repos.length > 1 && r.message && <span className="text-tx2">“{r.message}”</span>}
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

/** What the map filters the rows to, and the way back to all of them. */
export function FilterStrip({
  filter,
  name,
  onClear,
}: {
  filter: NonNullable<Filter>;
  name: Names;
  onClear: () => void;
}) {
  return (
    <div className="flex items-center gap-2 text-[12.5px] text-tx3">
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
      <button type="button" onClick={onClear} className="text-accTx">
        Show all
      </button>
    </div>
  );
}
