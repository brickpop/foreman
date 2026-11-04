import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { FoldedText } from "./ui";

const html = (body: string) => renderToStaticMarkup(<FoldedText text={body} />);

// A request, a review: a one-line title and, when it needs more, a body
// (wiki/product/models/request.md, wiki/product/models/review.md).
describe("an agent's body text", () => {
  it("shows nothing when there is none", () => {
    expect(html("")).toBe("");
    expect(html(" \n ")).toBe("");
  });

  it("shows a short one whole, with no toggle", () => {
    const got = html("Postgres has the extensions.\nSQLite is one file.");
    expect(got).toContain("Postgres has the extensions.\nSQLite is one file.");
    expect(got).not.toContain("line-clamp");
    expect(got).not.toContain("Show all");
  });

  it("folds one of many lines", () => {
    const got = html(["1", "2", "3", "4", "5", "6", "7"].join("\n"));
    expect(got).toContain("line-clamp-6");
    expect(got).toContain("Show all");
  });

  it("folds one long line", () => {
    const got = html("x".repeat(481));
    expect(got).toContain("line-clamp-6");
    expect(got).toContain("Show all");
  });

  it("keeps an agent's markup as text", () => {
    expect(html("<b>bold</b>")).toContain("&lt;b&gt;bold&lt;/b&gt;");
  });
});
