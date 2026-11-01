import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Avatar, dotSize, radius, ringWidth, statusColor } from "./Avatar";

const html = (el: React.ReactElement) => renderToStaticMarkup(el);

describe("Avatar", () => {
  it("draws the traits it is given, and only those", () => {
    const bald = html(<Avatar look={{ hair: "bald", eyes: "dots", beard: "none" }} />);
    expect(bald).not.toContain("M8.4 15.2"); // no hair top
    expect(bald).not.toContain("M8.7 17.5"); // no beard
    const bun = html(<Avatar look={{ hair: "bun", beard: "full", eyes: "shades" }} />);
    expect(bun).toContain('cx="16" cy="6.2" r="3"'); // the bun
    expect(bun).toContain("M8.4 15.2"); // and the hair top under it
    expect(bun).toContain("M8.7 17.5"); // the full beard
    expect(bun).toContain('width="5.4"'); // shades
    const stubble = html(<Avatar look={{ beard: "stubble" }} />);
    expect(stubble).toContain('fill-opacity="0.32"');
    expect(html(<Avatar look={{ hair: "curly" }} />).match(/<circle/g)?.length).toBe(5 + 2);
  });

  it("falls back to the defaults for missing or empty traits", () => {
    const out = html(<Avatar look={{ color: "", skin: "#F1C9A5" }} />);
    expect(out).toContain('fill="#8E9AAF"'); // default background
    expect(out).toContain('fill="#F1C9A5"');
  });

  it("follows the size rules", () => {
    expect([radius(32), radius(24), radius(72)]).toEqual([8, 6, 17]);
    expect([dotSize(20), dotSize(32), dotSize(72)]).toEqual([7, 10, 23]);
    expect([ringWidth(20), ringWidth(32), ringWidth(72)]).toEqual([1.5, 2.1, 4.8]);
  });

  it("shows the status dot, dashed when queued, and the ring", () => {
    expect(html(<Avatar status="running" />)).toContain("background:var(--ok)");
    expect(html(<Avatar status="queued" />)).toContain("dashed");
    expect(html(<Avatar ring="var(--acc)" />)).toContain("inset 0 0 0 2.1px var(--acc)");
    expect(html(<Avatar />)).not.toContain("right:-3px");
  });

  it("maps every status to its color", () => {
    expect(
      ["running", "awaiting", "idle", "done", "starting", "stopped", "queued"].map(statusColor),
    ).toEqual([
      "var(--ok)",
      "var(--attn)",
      "var(--tx4)",
      "var(--info)",
      "var(--team)",
      "var(--err)",
      "var(--tx4)",
    ]);
  });
});
