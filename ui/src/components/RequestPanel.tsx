import { type MouseEvent, type ReactNode, useState } from "react";
import type { Request } from "../api/client";
import { FoldedText } from "./AgentText";
import { Button, KindChip } from "./ui";

// One request to the CEO, but a review (decided on its page). The Dashboard and
// a project's Inbox (Needs you) use this same panel, so they can't drift; the
// variant only changes what the specs say differs
// (wiki/product/features/dashboard.md#needs-you, wiki/product/features/project-pipeline.md#1-needs-you).

type Props = {
  request: Request;
  /** Who raised it; undefined when Foreman did. */
  who?: string;
  /** `<Team> on <project>`. */
  context: string;
  /** Settles once the answer is kept: a typed one stays in its field until then. */
  onAnswer: (answer: string) => Promise<unknown>;
  /** Closes it unanswered (wiki/product/features/requests.md#dismissing). */
  onDismiss: () => void;
  /** An answer is on its way: the options wait. */
  answering?: boolean;
  onOpenSession: () => void;
  /** Dashboard: avatar 32 with the amber ring. A project: kind chip first, avatar 24, the source. */
  variant?: "dashboard" | "project";
  /** A project: where it came from (`escalate(approval)`, `request_merge()`…). */
  source?: string;
  /** Under the text: e.g. the link line. */
  children?: ReactNode;
};

export function RequestPanel({
  request: r,
  who,
  context,
  onAnswer,
  onDismiss,
  answering = false,
  onOpenSession,
  variant = "dashboard",
  source,
  children,
}: Props) {
  const onProject = variant === "project";
  const [answer, setAnswer] = useState("");
  // An agent's escalation takes a typed answer; one waiting in its own session is answered there.
  const freeText =
    r.source === "agent" &&
    (r.kind === "approval" || r.kind === "question" || r.kind === "finding");
  // On the Dashboard the whole card opens the session of who raised it; its controls keep
  // their own action, and selecting text opens nothing.
  const opens = !onProject && r.instanceId !== "";
  const openCard = (e: MouseEvent<HTMLElement>) => {
    if (!opens || (e.target as HTMLElement).closest("a, button, input, textarea, label")) return;
    if (window.getSelection()?.toString()) return;
    onOpenSession();
  };
  return (
    // biome-ignore lint/a11y/useKeyWithClickEvents: the name button is the keyboard path
    <article
      onClick={openCard}
      className={`flex flex-col rounded-xl border border-line bg-panel ${onProject ? "gap-3 px-[18px] py-4" : "gap-3.5 px-5 py-[18px]"} ${opens ? "cursor-pointer hover:border-line3" : ""}`}
    >
      <header className="flex items-center gap-2.5">
        {onProject && <KindChip kind={r.kind} />}
        {/* Who raised it opens their session. With no agent behind it, Foreman raised it (a crash, a failed build): nothing to open. */}
        {r.instanceId ? (
          <button
            type="button"
            onClick={onOpenSession}
            title="Open the session"
            className="text-left font-semibold hover:text-accTx"
          >
            {who ?? r.instanceId}
          </button>
        ) : (
          <span className="font-semibold">Foreman</span>
        )}
        <span className="text-[13px] text-tx4">{context}</span>
        <span className="flex-1" />
        {onProject ? (
          <span className="font-mono text-[11.5px] text-tx4">{source}</span>
        ) : (
          <KindChip kind={r.kind} />
        )}
        <button
          type="button"
          aria-label="Dismiss"
          title="Dismiss: close it unanswered"
          disabled={answering}
          onClick={onDismiss}
          className="-my-1 -mr-2 grid size-7 shrink-0 place-items-center rounded-md text-[18px] text-tx4 hover:bg-hover hover:text-tx focus-visible:outline-2 focus-visible:outline-acc disabled:opacity-50"
        >
          ×
        </button>
      </header>
      <p className={`leading-[1.5] text-pretty ${onProject ? "text-[15.5px]" : "text-base"}`}>
        {r.summary}
      </p>
      <FoldedText text={r.body} />
      {children}
      <div className="flex flex-wrap items-center gap-2">
        {r.options.map((option, i) => (
          <Button
            key={option}
            variant={i === 0 ? "primary" : "secondary"}
            disabled={answering}
            className={`${i === 1 ? "!text-tx" : ""} ${onProject ? "" : "!text-[13.5px]"}`}
            onClick={() => onAnswer(option).catch(() => {})}
          >
            {option}
          </Button>
        ))}
        {/* On the Dashboard the card itself opens it; on a project this button does. Nothing to
            open for a Foreman-raised request. */}
        {onProject && r.instanceId !== "" && (
          <Button variant="ghost" onClick={onOpenSession}>
            Open session
          </Button>
        )}
      </div>
      {freeText && (
        <form
          className="flex gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            if (!answer.trim() || answering) return;
            // Kept until it reaches the agent: on a failure (a toast says why) it can be sent again.
            onAnswer(answer.trim()).then(
              () => setAnswer(""),
              () => {},
            );
          }}
        >
          <input
            value={answer}
            onChange={(e) => setAnswer(e.target.value)}
            placeholder="Or type an answer…"
            aria-label={`Answer ${who ?? r.instanceId}`}
            className="min-w-[180px] flex-1 rounded-[7px] border border-line2 bg-term px-3 py-[7px] text-tx placeholder:text-tx4 focus-visible:outline-2 focus-visible:outline-acc"
          />
          <Button type="submit" variant="secondary" disabled={answering || !answer.trim()}>
            Send
          </Button>
        </form>
      )}
    </article>
  );
}
