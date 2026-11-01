import { describe, expect, it } from "vitest";
import type { Field } from "../api/client";
import { summary, tagColor, withoutTitle } from "./frontmatter";

const field = (key: string, ...values: (string | { text: string; to: string; title: string })[]) =>
  ({
    key,
    values: values.map((v) => (typeof v === "string" ? { text: v, to: "", title: "" } : v)),
  }) satisfies Field;

const ref = (text: string, title: string) => ({ text, to: text.replace(/^\.\.\//, ""), title });

describe("withoutTitle", () => {
  it("takes the title out: it is the heading", () => {
    const fields = [field("status", "todo"), field("title", "Export CSV"), field("type", "task")];
    expect(withoutTitle(fields)).toEqual({
      title: "Export CSV",
      fields: [field("status", "todo"), field("type", "task")],
    });
  });

  it("has no heading for an empty or missing title, and drops the empty field", () => {
    expect(withoutTitle([field("title"), field("type", "note")])).toEqual({
      title: undefined,
      fields: [field("type", "note")],
    });
    expect(withoutTitle([field("title", "")]).title).toBeUndefined();
  });
});

describe("summary", () => {
  it("joins the values with no keys, skips empty fields, shows titles for entries", () => {
    expect(
      summary([
        field("assignee"),
        field("parent", ref("../epic.md", "The epic")),
        field("priority", "high", "now"),
        field("tags", "ui", "git"),
      ]),
    ).toEqual([
      { key: "parent", text: "The epic" },
      { key: "priority", text: "high, now" },
      { key: "tags", text: "", tags: ["ui", "git"] },
    ]);
  });

  it("counts a field naming several entries, so the line stays short", () => {
    expect(summary([field("blocked_by", ref("a.md", "A"), ref("b.md", "B"))])).toEqual([
      { key: "blocked_by", text: "2 blocked_by" },
    ]);
    // Mixed with plain text, it lists them: some aren't entries to count.
    expect(summary([field("blocked_by", ref("a.md", "A"), "gone.md")])[0].text).toBe("A, gone.md");
  });
});

describe("tagColor", () => {
  it("gives a tag one colour everywhere, and different tags different ones", () => {
    expect(tagColor("ui")).toBe(tagColor("ui"));
    const colors = new Set(["ui", "git", "bug", "feature", "model", "api"].map(tagColor));
    expect(colors.size).toBeGreaterThan(1);
  });
});
