# Foreman

Run several teams of AI coding agents across several projects on your own machine, from one web UI. What Foreman is and how it works lives in the [wiki](./wiki/index.md); start at [the product map](./wiki/product/index.md).

> Status: in development. Teams deploy and run Claude Code agents in Podman, which talk to each other and work through git. See the [backlog](./wiki/backlog/index.md).

## Install

Foreman is one binary with everything it needs inside: the web UI, the agent image's recipe, and the helper its agents use, for Apple silicon and for Intel or AMD machines. Beside it you need:

- [Podman](https://podman.io/docs/installation), where the agents run: `brew install podman` on macOS, `sudo dnf install podman` on Fedora. Foreman starts without it, and its setup checklist on Home walks you through the rest (on macOS, creating the Podman machine, sized for your agents).
- A Claude Code login, made once from Foreman (Settings → Global → Profiles, or the setup checklist on Home) and shared by all your workspaces. Foreman keeps what is yours on this machine (profiles and their logins, the theme, the agent image's build state) in `~/.local/share/foreman/`, on macOS too. Claude Code itself runs in the agents' image: nothing to install on your machine.

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
bin/foreman ~/agents-ws   # serve a workspace (or run it inside the folder)
```

The first time, Foreman asks before creating `.foreman/` in the folder. It then prints a URL with an access token and opens it in your browser. Use that URL: without the token every request is refused. `bin/foreman -h` lists the flags.

Ctrl-C stops Foreman and its agents: they go to sleep, and wake with their conversation when work arrives on the next run (a second Ctrl-C leaves them running instead).

## Develop

```sh
just dev       # rebuild the UI on change and serve a scratch workspace (.dev/real) with a sample team and project, agents on Claude Code (Podman; log in under Settings → Global → Profiles)
just dev fake  # the same on the fake harness, a stand-in that echoes (.dev/fake; no Podman, no login, no tokens)
just test      # all tests
just lint      # gofmt, go vet, staticcheck, Biome
just check     # wiki health check
just help      # everything else
```

How the work is organised (specs, backlog, conventions) is in [AGENTS.md](./AGENTS.md) and [WORKFLOW.md](./WORKFLOW.md).
