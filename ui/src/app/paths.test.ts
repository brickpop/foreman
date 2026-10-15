import { describe, expect, it } from "vitest";
import { isExternal, resolveEntry } from "./paths";

describe("wiki paths", () => {
  it("resolves links relative to the linking entry", () => {
    expect(resolveEntry("index.md", "backlog/LED-41.md")).toBe("backlog/LED-41.md");
    expect(resolveEntry("adr/014-settlement-v2.md", "../backlog/LED-41.md")).toBe(
      "backlog/LED-41.md",
    );
    expect(resolveEntry("a/b/c.md", "./d.md#part")).toBe("a/b/d.md");
    expect(resolveEntry("index.md", "../../escape.md")).toBe("escape.md");
  });

  it("tells external links apart", () => {
    expect(isExternal("https://example.com")).toBe(true);
    expect(isExternal("mailto:x@y.z")).toBe(true);
    expect(isExternal("//cdn.example.com/x")).toBe(true);
    expect(isExternal("backlog/LED-41.md")).toBe(false);
  });
});
