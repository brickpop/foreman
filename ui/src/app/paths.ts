// Wiki paths: entries link to each other relative to the linking entry.

/** Resolves href against the entry at from (both relative to the wiki). */
export function resolveEntry(from: string, href: string): string {
  const base = from.split("/").slice(0, -1);
  for (const part of href.split("#")[0].split("/")) {
    if (part === "..") base.pop();
    else if (part !== "." && part !== "") base.push(part);
  }
  return base.join("/");
}

/** Whether a link leaves the wiki (opens in a new tab). */
export function isExternal(href: string): boolean {
  return /^[a-z][a-z0-9+.-]*:/i.test(href) || href.startsWith("//");
}
