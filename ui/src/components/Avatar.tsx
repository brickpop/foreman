import type { Look } from "../api/types/home";

// An agent's generated face (wiki/product/design/avatar.md): a 32×32 SVG drawn
// from its soul's look, with an optional status dot and "needs you" ring.

const defaults: Look = {
  color: "#8E9AAF",
  skin: "#E2AE85",
  hair: "short",
  hairColor: "#2B2420",
  eyes: "dots",
  beard: "none",
};

const ink = "#1A1A1A";
const beardPath =
  "M8.7 17.5 C9.2 24.5 12.8 26.6 16 26.6 C19.2 26.6 22.8 24.5 23.3 17.5 C22.2 21.6 19.6 23.2 16 23.2 C12.4 23.2 9.8 21.6 8.7 17.5 Z";

export function radius(size: number): number {
  return Math.round(size * 0.24);
}

export function dotSize(size: number): number {
  return Math.max(7, Math.round(size * 0.32));
}

export function ringWidth(size: number): number {
  return Math.round(Math.max(1.5, size / 15) * 10) / 10;
}

type Props = {
  look?: Partial<Look>;
  size?: number;
  /** The instance's status, drawn as the corner dot. */
  status?: string;
  /** Must match the surface behind the avatar. */
  dotBorder?: string;
  /** The "needs you" highlight. */
  ring?: string;
  title?: string;
};

export function Avatar({ look, size = 32, status, dotBorder = "var(--bg)", ring, title }: Props) {
  // Empty strings (an unset trait in a hand-written team file) fall back too.
  const f = { ...defaults };
  for (const [k, v] of Object.entries(look ?? {})) {
    if (v) f[k as keyof Look] = v;
  }
  const r = radius(size);
  return (
    <span
      className="relative inline-block shrink-0"
      style={{ width: size, height: size }}
      title={title}
    >
      <svg
        viewBox="0 0 32 32"
        width={size}
        height={size}
        style={{ borderRadius: r, overflow: "hidden", display: "block" }}
        aria-hidden="true"
      >
        <rect width="32" height="32" fill={f.color} />
        {f.hair === "long" && (
          <path d="M7 16 C7 5.5 25 5.5 25 16 L25.6 27 L6.4 27 Z" fill={f.hairColor} />
        )}
        <path d="M4.5 33 C4.5 25.6 27.5 25.6 27.5 33 Z" fill="rgba(20,20,20,0.28)" />
        <ellipse cx="16" cy="16.6" rx="7.4" ry="8.4" fill={f.skin} />
        {(f.hair === "short" || f.hair === "long" || f.hair === "bun") && (
          <path
            d="M8.4 15.2 C7.6 6.8 24.4 6.8 23.6 15.2 C22.6 11.6 19.2 10.6 16 10.6 C12.8 10.6 9.4 11.6 8.4 15.2 Z"
            fill={f.hairColor}
          />
        )}
        {f.hair === "bun" && <circle cx="16" cy="6.2" r="3" fill={f.hairColor} />}
        {f.hair === "curly" &&
          [
            [9.6, 12.6, 2.6],
            [12, 9.6, 2.9],
            [16, 8.4, 3.1],
            [20, 9.6, 2.9],
            [22.4, 12.6, 2.6],
          ].map(([cx, cy, cr]) => <circle key={cx} cx={cx} cy={cy} r={cr} fill={f.hairColor} />)}
        {f.beard === "full" && <path d={beardPath} fill={f.hairColor} />}
        {f.beard === "stubble" && <path d={beardPath} fill={f.hairColor} fillOpacity={0.32} />}
        <Eyes kind={f.eyes} />
        {f.beard === "mustache" && (
          <path
            d="M13.4 20.3 C14.6 19.2 15.6 19.7 16 20.1 C16.4 19.7 17.4 19.2 18.6 20.3 C17.4 21 16.6 20.7 16 20.5 C15.4 20.7 14.6 21 13.4 20.3 Z"
            fill={f.hairColor}
          />
        )}
        <path
          d="M14.3 21.4 Q16 22.7 17.7 21.4"
          stroke="rgba(40,20,10,0.65)"
          strokeWidth="0.9"
          fill="none"
        />
      </svg>
      {ring && (
        <span
          className="absolute inset-0"
          style={{ borderRadius: r, boxShadow: `inset 0 0 0 ${ringWidth(size)}px ${ring}` }}
        />
      )}
      {status && (
        <span
          className="absolute rounded-full"
          style={{
            right: -3,
            bottom: -3,
            width: dotSize(size),
            height: dotSize(size),
            ...dotStyle(status),
            boxShadow: `0 0 0 2px ${dotBorder}`,
          }}
        />
      )}
    </span>
  );
}

function Eyes({ kind }: { kind: string }) {
  const dots = (
    <>
      <circle cx="13" cy="16.4" r="1.1" fill={ink} />
      <circle cx="19" cy="16.4" r="1.1" fill={ink} />
    </>
  );
  switch (kind) {
    case "glasses":
      return (
        <>
          {dots}
          {[13, 19].map((cx) => (
            <circle
              key={cx}
              cx={cx}
              cy="16.4"
              r="2.6"
              fill="rgba(255,255,255,0.18)"
              stroke={ink}
              strokeWidth="0.9"
            />
          ))}
          <path d="M15.6 16.2 L16.4 16.2" stroke={ink} strokeWidth="0.9" />
        </>
      );
    case "shades":
      return (
        <>
          <rect x="10.1" y="14.9" width="5.4" height="3" rx="1.3" fill={ink} />
          <rect x="16.5" y="14.9" width="5.4" height="3" rx="1.3" fill={ink} />
          <rect x="15.4" y="15.6" width="1.2" height="0.7" fill={ink} />
        </>
      );
    case "sleepy":
      return (
        <>
          {["M11.7 16.5 Q13 17.4 14.3 16.5", "M17.7 16.5 Q19 17.4 20.3 16.5"].map((d) => (
            <path key={d} d={d} stroke={ink} strokeWidth="0.9" strokeLinecap="round" fill="none" />
          ))}
        </>
      );
    default:
      return dots;
  }
}

/** The status color of an instance (wiki/product/design/design-system.md#status-colors). */
export function statusColor(status: string): string {
  switch (status) {
    case "running":
      return "var(--ok)";
    case "awaiting":
      return "var(--attn)";
    case "done":
      return "var(--info)";
    case "starting":
      return "var(--team)";
    case "stopped":
      return "var(--err)";
    default:
      return "var(--tx4)"; // idle, queued, asleep
  }
}

/** A status dot's fill: queued is a dashed outline and asleep a plain one, so they
 * read differently from idle. */
export function dotStyle(status: string): { background: string; border?: string } {
  if (status === "queued") return { background: "var(--bg)", border: "1.5px dashed var(--tx4)" };
  if (status === "asleep") return { background: "var(--bg)", border: "1.5px solid var(--tx4)" };
  return { background: statusColor(status) };
}
