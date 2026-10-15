import { roster } from "../../app/model";
import type { Seed } from "./seed";

// The demo's files: a team file written out as the server would read it, and
// the line diff a merge request shows (the server's comes from git).

export type TeamConfig = Seed["teams"][number]["config"];

const q = (s: string) => JSON.stringify(s);

/** A team file as TOML, for the File tab (the server shows the real file). */
export function teamToml(c: TeamConfig): string {
  const lines = [`team = ${q(c.team)}`, `name = ${q(c.name)}`];
  if (c.purpose) lines.push(`purpose = ${q(c.purpose)}`);
  lines.push(`lead = ${q(c.lead)}`);
  if (c.playbook.length > 0) {
    lines.push("playbook = [");
    for (const step of c.playbook) {
      for (const [who, text] of Object.entries(step)) lines.push(`  { ${who} = ${q(text)} },`);
    }
    lines.push("]");
  }
  for (const e of c.escalate)
    lines.push("", "[[escalate]]", `when = ${q(e.when)}`, `to = ${q(e.to)}`);
  for (const id of roster(c)) {
    const s = c.agents[id];
    lines.push(
      "",
      `[agents.${id}]`,
      `name = ${q(s.name)}`,
      `role = ${q(s.role)}`,
      `harness = ${q(s.harness)}`,
    );
    if (s.command) lines.push(`command = ${q(s.command)}`);
    if (s.personality) lines.push(`personality = ${q(s.personality)}`);
    if (s.skills.length > 0) lines.push(`skills = [${s.skills.map(q).join(", ")}]`);
    if (s.instruction) lines.push(`instruction = """\n${s.instruction.trim()}\n"""`);
    lines.push("", `[agents.${id}.look]`);
    for (const [k, v] of Object.entries(s.look)) {
      lines.push(`${k === "hairColor" ? "hair_color" : k} = ${q(v)}`);
    }
  }
  return `${lines.join("\n")}\n`;
}

/** A unified-style diff of one file: every line, marked `+` added, `-` removed
 * or ` ` kept (demo files are short, so no hunks). */
export function lineDiff(path: string, before: string, after: string): string {
  const a = before.split("\n");
  const b = after.split("\n");
  // Longest common subsequence, from the end.
  const keep: number[][] = Array.from({ length: a.length + 1 }, () => Array(b.length + 1).fill(0));
  for (let i = a.length - 1; i >= 0; i--)
    for (let j = b.length - 1; j >= 0; j--)
      keep[i][j] =
        a[i] === b[j] ? keep[i + 1][j + 1] + 1 : Math.max(keep[i + 1][j], keep[i][j + 1]);
  const out = [`--- a/${path}`, `+++ b/${path}`];
  let i = 0;
  let j = 0;
  while (i < a.length || j < b.length) {
    if (i < a.length && j < b.length && a[i] === b[j]) {
      out.push(` ${a[i++]}`);
      j++;
    } else if (j < b.length && (i === a.length || keep[i][j + 1] >= keep[i + 1][j])) {
      out.push(`+${b[j++]}`);
    } else {
      out.push(`-${a[i++]}`);
    }
  }
  return out.join("\n");
}
