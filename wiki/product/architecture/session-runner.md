---
type: spec
status: partial
tags: [architecture, runtime, sessions, security]
---

# Session runner

Where and how a harness process actually runs: the **Execution engine** of the [building blocks](./building-blocks.md#1-execution-engine-runner). Nothing above it starts processes itself: the [Instance manager](./agent-runtime.md) goes through a small **Runner** interface, so the engine is swappable without touching anything else.

**Decided:** every agent instance runs in a **rootless Podman container from day one**. The **Direct** engine (a process on the host) exists for hermetic tests and development, and can be picked for real use only through an explicit setting with a "no isolation" warning.

## The Runner interface

Conceptually (exact Go signatures are an implementation detail; the authority is [Building blocks](./building-blocks.md#1-execution-engine-runner)):

| Operation | Does |
|---|---|
| `Start(spec) → process` | Starts a process from a spec: `{image, command, dir, env, mounts, tty size, labels}`. `labels` carry the instance id so processes can be found again |
| process: read / write | The process's terminal as a bidirectional byte stream (input in, merged output out) |
| process: `Resize(cols, rows)` | Propagates a terminal resize |
| process: `Stop(grace)` | Graceful stop (SIGTERM, then kill after `grace`) |
| process: `Wait() → exit code` | Blocks until the process exits (used to raise blocked requests) |
| `List() → []process` | Running processes this engine started (by label), for [recovery](./agent-runtime.md#recovery) after a Foreman restart |

The [Terminal](./building-blocks.md#2-terminal-terminal) holding a process tees every byte it reads into the instance's session log ([Persistence](./persistence.md)) before fanning it out to viewers ([API](./api.md#pty-websocket)).

## Podman engine (production)

- Foreman talks to Podman through the **libpod REST API over the Podman socket**, with a plain `net/http` client: no dependency, structured errors, attach as a hijacked stream. The official Go bindings module is avoided (a very large dependency tree), and so is shelling out to the `podman` CLI (to verify in the [Prove epic](../../backlog/epics/prove.md)).
- **Detach keys off.** Podman's attach intercepts a "detach" key sequence (Ctrl-P Ctrl-Q by default), which would swallow those keystrokes before the agent sees them. Foreman always attaches with `detachKeys=` (empty): every byte reaches the program. Checked by experiment A.
- **Containers are created and started one at a time.** Creating and starting many containers at once *with* the `keep-id` user mapping races inside Podman's runtime: 6 of 12 starts failed with `crun: write to /proc/sys/net/ipv4/ping_group_range … (are all the IDs mapped in the user namespace?)` (Podman 5.7.1 on macOS, measured 2026-10-14). One at a time with `keep-id`, and all at once *without* it, both worked 12/12; serializing only the starts while creates ran concurrently did not help. A create + start takes about 0.2 s, so a team of 12 is up in about 2.5 s; the sessions then run concurrently.
- Each instance is a container named from a short hash of the home and its instance id (`mara@ledger` → `foreman-19269e-mara-ledger`), labelled `foreman.instance=<id>`, `foreman.deployment=<id>`, `foreman.home=<the home's path>` and `foreman.managed=true`, which `List` looks for. Every Foreman on the machine shares one Podman, so each touches only the containers with its own home's label ([Agent runtime → Recovery](./agent-runtime.md#recovery)).
- **Attached before it starts**, so the program's first output reaches the log. An exited container is removed, freeing its name; a stopped one left with the same name (its program exited while Foreman was down) is removed before the next start, and a running one refuses it.
- **No CPU or memory limits** by default; a [global setting](../models/global-settings.md) can add them later if 20+ sessions turn out to starve the laptop.
- **Mounts, and nothing else of the host:**
  - the instance's sandbox `~/.foreman/sandboxes/<project>/<agent>/` (its git worktrees) → read-write, as the working directory, **at the same path as on the host** (as `pclaude` does: harnesses key their session history by path, and paths in output match what the CEO sees)
  - each code repo's `.git` → **read-only**, at its own real path, wherever the CEO keeps the repo: a worktree points at its repo by host path, and agents read git but never write it ([Sandboxes and git policy](./sandboxes-and-git.md#agents-and-git))
  - the instance's private harness config directory → read-write ([Credentials](#credentials))
  - the live project repo `~/.foreman/projects/<name>/` → at `<sandbox>/project`, read-write for the lead, read-only for everyone else, with its `.git` read-only for everyone ([Sandboxes and git policy](./sandboxes-and-git.md#project-knowledge-base))
  - the login of the project's profile and the `foreman-agent` binary ([Credentials](#credentials), [Team tools → Transport](./team-tools.md#transport))
  - no `~/.config`, no `~/.ssh`, no keychain, nothing else of `~/.foreman/` (secrets, state, the global repo, other sandboxes), no other agents' worktrees, none of the CEO's working files.
- **Harness credentials:** see [Credentials](#credentials). The host's harness config directories are never mounted.
- **TTY:** the harness is the container's main process with a TTY, so the container keeps running if Foreman dies (a clean exit stops it: [Agent runtime → Recovery](./agent-runtime.md#recovery)). After a Foreman restart, `List` finds it by label and Foreman re-attaches. Output printed while Foreman was down isn't in its log; that gap is accepted, and the pane shows `ⓘ Foreman restarted, output may be missing` at that point.
- **User mapping:** `--userns keep-id:uid=1000,gid=1000` (the image's non-root user), so files written into the worktrees and the project repo are owned by the CEO's user, not a subordinate UID.
- **SELinux:** on hosts where `/sys/fs/selinux/enforce` exists (Fedora, RHEL), bind mounts get the shared `:z` label; elsewhere no suffix. The private `:Z` is not used: several containers mount the same project repo and `.git`, and `:Z` would relabel it away from the others (learned in `pclaude`).
- **Network:** containers need the network for model APIs. They reach Foreman's agent port at `host.containers.internal`, through `foreman-agent` ([Team tools → Transport](./team-tools.md#transport)).
- **macOS:** Podman runs inside a VM (`podman machine`). Bind mounts cross the VM boundary (virtiofs), which is slower for heavy file I/O (e.g. `npm install`) and needs the paths it mounts (the home and the code repos, all inside the CEO's home folder) shared with the machine. Unix sockets don't cross that boundary.
- **Images:** each project's instances run its layer-4 image, which carries the harnesses and the project's toolchain ([Agent images](./tech-stack.md#agent-images)).

### The Podman service

Foreman never spawns `podman` for container or image work: it calls the libpod REST API directly (create, attach, resize, stop, list, build, tag, prune), so images are managed by the binary itself. The API needs Podman's service to be running, and **Foreman makes sure it is**, so that starting `foreman` is the only step:

- **Linux:** Foreman connects to the rootless socket (`$XDG_RUNTIME_DIR/podman/podman.sock`). If nothing answers, it starts `podman system service --time=0` on that socket as a child process, which ends with Foreman. (A CEO who prefers systemd socket activation, `systemctl --user enable --now podman.socket`, gets the same socket and Foreman just uses it.)
- **macOS:** Podman runs in a Linux VM. If the default machine isn't running at launch, Foreman runs `podman machine start` and connects to the API socket the machine exposes. Foreman never stops the machine on exit: other tools may use it, and the containers (the agents' sessions) live in it.
- **Foreman starts whether or not Podman is there** ([First run](../features/first-run.md)): the setup watcher reaches it at launch and every 3 seconds after, and the Home checklist says what is missing. Installing Podman stays the CEO's step (the README); on macOS the checklist creates the machine, sized by the CEO. A stopped machine is started at Foreman's launch and by the checklist's **Start**, never by the periodic check, so one the CEO stopped stays stopped.
- The state ("Podman 5.x · running", or the machine's state) is shown in [Settings → Maintenance](../features/settings.md#maintenance).

Linking Podman's engine (libpod) into Foreman instead isn't viable: it would make Foreman a container engine, with cgo, conmon, crun and storage drivers, Linux only, and still no answer for macOS, where containers must run in a VM. To verify in the [Prove epic](../../backlog/epics/prove.md).

## Credentials

**One set of credentials per profile, set once, shared by every agent of every project.** For Claude Code, the harness's own interactive login (`/login` in a terminal): no tokens to paste. For Pi, a model server and one model on it, set in a form, with an optional API key stored in the profile folder ([Harness adapters → Pi](./harness-adapters.md#pi)).

**Profiles** (the CEO, 2026-10-18): a **profile** is a name, one harness and that harness's credentials: `default` is Claude Code and its login; `work` could be Claude Code with another account; `local` could be Pi on a model server, saved as Pi's `models.json`. Profiles are the CEO's, on this machine, shared by every project: `~/.foreman/profiles/<name>/` on macOS and Linux alike (in Foreman's home, [Persistence](./persistence.md)), each holding `profile.toml` (`harness = "claude-code"`) and the harness's own files, here its login (`internal/profiles`). They are managed in [Settings → Global → Profiles](../features/settings.md#profiles). `default` always exists (Foreman creates it, for Claude Code, at start) and can't be deleted. A profile name is one folder, never a path.

**A project uses one profile** (simple for now; per agent later: [Profiles per agent](../../backlog/profiles-per-agent.md)): `default` unless its [Project settings](../features/project-settings.md) pick another, kept on the project's [record](../models/project-config.md#the-project-record) in `state.db` (`PUT /api/projects/{id}/profile`), never in `project.toml`, which the lead can write. Every agent deployed on it runs on that profile's harness with its login. The harness and the login are looked up **at each start**, so a change reaches an agent at its next start; running ones keep theirs. Agents don't name a harness: a `harness` or `command` line in a team file is reported as a field Foreman doesn't know ([Team definition](../models/team-definition.md)). Deploy refuses, with the profile named, when the project's profile doesn't exist, its harness isn't supported, or it isn't connected. `just dev` and the Claude Code integration test (`just test-claude [profile]`) use the CEO's own profiles, so none logs in again; Go tests point Foreman at a temporary home (`--home`).

**The login container.** Settings → Global → Profiles → **`Connect`** starts a one-off **login container** from the harness image and shows its terminal in the browser, running the profile's harness login. The login lands in the **profile's folder** (0600), owned by Foreman and separate from the CEO's own `~/.claude`. Settings then shows `connected`, and **`Reconnect`** repeats the flow. Because the login happens inside Linux, the host's own login (on macOS, in the Keychain) is never involved and never synced.

**One writable login, shared by every instance on the profile; everything else private.** Every agent container mounts the profile's login file **read-write**, and the instance's private config directory points the harness at it (the adapter declares where, e.g. Claude Code's `.credentials.json` within `CLAUDE_CONFIG_DIR`). This is the situation the harnesses already handle on a single machine, with several `claude` sessions sharing one login: when one refreshes the token, it writes the shared file and the others read the new token instead of refreshing with a revoked one. With profiles that sharing spans projects: agents of every project on the profile mount the same file. If a profile has no login yet, its project's agents don't start (Deploy and the setup checklist say which profile to connect). If the shared login stops working (revoked, or a logout elsewhere), every affected instance raises one **blocked** request: "Reconnect the <profile> profile in Settings".

**How the login file is shared** ([experiment B](../../backlog/archive/prove-b-shared-login.md), Claude Code on macOS): the login file alone is **bind-mounted** into each instance's otherwise private config directory. Claude Code writes it in place, so a refresh by one instance lands in the shared file and the others pick it up: 12 sessions refreshing an expired token at once all succeeded, and all succeeded again afterwards. A **symlink** to the shared file breaks sharing (the first refresh replaces the link with a private file, and every other instance fails). Sharing the whole folder as the config dir also survived, but it would share everything, not just the login. An interactive session also refreshes mid-session into the shared file ([experiment D](../../backlog/archive/prove-d-claude-delivery-hooks.md)). Still to check: Linux, and each other harness. Claude Code also keeps an account record in `.claude.json`: each instance gets its own copy, seeded from the login's, with onboarding and the working folder's trust marked as done, and in its `settings.json` `skipDangerousModePermissionPrompt` plus `tui: "default"`, so an interactive session starts straight at its prompt (D). The classic renderer is pinned because the fullscreen one takes the mouse (no plain copy in the pane) and the alternate screen (no scrollback), and because an unset `tui` lets Claude Code 2.1.3xx open "Try the new fullscreen renderer?" at startup, which ate the first delivered line. The Connect flow runs `/login` explicitly, because the harness never asks on its own.

If sharing doesn't hold for some harness, the fallbacks are a **keeper** (the login container refreshes shortly before expiry and Foreman copies the fresh login to every instance) or one login per instance.

**Private config, sharing nothing but the login.** Each instance gets a **private harness config directory** (Claude Code: `CLAUDE_CONFIG_DIR`) in its sandbox area (`<sandbox>/.harness/<harness>/`, inside the sandbox mount), holding its own settings, history and memory, and its [session prompt](./session-prompt.md#delivery). The CEO's `~/.claude` is never mounted: an agent could plant a hook or setting there that later runs *on the host*, and would see the CEO's history from every other project. Forge tokens (`GH_TOKEN` and the like) are **never** injected: only Foreman's push path uses them ([Security](./security.md)).

**Git config.** If the CEO has a `~/.gitconfig`, it is mounted **read-only** into every agent container (nothing is mounted when it doesn't exist), so aliases and preferences behave as on the host. Agents make no commits themselves: every `.git` is read-only in their containers, and Foreman's `commit` tool commits on the host, authored as the agent (its display name and a per-agent address), so every commit says which agent made it ([Agents and git](./sandboxes-and-git.md#agents-and-git)). Credential helpers or `include` paths in the file that point at host programs or files simply don't resolve inside the container; agents never push to a forge anyway.

## Direct engine (tests and development)

- Spawns the command directly on the host in a pseudo-terminal (the approach of the original PTY proof of concept).
- Used with a **fake scripted harness** (a small program that prints scripted output, calls team tools and reacts to stdin) so the Terminal, Instance manager, Messaging and Workflow can be tested end-to-end, fast and without Podman or a model.
- Gives **no isolation**: it is for tests and development, and selectable for real use only through an explicit setting that shows a "no isolation" warning ([Global settings](../models/global-settings.md)).
- Sessions die with Foreman, so `List` returns nothing after a restart.

A tmux-based runner (sessions survive restarts, attachable from a real terminal) could be a third implementation later. Not planned.

Images are built by Foreman as a four-layer chain ([Tech stack](./tech-stack.md#agent-images)); the Direct engine's image builder is a no-op and the process uses the host's tools ([Building blocks](./building-blocks.md#1b-image-builder-runnerengine)).

## Prior art: the two proofs of concept

Two Go sketches written before this design, kept verbatim as reference at the end of this section (a PTY session and a Podman session). They are not production code: neither compiles as written (a mangled import path), and their approach to I/O is replaced by this design. The third piece of prior art is [`pclaude`](https://github.com/brickpop/pclaude), the CEO's launcher that runs Claude Code in Podman; its credential handling is the basis of [Credentials](#credentials). What the sketches established, and what this design changes:

| Kept | Changed |
|---|---|
| `creack/pty`: `pty.Start(cmd)` plus `pty.Setsize` with a stable baseline (120×40) before the first output, so the TUI lays out predictably | The sketch read the PTY in request/response chunks, with a 300ms read deadline and a substring match on `"Claude Code >"` to guess the end of a turn. Here the PTY is a continuous byte stream fanned out to WebSocket viewers and the log, and turn detection comes from harness signals ([Harness adapters](./harness-adapters.md)) |
| Running the harness **interactively, in a real TTY** | - |
| Podman container spec: `Terminal: true` (a real TTY in the container), the harness as entrypoint, a host folder bind-mounted rw with the `z` option (SELinux relabelling, needed on Fedora and other SELinux hosts) | The sketch mounted its one workspace folder; here it is the agent's own sandbox, and nothing else of the home ([Home → Invariants](../models/home.md#invariants)). It also mounted the host's harness config dir read-only to reuse its login. That leaks every credential in it into every container and doesn't work on macOS (the Keychain); each instance instead gets a long-lived token as environment and a private config directory ([Credentials](#credentials)) |
| A minimal image: base OS, `git`, `bash`, `openssh-client`, Node + the harness CLI, `TERM=xterm-256color` | The sketch's `WORKDIR /workspace` is gone: an agent starts in its sandbox. The image is Foreman's default image ([Agent images](./tech-stack.md#agent-images)); the official `containers/podman/v5/pkg/bindings` module the sketch used is avoided in favour of the libpod REST API ([Podman runner](#podman-engine-production)) |
| Stop with a timeout, then remove | Containers outlive a Foreman crash and are re-attached, so removal happens only on instance stop, deployment stop or a clean exit |

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
