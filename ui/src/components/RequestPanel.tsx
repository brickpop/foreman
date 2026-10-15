import { useState } from "react";
import type { Request } from "../api/client";
import type { Look } from "../api/types/workspace";
import { Avatar, chiefOfStaffLook } from "./Avatar";
import { Button, KindChip } from "./ui";

// One request to the CEO (wiki/product/features/home.md#request-list). Home
// and a project's Changes tab use this same panel, so they can't drift: only
// the extra context Changes adds differs (passed as children).

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
  onReview: () => void;
  onOpenChiefOfStaff: () => void;
  /** Home 16px, Changes 15.5px. */
  size?: "home" | "changes";
  children?: React.ReactNode;
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
  size = "home",
  children,
}: Props) {
  const fromChiefOfStaff = r.source === "cos";
  const [answer, setAnswer] = useState("");
  const freeText = r.kind === "approval" || r.kind === "question";
  return (
    <article className="flex flex-col gap-3.5 rounded-xl border border-line bg-panel px-5 py-[18px]">
      <header className="flex items-center gap-2.5">
        <Avatar
          look={fromChiefOfStaff ? chiefOfStaffLook : who?.look}
          size={32}
          ring="var(--acc)"
        />
        <span className="font-semibold">{fromChiefOfStaff ? "Chief of Staff" : who?.name}</span>
        <span className="text-[13px] text-tx4">{context}</span>
        <span className="flex-1" />
        <KindChip kind={r.kind} />
      </header>
      <p className={`leading-[1.5] text-pretty ${size === "home" ? "text-base" : "text-[15.5px]"}`}>
        {r.kind === "review" && reviewText ? reviewText : r.summary}
      </p>
      {children}
      <div className="flex flex-wrap items-center gap-2">
        {r.kind === "review" && !fromChiefOfStaff ? (
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
        )}
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
