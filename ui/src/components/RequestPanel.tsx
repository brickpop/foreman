import { type MouseEvent, type ReactNode, useState } from "react";
import type { Request } from "../api/client";
import type { Look } from "../api/types/workspace";
import { Avatar, chiefOfStaffLook } from "./Avatar";
import { Button, KindChip } from "./ui";

// One request to the CEO. Home and a project's Changes tab use this same panel,
// so they can't drift; the variant only changes what the specs say differs
// (wiki/product/features/home.md#request-list, wiki/product/features/changes.md#1-needs-you).

type Props = {
  request: Request;
  /** Who raised it; undefined for the Chief of Staff. */
  who?: { name: string; look: Look };
  /** `<Team> on <project>`, or `on <project>` for the Chief of Staff. */
  context: string;
  /** For a review: the sentence that replaces the summary. */
  reviewText?: string;
  onAnswer: (answer: string) => void;
  /** An answer is on its way: the options wait. */
  answering?: boolean;
  onOpenSession: () => void;
  /** Home: a review's Review button (opens Changes). */
  onReview?: () => void;
  onOpenChiefOfStaff: () => void;
  /** Home: avatar 32 with the amber ring. Changes: kind chip first, avatar 24, the source. */
  variant?: "home" | "changes";
  /** Changes: where it came from (`escalate(approval)`, `request_merge()`…). */
  source?: string;
  /** Under the text: e.g. the link line. */
  children?: ReactNode;
  /** Replaces the option buttons (Changes: a review's merge actions). */
  actions?: ReactNode;
};

export function RequestPanel({
  request: r,
  who,
  context,
  reviewText,
  onAnswer,
  answering = false,
  onOpenSession,
  onReview,
  onOpenChiefOfStaff,
  variant = "home",
  source,
  children,
  actions,
}: Props) {
  const fromChiefOfStaff = r.source === "cos";
  const changes = variant === "changes";
  const [answer, setAnswer] = useState("");
  const freeText = r.kind === "approval" || r.kind === "question";
  const look = fromChiefOfStaff ? chiefOfStaffLook : who?.look;
  // On Home the whole card opens who raised it (its session, or the Chief of
  // Staff); its controls keep their own action, and selecting text opens nothing.
  const opens = !changes && (fromChiefOfStaff || r.instanceId !== "");
  const openCard = (e: MouseEvent<HTMLElement>) => {
    if (!opens || (e.target as HTMLElement).closest("a, button, input, textarea, label")) return;
    if (window.getSelection()?.toString()) return;
    (fromChiefOfStaff ? onOpenChiefOfStaff : onOpenSession)();
  };
  return (
    // biome-ignore lint/a11y/useKeyWithClickEvents: the name button is the keyboard path
    <article
      onClick={openCard}
      className={`flex flex-col rounded-xl border border-line bg-panel ${changes ? "gap-3 px-[18px] py-4" : "gap-3.5 px-5 py-[18px]"} ${opens ? "cursor-pointer hover:border-line3" : ""}`}
    >
      <header className="flex items-center gap-2.5">
        {changes && <KindChip kind={r.kind} />}
        {/* Who raised it opens their session (the Chief of Staff: its drawer). With no
            agent behind it, Foreman raised it (a crash, a failed build): nothing to open. */}
        {fromChiefOfStaff || r.instanceId ? (
          <button
            type="button"
            onClick={fromChiefOfStaff ? onOpenChiefOfStaff : onOpenSession}
            title={fromChiefOfStaff ? "Open Chief of Staff" : "Open the session"}
            className="flex items-center gap-2.5 text-left"
          >
            <Avatar
              look={look}
              size={changes ? 24 : 32}
              ring={changes ? undefined : "var(--acc)"}
            />
            <span className="font-semibold">
              {fromChiefOfStaff ? "Chief of Staff" : (who?.name ?? r.instanceId)}
            </span>
          </button>
        ) : (
          <>
            <Avatar
              look={look}
              size={changes ? 24 : 32}
              ring={changes ? undefined : "var(--acc)"}
            />
            <span className="font-semibold">Foreman</span>
          </>
        )}
        <span className="text-[13px] text-tx4">{context}</span>
        <span className="flex-1" />
        {changes ? (
          <span className="font-mono text-[11.5px] text-tx4">{source}</span>
        ) : (
          <KindChip kind={r.kind} />
        )}
      </header>
      <p className={`leading-[1.5] text-pretty ${changes ? "text-[15.5px]" : "text-base"}`}>
        {r.kind === "review" && reviewText ? reviewText : r.summary}
      </p>
      {children}
      <div className="flex flex-wrap items-center gap-2">
        {actions ??
          (r.kind === "review" && onReview ? (
            <Button variant="primary" onClick={onReview}>
              Review
            </Button>
          ) : (
            r.options.map((option, i) => (
              <Button
                key={option}
                variant={i === 0 ? "primary" : "secondary"}
                disabled={answering}
                className={`${i === 1 ? "!text-tx" : ""} ${changes ? "" : "!text-[13.5px]"}`}
                onClick={() => onAnswer(option)}
              >
                {option}
              </Button>
            ))
          ))}
        {/* On Home the card itself opens it; in Changes this button does. Nothing to
            open for a Foreman-raised request. */}
        {changes && fromChiefOfStaff && (
          <Button variant="ghost" onClick={onOpenChiefOfStaff}>
            Open Chief of Staff
          </Button>
        )}
        {changes && !fromChiefOfStaff && r.instanceId !== "" && (
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
            onAnswer(answer.trim());
            setAnswer("");
          }}
        >
          <input
            value={answer}
            onChange={(e) => setAnswer(e.target.value)}
            placeholder="Or type an answer…"
            aria-label={`Answer ${who?.name ?? "the Chief of Staff"}`}
            className="min-w-[180px] flex-1 rounded-[7px] border border-line2 bg-term px-3 py-[7px] text-tx placeholder:text-tx4 focus-visible:outline-2 focus-visible:outline-acc"
          />
        </form>
      )}
    </article>
  );
}
