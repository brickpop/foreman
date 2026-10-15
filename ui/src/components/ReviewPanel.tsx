import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { type MergeRequest, type Request, useClient } from "../api/client";
import type { Look } from "../api/types/workspace";
import { filesChanged, forgeLabel, linkLine } from "../app/model";
import { keys, useDiff } from "../app/queries";
import { DiffView } from "./DiffView";
import { RequestPanel } from "./RequestPanel";
import { useToast } from "./Toasts";
import { Button } from "./ui";

// A review request with its merge request: what to look at (the link line
// opens the change) and what to decide: Merge to main, Create PR on the forge,
// or Reject with a message that goes back to its author. Home (Chief of Staff
// changes) and Changes use this same panel (wiki/product/features/changes.md).

export function ReviewPanel({
  request: r,
  mr,
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
  const client = useClient();
  const queries = useQueryClient();
  const toast = useToast();
  const [rejecting, setRejecting] = useState(false);
  const [reason, setReason] = useState("");
  const [viewing, setViewing] = useState(false);
  // A Chief of Staff change has no checks: its line says how many files it changes.
  const diff = useDiff(mr.authorInstance === "cos" ? mr.id : "");
  const act = useMutation({
    mutationFn: async (action: "merge" | "createPr" | "reject") => {
      if (action === "merge") await client.pipeline.merge(mr.id);
      if (action === "createPr") await client.pipeline.createPr(mr.id);
      if (action === "reject") await client.pipeline.reject(mr.id, reason.trim());
      // Merging or rejecting resolves the review request on the server (pipeline-and-merges.md).
    },
    onError: (e: Error) => toast({ title: "Not done", body: e.message, color: "var(--err)" }),
    onSettled: () => {
      queries.invalidateQueries({ queryKey: ["pipeline"] });
      queries.invalidateQueries({ queryKey: keys.requests });
    },
  });
  const line = linkLine(mr, forge, diff.data ? filesChanged(diff.data) : undefined);
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
        actions={
          <>
            {mr.forgePrUrl ? (
              <Button
                variant="primary"
                onClick={() => window.open(mr.forgePrUrl, "_blank", "noopener")}
              >
                Open PR ↗
              </Button>
            ) : (
              <>
                <Button
                  variant="primary"
                  disabled={act.isPending}
                  onClick={() => act.mutate("merge")}
                >
                  Merge to main
                </Button>
                {forge !== "none" && (
                  <Button disabled={act.isPending} onClick={() => act.mutate("createPr")}>
                    Create PR on {forgeLabel(forge)}
                  </Button>
                )}
              </>
            )}
            <Button variant="danger" onClick={() => setRejecting(true)}>
              Reject
            </Button>
          </>
        }
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
      {rejecting && (
        <div className="flex flex-col gap-2 rounded-xl border border-line bg-panel p-4">
          <textarea
            rows={3}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder={`What should ${author} change? (optional)`}
            aria-label={`Message to ${author}`}
            className="resize-y rounded-lg border border-line2 bg-term px-3 py-2 text-tx placeholder:text-tx4"
          />
          <span className="flex justify-end gap-2">
            <Button
              onClick={() => {
                setRejecting(false);
                setReason("");
              }}
            >
              Cancel
            </Button>
            <Button variant="danger" disabled={act.isPending} onClick={() => act.mutate("reject")}>
              Reject and tell {author}
            </Button>
          </span>
        </div>
      )}
    </div>
  );
}
