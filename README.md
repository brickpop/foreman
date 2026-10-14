# Foreman

Run several teams of AI coding agents across several projects on your own machine, from one web UI. What Foreman is and how it works lives in the [wiki](./wiki/index.md); start at [the product map](./wiki/product/index.md).

> Status: walking skeleton. The server starts, serves the UI behind its access token, and nothing else yet. See the [backlog](./wiki/backlog/index.md).

## Prerequisites

| Tool | For | Install |
|---|---|---|
| [Go](https://go.dev/dl/) (current stable) | the server | `brew install go` · `sudo dnf install golang` |
| [Bun](https://bun.sh) | building the UI (Bun only: never npm, yarn or node) | `curl -fsSL https://bun.sh/install \| bash` |
| [just](https://just.systems) | every command | `brew install just` · `sudo dnf install just` |
| [wiki](https://github.com/agentic-wiki/wiki) | the knowledge base | `brew install agentic-wiki/tap/wiki` |

Podman will be needed once agents run (it isn't yet).

## Setup and run

```sh
just setup                # install the UI's dependencies (once, and after pulling)
just build                # one binary, with the UI embedded: bin/foreman
bin/foreman ~/agents-ws   # serve a workspace (or run it inside the folder)
```

The first time, Foreman asks before creating `.foreman/` in the folder. It then prints a URL with an access token and opens it in your browser. Use that URL: without the token every request is refused. `bin/foreman -h` lists the flags.

## Develop

```sh
just dev       # rebuild the UI on change and serve a scratch workspace (.dev/workspace)
just test      # all tests
just lint      # gofmt, go vet, staticcheck, Biome
just check     # wiki health check
just help      # everything else
```

How the work is organised (specs, backlog, conventions) is in [AGENTS.md](./AGENTS.md) and [WORKFLOW.md](./WORKFLOW.md).
