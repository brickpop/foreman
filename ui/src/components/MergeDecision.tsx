import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { type MergeRequest, useClient } from "../api/client";
import { forgeLabel } from "../app/model";
import { keys } from "../app/queries";
import { useToast } from "./Toasts";
import { Button } from "./ui";

// The CEO's decision on a merge request: Merge to main (the project's
// protected branch), Create PR on its forge, or Reject with a message to its
// author (wiki/product/features/pipeline-and-merges.md#review-and-merge). The
// review panel and a merge request's row in Changes share it.

export type Decision = ReturnType<typeof useMergeDecision>;

export function useMergeDecision(mr: MergeRequest) {
  const client = useClient();
  const queries = useQueryClient();
  const toast = useToast();
  const [rejecting, setRejecting] = useState(false);
  const [reason, setReason] = useState("");
  const act = useMutation({
    mutationFn: async (action: "merge" | "createPr" | "reject") => {
      if (action === "merge") await client.pipeline.merge(mr.id);
      if (action === "createPr") await client.pipeline.createPr(mr.id);
      if (action === "reject") await client.pipeline.reject(mr.id, reason.trim());
    },
    onSuccess: (_, action) => {
      if (action === "reject") {
        setRejecting(false);
        setReason("");
      }
    },
    onError: (e: Error) => toast({ title: "Not done", body: e.message, color: "var(--err)" }),
    onSettled: () => {
      queries.invalidateQueries({ queryKey: ["pipeline"] });
      queries.invalidateQueries({ queryKey: keys.requests });
    },
  });
  return { act, rejecting, setRejecting, reason, setReason };
}

/** Merge to <main> and Create PR (or Open PR once there is one), then Reject. */
export function DecisionButtons({
  decision: d,
  mr,
  main,
  forge,
  size,
}: {
  decision: Decision;
  mr: MergeRequest;
  main: string;
  forge: string;
  size?: "small";
}) {
  return (
    <>
      {mr.forgePrUrl ? (
        <Button
          variant={size ?? "primary"}
          onClick={() => window.open(mr.forgePrUrl, "_blank", "noopener")}
        >
          Open PR ↗
        </Button>
      ) : (
        <>
          <Button
            variant={size ?? "primary"}
            disabled={d.act.isPending}
            onClick={() => d.act.mutate("merge")}
          >
            Merge to {main}
          </Button>
          {forge !== "none" && (
            <Button
              variant={size}
              disabled={d.act.isPending}
              onClick={() => d.act.mutate("createPr")}
            >
              Create PR on {forgeLabel(forge)}
            </Button>
          )}
        </>
      )}
      <Button variant={size ?? "danger"} onClick={() => d.setRejecting(true)}>
        Reject
      </Button>
    </>
  );
}

/** The message a rejection carries to its author, while rejecting. */
export function RejectForm({ decision: d, author }: { decision: Decision; author: string }) {
  if (!d.rejecting) return null;
  return (
    <div className="flex flex-col gap-2 rounded-xl border border-line bg-panel p-4">
      <textarea
        rows={3}
        value={d.reason}
        onChange={(e) => d.setReason(e.target.value)}
        placeholder={`What should ${author} change? (optional)`}
        aria-label={`Message to ${author}`}
        className="resize-y rounded-lg border border-line2 bg-term px-3 py-2 text-tx placeholder:text-tx4"
      />
      <span className="flex justify-end gap-2">
        <Button
          onClick={() => {
            d.setRejecting(false);
            d.setReason("");
          }}
        >
          Cancel
        </Button>
        <Button variant="danger" disabled={d.act.isPending} onClick={() => d.act.mutate("reject")}>
          Reject and tell {author}
        </Button>
      </span>
    </div>
  );
}
