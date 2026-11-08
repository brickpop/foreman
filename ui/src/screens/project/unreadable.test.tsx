import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { plural } from "../../app/model";
import { FrontmatterError } from "../../components/Frontmatter";
import { UnreadNotice } from "./Board";
import { UnreadableMark } from "./Wiki";

// An entry whose header the wiki library couldn't read has no fields. The reader,
// the tree and the board each say so with the library's reason, as the server
// sends it (frontmatterError, Board.unread).

const reason = "line 2: mapping values are not allowed in this context";
const text = (html: string) => html.replace(/<[^>]+>/g, "").replace(/&#x27;/g, "'");

describe("an entry whose header couldn't be read", () => {
  it("says so above the body, with the reason and what to do", () => {
    expect(text(renderToStaticMarkup(<FrontmatterError error={reason} />))).toBe(
      `This entry's frontmatter couldn't be read: ${reason}. Fix it, then run wiki check.`,
    );
  });

  // A repo's Markdown file isn't a wiki entry, so the notice calls it a file.
  it("names a repo's file as a file", () => {
    expect(text(renderToStaticMarkup(<FrontmatterError error={reason} noun="file" />))).toBe(
      `This file's frontmatter couldn't be read: ${reason}. Fix it, then run wiki check.`,
    );
  });

  it("is marked in the tree, named with the reason", () => {
    const html = renderToStaticMarkup(<UnreadableMark error={reason} />);
    const name = `Frontmatter couldn&#x27;t be read: ${reason}`;
    expect(html).toContain('role="img"');
    expect(html).toContain(`aria-label="${name}"`);
    expect(html).toContain(`title="${name}"`);
  });

  it("is listed above a board's columns, each with its reason", () => {
    const html = renderToStaticMarkup(
      <UnreadNotice
        unread={[
          { path: "backlog/a.md", label: "A", error: reason },
          { path: "backlog/b.md", label: "B", error: "more than 10000 values through aliases" },
        ]}
        onOpen={() => {}}
      />,
    );
    expect(text(html)).toBe(
      "Not on the board: 2 entries whose frontmatter couldn't be read." +
        `A: ${reason}` +
        "B: more than 10000 values through aliases",
    );
    // Each opens, from the keyboard too: a button, not a bare span.
    expect(html.match(/<button type="button"/g)).toHaveLength(2);
  });

  it("counts one entry in the singular", () => {
    const html = renderToStaticMarkup(
      <UnreadNotice unread={[{ path: "a.md", label: "A", error: reason }]} onOpen={() => {}} />,
    );
    expect(text(html)).toContain("Not on the board: 1 entry whose");
  });
});

describe("plural", () => {
  it("takes the plural a noun needs", () => {
    expect(plural(2, "session")).toBe("2 sessions");
    expect(plural(1, "entry", "entries")).toBe("1 entry");
    expect(plural(3, "entry", "entries")).toBe("3 entries");
  });
});
