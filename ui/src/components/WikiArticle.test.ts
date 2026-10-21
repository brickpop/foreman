import { describe, expect, it } from "vitest";
import { chipsOf } from "./WikiArticle";

describe("chipsOf", () => {
  it("takes the title out of the chips: it is the heading", () => {
    expect(chipsOf({ type: "task", title: "Export CSV", status: "todo" })).toEqual({
      title: "Export CSV",
      fields: [
        ["type", "task"],
        ["status", "todo"],
      ],
    });
  });

  it("has no heading for an empty or missing title, and hides empty fields", () => {
    expect(chipsOf({ type: "note", title: "", assignee: "" })).toEqual({
      title: undefined,
      fields: [["type", "note"]],
    });
    expect(chipsOf({}).title).toBeUndefined();
  });
});
