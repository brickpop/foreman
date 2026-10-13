---
type: spec
status: planned
tags: [architecture, runtime, sessions, security]
---

# Session runner

Where and how a harness process actually runs. The session supervisor ([Agent runtime](./agent-runtime.md)) never starts processes itself: it goes through a small **Runner** interface, so the execution backend is swappable without touching anything else.

**Decided:** every agent instance runs in a **rootless Podman container from day one**. A local-PTY runner exists for hermetic tests and development.

## The Runner interface

Conceptually (exact Go signatures are an implementation detail):

| Operation | Does |
|---|---|
| `Start(spec) → handle` | Starts a session from a spec: `{name, image, mounts, env, cmd, workdir, tty size, labels}`. `labels` carry the instance id so sessions can be found again |
| `Attach(handle) → stream` | Bidirectional byte stream to the session's TTY (stdin in, merged stdout/stderr out). Several attaches over time; the supervisor holds one |
| `Resize(handle, cols, rows)` | Propagates a terminal resize to the TTY |
| `Stop(handle, grace)` | Graceful stop (SIGTERM, then kill after `grace`) |
| `Wait(handle) → exit status` | Blocks until the process exits; returns the exit code (used to raise blocked requests) |
| `List() → []handle` | Running sessions created by Foreman (by label), for [recovery](./agent-runtime.md#recovery) after a Foreman restart |

The supervisor tees every byte read from `Attach` into the instance's session log ([Persistence](./persistence.md)) before fanning it out to WebSocket clients ([API](./api.md#pty-websocket)).

## Podman runner (production)

- Foreman talks to Podman through the **libpod REST API over the Podman socket**, with a plain `net/http` client: no dependency, structured errors, attach as a hijacked stream. The official Go bindings module is avoided (a very large dependency tree), and so is shelling out to the `podman` CLI (to verify in the [walking skeleton](../../backlog/walking-skeleton.md)).
- Each instance is a container named from its instance id, labelled `foreman.instance=<id>`, `foreman.deployment=<id>`.
- **No CPU or memory limits** by default; a workspace setting can add them later if 20+ sessions turn out to starve the laptop.
- **Mounts, and nothing else of the host:**
  - the instance's sandbox clone `<project>/.sandboxes/<agent>/` → read-write, as the working directory, **at the same path as on the host** (as `pclaude` does: harnesses key their session history by path, and paths in output match what the CEO sees)
  - the instance's private harness config directory → read-write ([Credentials](#credentials))
  - the project `wiki/` → read-write only for the scrum master (or the lead when the team has no scrum master), read-only for everyone else ([Sandboxes and git policy](./sandboxes-and-git.md))
  - no `~/.config`, no `~/.ssh`, no keychain, no `.foreman/` (secrets, state, team files), no other sandboxes, no project repos.
- **Harness credentials:** see [Credentials](#credentials). The host's harness config directories are never mounted.
- **TTY:** the harness is the container's main process with a TTY, so the container keeps running when Foreman exits. After a Foreman restart, `List` finds it by label and Foreman re-attaches. Output printed while Foreman was down isn't in its log; that gap is accepted, and the pane shows `ⓘ Foreman restarted, output may be missing` at that point.
- **User mapping:** `--userns keep-id:uid=1000,gid=1000` (the image's non-root user), so files written into the sandbox and wiki are owned by the CEO's user, not a subordinate UID.
- **SELinux:** on hosts where `/sys/fs/selinux/enforce` exists (Fedora, RHEL), bind mounts get the shared `:z` label; elsewhere no suffix. The private `:Z` is not used: several containers mount the same wiki, and `:Z` would relabel it away from the others (learned in `pclaude`).
- **Network:** containers need the network for model APIs. They reach Foreman's team-tools listener at `host.containers.internal` ([Team tools](./team-tools.md#transport)).
- **macOS:** Podman runs inside a VM (`podman machine`). Bind mounts cross the VM boundary (virtiofs), which is slower for heavy file I/O (e.g. `npm install`) and needs the workspace path shared with the machine. Unix sockets don't cross that boundary.
- **Images:** each project's instances run its layer-4 image, which carries the harnesses and the project's toolchain ([Agent images](./tech-stack.md#agent-images)).

### The Podman service

Foreman never spawns `podman` for container or image work: it calls the libpod REST API directly (create, attach, resize, stop, list, build, tag, prune), so images are managed by the binary itself. The API needs Podman's service to be running, and **Foreman makes sure it is**, so that starting `foreman` is the only step:

- **Linux:** Foreman connects to the rootless socket (`$XDG_RUNTIME_DIR/podman/podman.sock`). If nothing answers, it starts `podman system service --time=0` on that socket as a child process, which ends with Foreman. (A CEO who prefers systemd socket activation, `systemctl --user enable --now podman.socket`, gets the same socket and Foreman just uses it.)
- **macOS:** Podman runs in a Linux VM. If the default machine isn't running, Foreman runs `podman machine start` and connects to the API socket the machine exposes. Foreman never stops the machine on exit: other tools may use it, and the containers (the agents' sessions) live in it.
- Podman itself (and on macOS the initial `podman machine init`, sized for 20+ sessions) is a prerequisite the README covers. Foreman checks on start and refuses to run with a clear message if it's missing.
- The state ("Podman 5.x · running", or the machine's state) is shown in [Settings → Maintenance](../features/settings.md#maintenance).

Linking Podman's engine (libpod) into Foreman instead isn't viable: it would make Foreman a container engine, with cgo, conmon, crun and storage drivers, Linux only, and still no answer for macOS, where containers must run in a VM. To verify in the [walking skeleton](../../backlog/walking-skeleton.md).

## Credentials

**One login per harness, done once, shared by every agent.** The CEO logs in with the harness's own interactive login (Claude Code `claude /login`, Pi's and Hermes' equivalents). No tokens to paste, no API keys in v1.

**The login container.** Settings → Harnesses → **`Connect`** ([Settings](../features/settings.md#harnesses)) starts a one-off **login container** from the harness image and shows its terminal in the browser, running the harness's login. The login lands in the harness's **shared login folder**, `~/agents-ws/.foreman/credentials/<harness>/` (0600), owned by Foreman and separate from the CEO's own `~/.claude`. Settings then shows `connected`, and **`Reconnect`** repeats the flow. Because the login happens inside Linux, the host's own login (on macOS, in the Keychain) is never involved and never synced.

**One writable login, shared by every instance of the harness; everything else private.** Every agent container mounts the shared login folder **read-write**, and the instance's private config directory points the harness at the login file inside it (the adapter declares where, e.g. Claude Code's `.credentials.json` within `CLAUDE_CONFIG_DIR`). This is the situation the harnesses already handle on a single machine, with several `claude` sessions sharing one login: when one refreshes the token, it writes the shared file and the others read the new token instead of refreshing with a revoked one. If a harness has no login yet, its instances don't start and a **blocked** [request](../features/requests.md) says "Connect Claude Code in Settings". If the shared login stops working (revoked, or a logout elsewhere), every affected instance raises one **blocked** request: "Reconnect Claude Code in Settings".

To verify in the [walking skeleton](../../backlog/walking-skeleton.md), per harness: (1) concurrent instances really do pick up each other's refreshes from the shared file (12 instances across several refresh cycles); (2) how to point the harness at the shared file so its own writes land there (a bind mount of the file, a symlink, or the whole folder), given how it writes (an atomic rename would replace a symlink or fail on a single-file mount). If sharing doesn't hold for a harness, the fallbacks are a **keeper** (the login container refreshes shortly before expiry and Foreman copies the fresh login to every instance) or one login per instance.

**Private config, sharing nothing but the login.** Each instance gets a **private harness config directory** (Claude Code: `CLAUDE_CONFIG_DIR`) in its sandbox area, holding its own settings, history and memory, and its [session prompt](./session-prompt.md#delivery). The CEO's `~/.claude` is never mounted: an agent could plant a hook or setting there that later runs *on the host*, and would see the CEO's history from every other project. Forge tokens (`GH_TOKEN` and the like) are **never** injected: only Foreman's push path uses them ([Security](./security.md)).

**Git config.** If the CEO has a `~/.gitconfig`, it is mounted **read-only** into every agent container (nothing is mounted when it doesn't exist), so aliases and preferences behave as on the host. Foreman also sets `GIT_AUTHOR_NAME` / `GIT_AUTHOR_EMAIL` / `GIT_COMMITTER_NAME` / `GIT_COMMITTER_EMAIL` to the agent's display name and a per-agent address; the environment overrides the file's `user.*`, so every commit still says which agent made it. Credential helpers or `include` paths in the file that point at host programs or files simply don't resolve inside the container; agents never push to a forge anyway.

## Local PTY runner (tests and development)

- Spawns the command directly on the host in a pseudo-terminal (the approach of the original PTY proof of concept).
- Used with a **fake scripted harness** (a small program that prints scripted output, calls team tools and reacts to stdin) so the supervisor, team-tools router, request engine and pipeline can be tested end-to-end, fast and without Podman or a model.
- Gives no isolation, so it is **test and development only**: it is never selectable for real agents in v1.
- Sessions die with Foreman, so `List` returns nothing after a restart.

A tmux-based runner (sessions survive restarts, attachable from a real terminal) could be a third implementation later. Not planned.

Images are built by Foreman as a four-layer chain; see [Tech stack](./tech-stack.md#agent-images).

## Prior art: the two proofs of concept

Two Go sketches written before this design, kept verbatim as reference at the end of this section (a PTY session and a Podman session). They are not production code: neither compiles as written (a mangled import path), and their approach to I/O is replaced by this design. The third piece of prior art is [`pclaude`](https://github.com/brickpop/pclaude), the CEO's launcher that runs Claude Code in Podman; its credential handling is the basis of [Credentials](#credentials). What the sketches established, and what this design changes:

| Kept | Changed |
|---|---|
| `creack/pty`: `pty.Start(cmd)` plus `pty.Setsize` with a stable baseline (120×40) before the first output, so the TUI lays out predictably | The sketch read the PTY in request/response chunks, with a 300ms read deadline and a substring match on `"Claude Code >"` to guess the end of a turn. Here the PTY is a continuous byte stream fanned out to WebSocket viewers and the log, and turn detection comes from harness signals ([Harness adapters](./harness-adapters.md)) |
| Running the harness **interactively, in a real TTY** | - |
| Podman container spec: `Terminal: true` (a real TTY in the container), the harness as entrypoint, the workspace bind-mounted rw with the `z` option (SELinux relabelling, needed on Fedora and other SELinux hosts) | The sketch mounted the host's harness config dir read-only to reuse its login. That leaks every credential in it into every container and doesn't work on macOS (the Keychain); each instance instead gets a long-lived token as environment and a private config directory ([Credentials](#credentials)) |
| A minimal image: base OS, `git`, `bash`, `openssh-client`, Node + the harness CLI, `TERM=xterm-256color`, `WORKDIR /workspace` | The image is Foreman's default image ([Agent images](./tech-stack.md#agent-images)); the official `containers/podman/v5/pkg/bindings` module the sketch used is avoided in favour of the libpod REST API ([Podman runner](#podman-runner-production)) |
| Stop with a timeout, then remove | Containers outlive Foreman restarts and are re-attached, so removal happens only on instance stop |

### Reference: the PTY sketch

```go
package claudeorchestrator

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"://github.com"
)

type ClaudeAgentSession struct {
	ptyFile *os.File
	cmd     *exec.Cmd
	mutex   sync.Mutex
}

// NewClaudeAgentSession spawns a new isolated Claude CLI process bound to your local authentication
func NewClaudeAgentSession(workspaceDir string) (*ClaudeAgentSession, error) {
	// 1. Prepare the execution command
	c := exec.Command("claude")
	c.Dir = workspaceDir
	
	// Crucial: Inherit the local environment variables.
	// This carries your active subscription login state into the process.
	c.Env = os.Environ() 

	// 2. Start the process inside a real Linux Pseudo-Terminal (PTY)
	// This tricks Claude into interactive mode (Subscription) instead of automation mode (Agent SDK API)
	ptyFile, err := pty.Start(c)
	if err != nil {
		return nil, err
	}

	// Optional: Set a stable baseline window size so output rendering behaves predictably
	_ = pty.Setsize(ptyFile, &pty.Winsize{Rows: 40, Cols: 120, X: 0, Y: 0})

	return &ClaudeAgentSession{
		ptyFile: ptyFile,
		cmd:     c,
	}, nil
}

// SendCommand writes a prompt to the agent and streams the RAW response including ANSI codes
func (s *ClaudeAgentSession) SendCommand(prompt string, timeout time.Duration) ([]byte, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Append a newline character to simulate the user hitting "Enter"
	_, err := s.ptyFile.Write([]byte(prompt + "\n"))
	if err != nil {
		return nil, err
	}

	var outputBuf bytes.Buffer
	tmpBuf := make([]byte, 2048)
	expired := time.After(timeout)
	
	// Keep reading chunks of data until Claude rests and waits for a new prompt
	for {
		select {
		case <-expired:
			return outputBuf.Bytes(), errors.New("timeout reached while waiting for Claude to finish")
		default:
			// Set a short read deadline to non-blockingly check if new characters are arriving
			_ = s.ptyFile.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
			n, err := s.ptyFile.Read(tmpBuf)
			
			if n > 0 {
				// Write the RAW bytes directly into the buffer (preserving all ANSI, colors, and positioning data)
				outputBuf.Write(tmpBuf[:n])
				
				// HEURISTIC STOPPER: Check if Claude is waiting for input.
				// Since we are not stripping ANSI codes here, we inspect the tail bytes of the stream
				// for the raw prompt strings (e.g., "Claude Code >" or common interactive symbols)
				if bytes.Contains(outputBuf.Bytes(), []byte("Claude Code >")) {
					return outputBuf.Bytes(), nil
				}
			}
			
			if err != nil {
				if os.IsTimeout(err) {
					// If the stream paused, but we already have an established response block, 
					// we check if it looks like Claude ended its turn.
					if outputBuf.Len() > 0 && bytes.Contains(outputBuf.Bytes(), []byte("Claude Code")) {
						return outputBuf.Bytes(), nil
					}
					continue
				}
				if err == io.EOF {
					return outputBuf.Bytes(), nil
				}
				return outputBuf.Bytes(), err
			}
		}
	}
}

// Close gracefully kills the running agent session
func (s *ClaudeAgentSession) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	_ = s.ptyFile.Close()
	return s.cmd.Process.Kill()
}


```

### Reference: the Podman sketch

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"://github.com"
)

type PodmanClaudeAgent struct {
	ContainerID string
	ConnCtx     context.Context
}

// NewPodmanAgent spawns an isolated container running Claude Code via Go bindings
func NewPodmanAgent(imageName, workspaceHostPath string) (*PodmanClaudeAgent, error) {
	// 1. Establish connection to local Podman service via socket path
	// Rootless default socket location: /run/user/<UID>/podman/podman.sock
	uid := os.Getuid()
	socketURI := fmt.Sprintf("unix://run/user/%d/podman/podman.sock", uid)
	
	connCtx, err := bindings.NewConnection(context.Background(), socketURI)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to podman engine: %w", err)
	}

	// 2. Map host state files to preserve subscription authentication inside jail
	homeDir, _ := os.UserHomeDir()
	hostAuthPath := fmt.Sprintf("%s/.config/claude-code", homeDir)

	// 3. Configure the container specification
	spec := specgen.NewSpecGenerator(imageName, false)
	spec.Name = fmt.Sprintf("claude-agent-%d", time.Now().UnixNano())
	
	// Crucial settings for subscription safety:
	spec.Terminal = true // Enforces a real Linux TTY inside Podman
	spec.Entrypoint = []string{"claude"} // Directly fire the Claude binary

	// Set Mounts:
	// A: Host workspace mapped to internal path
	spec.Mounts = append(spec.Mounts, specgen.Mount{
		Source:      workspaceHostPath,
		Destination: "/workspace",
		Type:        "bind",
		Options:     []string{"rw", "z"}, // "z" handles SELinux flag adjustments if required
	})
	// B: Host Authentication cache mounted read-only to preserve session
	spec.Mounts = append(spec.Mounts, specgen.Mount{
		Source:      hostAuthPath,
		Destination: "/root/.config/claude-code",
		Type:        "bind",
		Options:     []string{"ro", "z"},
	})

	// 4. Create and start the container instance programmatically
	createResponse, err := containers.CreateWithSpec(connCtx, spec, nil)
	if err != nil {
		return nil, fmt.Errorf("container generation failed: %w", err)
	}

	err = containers.Start(connCtx, createResponse.ID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed starting container: %w", err)
	}

	return &PodmanClaudeAgent{
		ContainerID: createResponse.ID,
		ConnCtx:     connCtx,
	}, nil
}

// SendCommandInteractive connects to the TTY execution stream of the container
func (a *PodmanClaudeAgent) SendCommandInteractive(prompt string, timeout time.Duration) ([]byte, error) {
	// Prepare attachment streams to pipe raw input and receive responses with ANSI formatting intact
	// Using containers.Attach lets us talk directly to the container TTY line
	options := new(containers.AttachOptions).WithStream(true).WithStdin(true).WithStdout(true)

	// In a complete implementation, you establish raw IO loops over the socket returned by Attach.
	// Below is the high-level channel structure used by Podman bindings to execute commands:
	log.Printf("[Agent %s] Sending raw prompt to TTY stream...", a.ContainerID[:8])
	
	// For example purposes, we simulate the execution flow.
	// In production, you write prompt + "\n" into the net.Conn connection returned by bindings.
	
	return []byte("Raw response with \u001b[32mANSI colors\u001b[0m from Claude Code container >"), nil
}

// Cleanup destroys the tracking environment
func (a *PodmanClaudeAgent) Cleanup() {
	stopTimeout := uint(5)
	_ = containers.Stop(a.ConnCtx, a.ContainerID, &containers.StopOptions{Timeout: &stopTimeout})
	_ = containers.Remove(a.ConnCtx, a.ContainerID, nil)
}

func main() {
	// Initialize your Go orchestrated pool safely inside Podman!
	agent, err := NewPodmanAgent("custom-claude-dev-image:latest", "/home/user/code/my-project")
	if err != nil {
		log.Fatalf("Initialization failed: %v", err)
	}
	defer agent.Cleanup()

	// Execute command safely
	rawOutput, _ := agent.SendCommandInteractive("Check the codebase health status", 30*time.Second)
	fmt.Printf("Received raw buffer block:\n%s\n", string(rawOutput))
}

/*
FROM alpine:3.19

# Install system dependencies needed by Claude CLI
RUN apk add --no-cache \
    nodejs \
    npm \
    git \
    bash \
    openssh-client

# Install Claude Code globally 
RUN npm install -g @anthropic-ai/claude-code

# Set runtime workspace
WORKDIR /workspace

# Force shell execution configuration to ensure TTY behaves predictably
ENV TERM=xterm-256color

CMD ["claude"]

*/
```
