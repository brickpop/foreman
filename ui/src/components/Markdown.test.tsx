import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { Entry } from "../api/client";
import { Markdown, type ResolvedLink } from "./Markdown";

// As the server sends it: the body as on disk, with the engine's lookup tables.
const entry = (body: string, tables: Partial<Entry> = {}): Entry => ({
  path: "plan/a.md",
  title: "A",
  version: "v1",
  frontmatter: [],
  body,
  links: [],
  headings: [],
  checkboxes: [],
  backlinks: [],
  ...tables,
});

// Fits a wiki entry's links and a repo file's (folder).
type AnyLink = Entry["links"][number] & { folder?: boolean };
const link = (raw: string, more: Partial<AnyLink>): AnyLink => ({
  raw,
  to: "",
  anchor: "",
  exists: false,
  file: false,
  image: false,
  outside: false,
  ...more,
});

const html = (e: Entry) =>
  renderToStaticMarkup(
    <Markdown
      body={e.body}
      links={e.links}
      headings={e.headings}
      checkboxes={e.checkboxes}
      onOpen={() => {}}
      onTick={() => {}}
      imageUrl={(p) => `/img/${p}`}
    />,
  );

describe("Markdown", () => {
  it("gives headings the engine's ids, matched by line", () => {
    const out = html(
      entry("# A\n\n## Goals\n\n## Goals\n", {
        headings: [
          { id: "a", bodyLine: 1 },
          { id: "goals", bodyLine: 3 },
          { id: "goals-1", bodyLine: 5 },
        ],
      }),
    );
    expect(out).toContain('<h2 id="goals">Goals</h2>');
    expect(out).toContain('<h2 id="goals-1">Goals</h2>');
  });

  it("ticks checkboxes from the entry's table, not from the Markdown", () => {
    const out = html(
      entry("- [ ] first\n- [x] second\n- plain\n", {
        checkboxes: [
          { line: 4, bodyLine: 1, done: true, text: "first" },
          { line: 5, bodyLine: 2, done: false, text: "second" },
        ],
      }),
    );
    expect(out).toContain('<input type="checkbox" aria-label="first" checked=""/>');
    expect(out).toContain('<input type="checkbox" aria-label="second"/>');
    // remark-gfm's own disabled box is dropped: one box per item.
    expect(out.match(/<input/g)).toHaveLength(2);
    expect(out).not.toContain("disabled");
    expect(out).toContain("<li>plain</li>");
  });

  it("resolves links by lookup, and shows what they point at", () => {
    const out = html(
      entry(
        "[home](../index.md) [later](todo.md) [code](../../x.go) [png](d.png) [pdf](d.pdf) " +
          "[web](https://example.com) [here](#goals)\n",
        {
          links: [
            link("../index.md", { to: "index.md", exists: true }),
            link("todo.md", { to: "plan/todo.md" }),
            link("../../x.go", { outside: true }),
            link("d.png", { to: "plan/d.png", file: true, image: true }),
            link("d.pdf", { to: "plan/d.pdf", file: true }),
          ],
        },
      ),
    );
    expect(out).toContain('<a href="index.md">home</a>');
    expect(out).toContain('<a href="plan/todo.md" class="unwritten" title="Not written yet">');
    expect(out).toContain(
      '<span class="outside" title="../../x.go is outside the wiki">code</span>',
    );
    expect(out).toContain(
      '<a href="/img/plan/d.png" target="_blank" rel="noopener noreferrer">png</a>',
    );
    expect(out).toContain(
      '<span class="outside" title="Foreman doesn&#x27;t show plan/d.pdf">pdf</span>',
    );
    expect(out).toContain(
      '<a href="https://example.com" target="_blank" rel="noopener noreferrer">web</a>',
    );
    expect(out).toContain('<a href="#goals">here</a>');
  });

  it("loads the wiki's own images only", () => {
    const out = html(
      entry("![shot](img/s.png) ![tracker](https://evil.example/t.png) ![gone](img/no.png)\n", {
        links: [
          link("img/s.png", { to: "plan/img/s.png", file: true, image: true }),
          link("img/no.png", { to: "plan/img/no.png" }),
        ],
      }),
    );
    expect(out).toContain('<img src="/img/plan/img/s.png" alt="shot" loading="lazy"/>');
    expect(out.match(/<img/g)).toHaveLength(1);
    expect(out).toContain('<a href="https://evil.example/t.png" target="_blank"');
    expect(out).toContain('<span class="unwritten">gone</span>');
  });

  it("renders no raw HTML and no script URLs", () => {
    const out = html(
      entry(
        '<script>alert(1)</script>\n\n<img src="x" onerror="alert(2)">\n\n[x](javascript:alert(3))\n',
      ),
    );
    // Raw HTML shows as text; a script URL is emptied, and the link is text.
    expect(out).toContain("&lt;script&gt;alert(1)&lt;/script&gt;");
    expect(out).not.toMatch(/<script|<img|javascript:/);
    expect(out).toContain("<p><span>x</span></p>");
  });

  it("labels callouts, any word being a type", () => {
    const out = html(entry("> [!warning] Careful\n> Text\n\n> [!tldr]\n> Short\n\n> plain\n"));
    expect(out).toContain(
      '<blockquote class="callout" data-callout="warning"><p class="callout-label">Careful</p>',
    );
    expect(out).toContain('data-callout="tldr"><p class="callout-label">Tldr</p>');
    expect(out).toContain("<blockquote>\n<p>plain</p>");
  });

  it("wraps tables so a wide one scrolls on its own", () => {
    expect(html(entry("| a | b |\n|---|---|\n| 1 | 2 |\n"))).toContain(
      '<div class="table"><table>',
    );
  });
});

