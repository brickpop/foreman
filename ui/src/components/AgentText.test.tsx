import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { FoldedText, firstLine, TextBox } from "./AgentText";

const html = (body: string) => renderToStaticMarkup(<FoldedText text={body} />);

// A request, a review: a one-line title and, when it needs more, a body in
// Markdown (wiki/product/models/request.md, wiki/product/models/review.md).
describe("an agent's body text", () => {
  it("shows nothing when there is none", () => {
    expect(html("")).toBe("");
    expect(html(" \n ")).toBe("");
  });

  it("renders its Markdown", () => {
    const got = html("Pick **one**:\n\n- Postgres\n- SQLite\n\n| a | b |\n|---|---|\n| 1 | 2 |");
    expect(got).toContain("<strong>one</strong>");
    expect(got).toContain("<li>Postgres</li>");
    expect(got).toContain("<td>1</td>");
  });

  it("shows a short one whole, with no toggle", () => {
    const got = html("Postgres has the extensions.");
    expect(got).not.toContain("folded");
    expect(got).not.toContain("Show all");
  });

  it("folds one of many lines", () => {
    const got = html(["- 1", "- 2", "- 3", "- 4", "- 5", "- 6", "- 7"].join("\n"));
    expect(got).toContain("folded");
    expect(got).toContain("Show all");
  });

  it("folds one long line", () => {
    const got = html("x".repeat(481));
    expect(got).toContain("folded");
    expect(got).toContain("Show all");
  });

  it("keeps an agent's raw HTML as text, and its links out of Foreman", () => {
    const got = html(
      '<img src=x onerror="alert(1)"> [docs](https://example.com) [x](javascript:alert(1))',
    );
    expect(got).not.toContain("<img");
    expect(got).toContain('href="https://example.com" target="_blank" rel="noreferrer"');
    expect(got).not.toContain("javascript:");
  });
});

// A review's description, in its header: one line in a box, the whole of it
// in a popover (the CEO, 2026-11-05).
describe("an agent's body text in a box", () => {
  it("shows nothing when there is none", () => {
    expect(renderToStaticMarkup(<TextBox text={" \n "} label="Description" />)).toBe("");
  });

  it("shows its first line, as plain words", () => {
    expect(firstLine("\n\n## The **plan**\n\nmore")).toBe("The plan");
    expect(firstLine("- Use `sqlc` for [the queries](https://x.dev)")).toBe(
      "Use sqlc for the queries",
    );
    expect(firstLine("1. First step")).toBe("First step");
    expect(firstLine("> quoted")).toBe("quoted");
    // A plain line keeps its words, numbers and hyphens.
    expect(firstLine("v2-api keeps 3 routes")).toBe("v2-api keeps 3 routes");
  });

  it("is a button holding the first line only, until opened", () => {
    const got = renderToStaticMarkup(
      <TextBox text={"Moves invoices to CSV\n\nSecret second paragraph"} label="Description" />,
    );
    expect(got).toContain('aria-label="Description"');
    expect(got).toContain("Moves invoices to CSV");
    expect(got).not.toContain("Secret second paragraph");
  });
});
