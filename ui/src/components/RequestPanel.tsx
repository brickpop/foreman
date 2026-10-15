import { type ReactNode, useState } from "react";
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
  return (
    <article
      className={`flex flex-col rounded-xl border border-line bg-panel ${changes ? "gap-3 px-[18px] py-4" : "gap-3.5 px-5 py-[18px]"}`}
    >
      <header className="flex items-center gap-2.5">
        {changes && <KindChip kind={r.kind} />}
        <Avatar look={look} size={changes ? 24 : 32} ring={changes ? undefined : "var(--acc)"} />
        <span className="font-semibold">{fromChiefOfStaff ? "Chief of Staff" : who?.name}</span>
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
                className={i === 1 ? "!text-tx" : ""}
                onClick={() => onAnswer(option)}
              >
                {option}
              </Button>
            ))
          ))}
        {fromChiefOfStaff ? (
          <Button variant="ghost" onClick={onOpenChiefOfStaff}>
            Open Chief of Staff
          </Button>
        ) : (
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
            if (answer.trim()) onAnswer(answer.trim());
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