// A repo's file: its links resolved against the branch, nothing to tick.
describe("Markdown, for a repo file", () => {
  const repo = (body: string, links: ResolvedLink[]) =>
    renderToStaticMarkup(
      <Markdown
        body={body}
        links={links}
        headings={[]}
        onOpen={() => {}}
        imageUrl={(p) => `/img/${p}`}
        scope="repo"
      />,
    );

  it("links the repo's files and folders, marks a missing one, keeps an escape as text", () => {
    const got = repo(
      "[doc](./a.md) [dir](sub/) [gone](./b.md) [out](../../x.md) [site](https://x.dev) [js](javascript:alert(1))",
      [
        link("./a.md", { to: "docs/a.md", exists: true }),
        link("sub/", { to: "docs/sub", exists: true, folder: true }),
        link("./b.md", { to: "docs/b.md" }),
        link("../../x.md", { outside: true }),
      ],
    );
    expect(got).toContain('<a href="docs/a.md">doc</a>');
    expect(got).toContain('<a href="docs/sub">dir</a>');
    expect(got).toContain(
      '<a href="docs/b.md" class="unwritten" title="Not on this branch">gone</a>',
    );
    expect(got).toContain(
      '<span class="outside" title="../../x.md is outside the repo">out</span>',
    );
    expect(got).toContain(
      '<a href="https://x.dev" target="_blank" rel="noopener noreferrer">site</a>',
    );
    expect(got).not.toContain("javascript:");
  });

  it("finds a link the renderer re-encoded (a name with a space)", () => {
    const got = repo("[spaced](<my file.md>)", [
      link("my file.md", { to: "docs/my file.md", exists: true }),
    ]);
    expect(got).toContain(">spaced</a>");
    expect(got).not.toContain("unwritten");
    expect(got).not.toContain("_blank");
  });

  it("loads the repo's own images only, and never an image elsewhere", () => {
    const got = repo("![logo](../logo.png) ![far](https://t.dev/p.png) ![gone](x.png)", [
      link("../logo.png", { to: "logo.png", exists: true, image: true }),
      link("x.png", { to: "docs/x.png" }),
    ]);
    expect(got).toContain('<img src="/img/logo.png" alt="logo" loading="lazy"/>');
    expect(got.match(/<img/g)).toHaveLength(1);
  });

  it("leaves a task item as text: a repo file isn't ticked here", () => {
    expect(repo("- [ ] todo", [])).toContain('disabled=""');
  });
});
