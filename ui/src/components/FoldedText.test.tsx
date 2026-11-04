import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { FoldedText } from "./FoldedText";

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
