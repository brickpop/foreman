import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { Request } from "../api/client";
import { RequestPanel } from "./RequestPanel";

const request = (r: Partial<Request>): Request => ({
  id: "req-1",
  kind: "finding",
  projectId: "ledger",
  deploymentId: "atlas@ledger",
  instanceId: "mara@ledger",
  summary: "The importer drops rows",
  body: "",
  options: [],
  source: "agent",
  status: "open",
  answer: "",
  createdAt: "",
  resolvedAt: "",
  reviewId: "",
  links: [],
  ...r,
});

const html = (r: Partial<Request>) =>
  renderToStaticMarkup(
    <RequestPanel
      request={request(r)}
      who="Mara"
      context="Atlas on Ledger"
      onAnswer={async () => {}}
      onDismiss={() => {}}
      onOpenSession={() => {}}
      variant="project"
    />,
  );

// wiki/product/features/requests.md: every card can be dismissed; an agent's
// escalation takes a typed answer, sent with Enter or its Send button.
describe("a request's panel", () => {
  it("dismisses any card, whoever raised it", () => {
    for (const source of ["agent", "harness", "foreman"]) {
      expect(html({ source, instanceId: source === "foreman" ? "" : "mara@ledger" })).toContain(
        'aria-label="Dismiss"',
      );
    }
  });

  it("sends an agent's escalation a typed answer, with a visible button", () => {
    const got = html({ source: "agent" });
    expect(got).toContain('aria-label="Answer Mara"');
    expect(got).toMatch(/<button type="submit"[^>]*disabled=""[^>]*>Send<\/button>/);
  });

  it("answers a harness's own question in its session, not here", () => {
    const got = html({ source: "harness", kind: "question" });
    expect(got).not.toContain("Answer Mara");
    expect(got).not.toContain(">Send<");
  });
});
