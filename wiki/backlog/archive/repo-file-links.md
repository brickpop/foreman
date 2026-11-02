---
type: task
status: done
priority: high
tags: [feature, ui]
epic: workflow
---

# Repos: links in a rendered Markdown file open their target

**Why:** in a repo's rendered Markdown, `[Deploy your first DAO](./deploy-a-dao.md)` is plain text even with `deploy-a-dao.md` right beside it (the CEO, 2026-11-03, on `protocol-doc/guides/install-a-plugin.md`). `FileMarkdown` (`ui/src/components/Markdown.tsx`) keeps only `http(s)` links, a rule from before the Repos section opened files by URL (`?path=`).

**Decided (the CEO, 2026-11-03),** written into [Repos → The file](../../product/features/repos.md):
- A relative link resolves against the file's folder; a root-absolute one (`/x.md`) against the wiki root, the folder holding the nearest `wiki.toml` above the file, else the repo root. Same repo, same branch, opened like a click in the tree (`?path=`, a step Back returns from).
- A folder opens its `index.md`, else its `README.md`, else a placeholder listing what it holds.
- `#anchor` scrolls to that heading, on this file or the one opened (`FileMarkdown` sets no heading ids today).
- A path the branch doesn't have is marked missing, as the wiki reader marks it (`unwritten`).
- The repo's own images load, under the wiki image endpoint's rules (`internal/server/wiki.go`: image extensions only, `nosniff`, an SVG under `svgPolicy`). Anything else stays unloaded.
- Out: a link climbing out of the repo, into another repo of the project, or into the project's wiki stays text.

**Approach (built 2026-11-03):** resolve on the server, as the wiki does, so the browser looks links up and never re-derives them: `…/repos/:repo/file` sends the Markdown file's links resolved (target path, folder or file, exists, image), and the one `Markdown` renderer serves both readers, retiring `FileMarkdown`. Images from `…/repos/:repo/image?ref=&path=`, read at the ref through the Git block, with a size cap.

- [x] link resolution on the server (`internal/repos/links.go`, reading each folder once per file; `parse.Links` and `wiki.Headings`, the engine's own rules) (relative, root-absolute via the nearest `wiki.toml`, `..` refused past the repo root, `#anchor` and `?query` apart, percent-decoding), checked against the ref's tree
- [x] the image endpoint, the wiki's rules, a size cap
- [x] the folder view: `index.md`, `README.md`, else the listing placeholder
- [x] heading ids and anchor scrolling, in the file and across files
- [x] one Markdown renderer for wiki and repo files; `FileMarkdown` removed (`Markdown` takes body and tables, `scope` wiki or repo; a lookup also tries the href decoded, as the renderer re-encodes `<my file.md>`)
- [x] tests (`internal/repos/links_test.go`, `internal/server/repos_test.go`, `Markdown.test.tsx`, `repos.test.ts` folderPage, `client.test.ts`): `./x.md`, `x.md`, `../x.md`, `/x.md` with and without a `wiki.toml` above, `./dir/` (each fallback), `x.md#part`, `#part`, a missing target, a path escaping the repo, `my%20file.md`, a `javascript:` href inert, a web link in a new tab; images: an allowed type loads, a non-image path and a path outside the repo are `404`, an SVG carries the policy
- [x] [Repos](../../product/features/repos.md) and [API](../../product/architecture/api.md) as built
