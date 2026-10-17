import { useState } from "react";
import type { MergeRequest, Request } from "../api/client";
import type { Look } from "../api/types/workspace";
import { filesChanged, linkLine } from "../app/model";
import { useDiff } from "../app/queries";
import { DiffView } from "./DiffView";
import { DecisionButtons, RejectForm, useMergeDecision } from "./MergeDecision";
import { RequestPanel } from "./RequestPanel";

// A review request with its merge request: what to look at (the link line
// opens the change) and what to decide: Merge to main, Create PR on the forge,
// or Reject with a message that goes back to its author. Home (Chief of Staff
// changes) and Changes use this same panel (wiki/product/features/changes.md).

export function ReviewPanel({
  request: r,
  mr,
  main = "main",
  forge,
  who,
  context,
  reviewText,
  author,
  variant,
  source,
  onOpenSession,
  onOpenChiefOfStaff,
}: {
  request: Request;
  mr: MergeRequest;
  /** The branch it merges into: the project's protected branch (`main` in project.toml). */
  main?: string;
  /** The forge of the repo it targets ("none": no Create PR). */
  forge: string;
  who?: { name: string; look: Look };
  context: string;
  reviewText?: string;
  /** Who a rejection is sent to: the lead, or the Chief of Staff. */
  author: string;
  variant: "home" | "changes";
  source?: string;
  onOpenSession: () => void;
  onOpenChiefOfStaff: () => void;
}) {
  const decision = useMergeDecision(mr);
  const [viewing, setViewing] = useState(false);
  // A Chief of Staff change has no checks: its line says how many files it changes.
  const diff = useDiff(mr.authorInstance === "cos" ? mr.id : "");
  const line = linkLine(mr, forge, diff.data ? filesChanged(diff.data) : undefined, main);
  return (
    <div className="flex flex-col gap-2">
      <RequestPanel
        request={r}
        variant={variant}
        source={source}
        who={who}
        context={context}
        reviewText={reviewText}
        onAnswer={() => {}}
        onOpenSession={onOpenSession}
        onOpenChiefOfStaff={onOpenChiefOfStaff}
        actions={<DecisionButtons decision={decision} mr={mr} main={main} forge={forge} />}
      >
        {/* The change itself: the PR on the forge once there is one, its diff before. */}
        {mr.forgePrUrl ? (
          <a
            href={mr.forgePrUrl}
            target="_blank"
            rel="noopener"
            className="font-mono text-[12.5px] text-accTx"
          >
            {line}
          </a>
        ) : (
          <button
            type="button"
            onClick={() => setViewing(true)}
            title="See what it changes"
            className={`self-start text-left font-mono text-[12.5px] ${mr.conflicting ? "text-err" : "text-accTx"}`}
          >
            {line}
          </button>
        )}
      </RequestPanel>
      {viewing && (
        <DiffView mergeRequest={mr.id} title={mr.branch} onClose={() => setViewing(false)} />
      )}
      <RejectForm decision={decision} author={author} />
    </div>
  );
}
