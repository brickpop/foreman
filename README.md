# Foreman

Run several teams of AI coding agents across several projects on your own machine, from one web UI. What Foreman is and how it works lives in the [wiki](./wiki/index.md); start at [the product map](./wiki/product/index.md).

> Status: in development. Teams deploy and run Claude Code agents in Podman, which talk to each other and work through git. See the [backlog](./wiki/backlog/index.md).

## Install

Foreman is one binary with everything it needs inside: the web UI, the agent image's recipe, and the helper its agents use, for Apple silicon and for Intel or AMD machines. Beside it you need:

- [Podman](https://podman.io/docs/installation), where the agents run: `brew install podman` on macOS, `sudo dnf install podman` on Fedora. Foreman starts without it, and its setup checklist on the Dashboard walks you through the rest (on macOS, creating the Podman machine, sized for your agents).
- A Claude Code login, made once from Foreman (Settings → Agents → Profiles, or the setup checklist on the Dashboard) and shared by all your projects. Foreman keeps its own data in its home, `~/.foreman/`, on macOS too: the global repo `config/` (your teams and `settings.toml`, in git), the agents' harness folders (their config and conversations), the profiles and their logins, and its state. Each project's files are in its workspace (below). Claude Code itself runs in the agents' image: nothing to install on your machine.

There are no published binaries yet: build one from source as below.

## Build from source

| Tool | For | Install |
|---|---|---|
| [Go](https://go.dev/dl/) (current stable) | the server | `brew install go` · `sudo dnf install golang` |
| [Bun](https://bun.sh) | building the UI (Bun only: never npm, yarn or node) | `curl -fsSL https://bun.sh/install \| bash` |
| [just](https://just.systems) | every command | `brew install just` · `sudo dnf install just` |
| [wiki](https://github.com/agentic-wiki/wiki) | the knowledge base | `brew install agentic-wiki/tap/wiki` |

The build machine needs no Podman: Go cross-compiles the agents' helper for both CPU types.

## Run

```sh
just setup                # install the UI's dependencies (once, and after pulling)
just build                # one binary, everything embedded: bin/foreman (copy it anywhere)
bin/foreman               # serve, from ~/.foreman (one Foreman per machine)
```

Foreman prints a URL with an access token and opens it in your browser. Use that URL: without the token every request is refused. `bin/foreman -h` lists the flags. Run it again while it runs and it prints that URL.

A project is a set of git repos anywhere in your home folder, plus a **workspace**, a folder of yours (`~/workspace/foreman/<name>/` by default, set in Settings → System) holding its project repo (`project/`: its wiki and `project.toml`) and its agents' worktrees (`agents/<agent>/`), where you can look at their work and run it. Agents never work in your checkouts: each gets its own git worktree of each repo, and work reaches `main` only when you merge it. A project made before workspaces lives in `~/.foreman/` until you move it from its Settings section.

Ctrl-C stops Foreman and its agents: they go to sleep, and wake with their conversation when work arrives on the next run (a second Ctrl-C leaves them running instead).

## Develop

```sh
just dev       # rebuild the UI on change and serve a scratch home (.dev/real; its repo in .dev/real-repos) with a sample team and project, agents on Claude Code (Podman; log in under Settings → Agents → Profiles)
just dev fake  # the same on the fake harness, a stand-in that echoes (.dev/fake; no Podman, no login, no tokens)
just test      # all tests
just lint      # gofmt, go vet, staticcheck, Biome
just check     # wiki health check
just help      # everything else
```

How the work is organised (specs, backlog, conventions) is in [AGENTS.md](./AGENTS.md) and [WORKFLOW.md](./WORKFLOW.md).
