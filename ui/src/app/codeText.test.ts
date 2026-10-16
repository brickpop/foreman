import { describe, expect, it } from "vitest";
import { codePieces } from "./codeText";

describe("codePieces", () => {
  it("tells inline code from prose", () => {
    expect(codePieces("Run `go test ./...` then `cal 2 2024`.")).toEqual([
      { kind: "text", text: "Run " },
      { kind: "code", text: "go test ./..." },
      { kind: "text", text: " then " },
      { kind: "code", text: "cal 2 2024" },
      { kind: "text", text: "." },
    ]);
  });
  it("keeps a fenced block whole, its language line dropped", () => {
    expect(codePieces("Output:\n```text\n   October 2026\nSu Mo\n```\nDone.")).toEqual([
      { kind: "text", text: "Output:\n" },
      { kind: "block", text: "   October 2026\nSu Mo" },
      { kind: "text", text: "\nDone." },
    ]);
  });
  it("leaves unclosed backticks and plain text alone", () => {
    expect(codePieces("a ` b")).toEqual([{ kind: "text", text: "a ` b" }]);
    expect(codePieces("```\nnever closed")).toEqual([{ kind: "text", text: "```\nnever closed" }]);
    expect(codePieces("")).toEqual([]);
  });
});
