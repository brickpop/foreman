// Command foreman serves a workspace: `foreman [<workspace-dir>]`.
// See wiki/product/models/workspace.md#starting-foreman.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/brickpop/foreman/agentbin"
	"github.com/brickpop/foreman/internal/access"
	"github.com/brickpop/foreman/internal/agentlink"
	"github.com/brickpop/foreman/internal/events"
	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/harness"
	"github.com/brickpop/foreman/internal/harness/claudecode"
	"github.com/brickpop/foreman/internal/harness/fake"
	"github.com/brickpop/foreman/internal/instances"
	"github.com/brickpop/foreman/internal/messaging"
	"github.com/brickpop/foreman/internal/runner/podman"
	"github.com/brickpop/foreman/internal/server"
	"github.com/brickpop/foreman/internal/sessionlog"
	"github.com/brickpop/foreman/internal/setup"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/teamtools"
	"github.com/brickpop/foreman/internal/terminal"
	"github.com/brickpop/foreman/internal/wiki"
	"github.com/brickpop/foreman/internal/workflow"
	"github.com/brickpop/foreman/internal/workspace"
	"github.com/brickpop/foreman/ui"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "foreman:", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	flags := flag.NewFlagSet("foreman", flag.ContinueOnError)
	yes := flags.Bool("yes", false, "create .foreman/ without asking if the folder isn't a workspace yet")
	noOpen := flags.Bool("no-open", false, "print the URL but don't open the browser")
	port := flags.String("port", "8888", "port for the web UI")
	newToken := flags.Bool("new-token", false, "replace the access token (logs every browser out)")
	uiDir := flags.String("ui-dir", "", "development: serve the web UI from this folder instead of the embedded build")
	fakeHarness := flags.String("fake-harness", "", "development: run agents and logins on this program (cmd/fakeharness) instead of Claude Code")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "Usage: foreman [<workspace-dir>] [flags]")
		flags.PrintDefaults()
	}
	dir, err := parseArgs(flags, args)
	if err != nil {
		return err
	}

	ws, err := openOrCreate(dir, *yes, stdin, stdout)
	if err != nil {
		return err
	}
	token, err := ws.AccessToken(*newToken)
	if err != nil {
		return fmt.Errorf("access token: %w", err)
	}
	ctx := context.Background()
	gitCLI := git.New()
	if err := ws.Load(ctx, gitCLI); err != nil {
		return err
	}
	// The engine is chosen once: a change of the setting takes effect on restart.
	running := ws.Settings().Settings.Engine
	// The fake harness is a host binary: a container can't run it.
	if *fakeHarness != "" && running != "direct" {
		return fmt.Errorf("this workspace runs on %s, where --fake-harness can't run (it is a host program): drop --fake-harness to run Claude Code, or set engine = \"direct\" in settings.toml", running)
	}
	engineRunner, engineImages, pod := engine(running, stdout)
	if pod != nil {
		defer pod.Close()
	}

	// Listen before printing the URL, so a busy port fails before the browser opens.
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", *port))
	if err != nil {
		return err
	}
	// The agent port: the next one, on the host's loopback only, where
	// containers reach it through Podman's host alias (experiment C).
	agentPort, err := nextPort(*port)
	if err != nil {
		return err
	}
	agentListener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", agentPort))
	if err != nil {
		return fmt.Errorf("the agent port: %w", err)
	}
	url := fmt.Sprintf("http://127.0.0.1:%s/?token=%s", *port, token)
	fmt.Fprintf(stdout, "Foreman is serving %s\nOpen: %s\n", ws.Root(), url)
	if !*noOpen {
		openBrowser(url, stdout)
	}

	// The composition root: the only place that picks concrete implementations.
	files := ui.UI()
	if *uiDir != "" {
		files = os.DirFS(*uiDir)
	}
	state, err := store.Open(ctx, ws.Path("state.db"))
	if err != nil {
		return fmt.Errorf("state: %w", err)
	}
	defer state.Close()
	bus := events.New()
	if err := ws.Watch(ctx, func(c workspace.Change) { bus.Publish(configEvent(c)) }); err != nil {
		return fmt.Errorf("watching the workspace: %w", err)
	}
	logs := sessionlog.Dir{Root: ws.Path("logs")}
	openLog := func(instance string) (terminal.Log, error) { return logs.Open(instance) }
	adapters := map[string]harness.Adapter{
		"claude-code": claudecode.Adapter{Credentials: ws.Path("credentials", "claude-code"), GitConfig: homeGitConfig()},
	}
	if *fakeHarness != "" {
		program, err := resolveProgram(*fakeHarness)
		if err != nil {
			return fmt.Errorf("fake harness: %w", err)
		}
		adapters["claude-code"] = fake.Adapter{Program: program}
		fmt.Fprintf(stdout, "Agents run on the fake harness (%s), not Claude Code\n", program)
	}
	secret, err := ws.AgentSecret()
	if err != nil {
		return fmt.Errorf("agent link: %w", err)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("foreman-agent: %w", err)
	}
	agentHost, err := writeAgent(filepath.Join(cache, "foreman"), agentbin.Binary(runtime.GOARCH), time.Now())
	if err != nil {
		return fmt.Errorf("foreman-agent: %w", err)
	}
	if agentHost == "" {
		fmt.Fprintf(stdout, "foreman-agent isn't built for %s (just agent): agents start without the team tools\n", runtime.GOARCH)
	}
	var link *agentlink.Link
	agent := instances.AgentLink{
		Host: agentHost, Path: "/opt/foreman/foreman-agent", URL: "http://host.containers.internal:" + agentPort,
		Token: func(id string) string { return link.Token(id) },
	}
	if running == "direct" {
		// A host process: no mount, no host alias.
		agent.Path, agent.URL = agentHost, "http://127.0.0.1:"+agentPort
	}
	// Messaging hears every status change and stop from the manager itself,
	// never through the bus, which drops a subscriber that falls behind.
	var messenger *messaging.Messenger
	var checklist *setup.Setup
	publish := func(e events.Event) {
		bus.Publish(e)
		switch d := e.Data.(type) {
		case instances.StatusChange:
			messenger.StatusChanged(d.ID, d.Status)
		case store.Deployment:
			if e.Type == "deployment.stopped" {
				messenger.DeploymentStopped(d.ID)
			}
		}
	}
	manager := instances.New(instances.Deps{
		Root: ws.Root(), Runner: engineRunner, Images: engineImages, Git: gitCLI, Config: ws,
		Deployments: state.Deployments(), Instances: state.Instances(), Adapters: adapters,
		OpenLog: openLog, Publish: publish, Agent: agent,
		Blocked: func() error { return checklist.Blocked() },
	})
	messenger = messaging.New(messaging.Deps{
		Messages: state.Messages(), Instances: state.Instances(), Deployments: state.Deployments(),
		Config: ws, Sessions: manager, Publish: bus.Publish, Pause: 150 * time.Millisecond,
	})
	flow := workflow.New(workflow.Deps{Root: ws.Root(), Git: gitCLI, Config: ws, Messages: messenger,
		Actions: state.Actions(), Publish: bus.Publish})
	router := &teamtools.Router{Instances: state.Instances(), Deployments: state.Deployments(), Config: ws,
		Blocks: []teamtools.Block{messenger, flow}}
	link = agentlink.New(secret, manager.Live, router, manager)
	agentServer := &http.Server{Handler: link.Handler()}
	go func() {
		if err := agentServer.Serve(agentListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("the agent port stopped", "err", err)
		}
	}()
	login := logins{adapters: adapters, images: engineImages, terms: newSessions(engineRunner, openLog)}
	// Foreman serves whether or not Podman is there: the setup watcher reaches
	// it, then restores the instances and delivers what waited.
	var podEngine setup.Engine
	if pod != nil {
		podEngine = pod
	}
	sweep := func(ctx context.Context) {
		if err := messenger.Sweep(ctx); err != nil {
			slog.Error("delivering what waited", "err", err)
		}
	}
	checklist = setup.New(setup.Deps{
		Engine: podEngine, Images: engineImages, Harnesses: login.Harnesses, Config: ws, Publish: bus.Publish,
		Ready: func(ctx context.Context) error {
			if err := manager.Restore(ctx); err != nil {
				return fmt.Errorf("restoring the instances: %w", err)
			}
			sweep(ctx)
			return nil
		},
		Reconnected: sweep,
	})
	// A clean exit (Ctrl-C, SIGTERM) stops the agents; a crash leaves them to
	// the next start's restore.
	quit, stopSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	go checklist.Run(quit)
	views := agents{Manager: manager, logins: login.terms, typed: messenger.Typed}
	handler := server.New(server.Options{
		Guard:       access.NewTokenGuard(token, access.LocalHosts(*port), *port),
		Workspace:   ws,
		Config:      ws,
		Events:      bus,
		Agents:      views,
		Messages:    messenger,
		Actions:     state.Actions(),
		Harnesses:   login,
		Wiki:        wiki.New(ws.WikiDir),
		Setup:       checklist,
		Maintenance: maintenanceOf(running, engineRunner, engineImages, podman.Installed),
		Terminals:   views,
		UI:          files,
	})
	return serveUntil(quit, stopSignals, stdout, &http.Server{Handler: handler}, listener, func() {
		manager.Shutdown(context.Background())
		login.terms.stopAll()
		_ = agentServer.Close()
	})
}

// serveUntil serves until quit, then stops serving and runs shutdown. A
// second signal during it ends Foreman at once (release puts the signals
// back to their default).
func serveUntil(quit context.Context, release func(), stdout io.Writer, srv *http.Server, l net.Listener, shutdown func()) error {
	served := make(chan error, 1)
	go func() { served <- srv.Serve(l) }()
	select {
	case err := <-served:
		return err
	case <-quit.Done():
	}
	release()
	fmt.Fprintln(stdout, "Stopping the agents (Ctrl-C again to leave them running)")
	_ = srv.Close()
	shutdown()
	return nil
}

// parseArgs accepts flags before and after the workspace folder (the flag
// package alone stops at the first positional argument).
func parseArgs(flags *flag.FlagSet, args []string) (string, error) {
	var positional []string
	for {
		if err := flags.Parse(args); err != nil {
			return "", err
		}
		if flags.NArg() == 0 {
			break
		}
		positional = append(positional, flags.Arg(0))
		args = flags.Args()[1:]
	}
	switch len(positional) {
	case 0:
		return ".", nil
	case 1:
		return positional[0], nil
	default:
		return "", errors.New("expected at most one workspace folder")
	}
}

func openOrCreate(dir string, yes bool, stdin io.Reader, stdout io.Writer) (*workspace.Workspace, error) {
	ws, err := workspace.Open(dir)
	if !errors.Is(err, workspace.ErrNotInitialized) {
		return ws, err
	}
	if !yes {
		fmt.Fprintf(stdout, "%s isn't a Foreman workspace yet. Create %s/ here? [y/N] ", dir, workspace.DirName)
		answer, _ := bufio.NewReader(stdin).ReadString('\n')
		if strings.ToLower(strings.TrimSpace(answer)) != "y" {
			return nil, errors.New("not a workspace; nothing was created")
		}
	}
	return workspace.Init(dir)
}

// openBrowser is best effort: the URL is printed either way.
func openBrowser(url string, stdout io.Writer) {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	if err := exec.Command(name, url).Start(); err != nil {
		fmt.Fprintln(stdout, "(couldn't open a browser; use the URL above)")
	}
}

// resolveProgram makes a relative program path ("bin/x", "./x") absolute,
// against the folder foreman was started in: agents run in another working
// directory, where the relative path would point somewhere else. Bare names
// ("bash") are left for the PATH lookup.
func resolveProgram(program string) (string, error) {
	if !strings.ContainsRune(program, filepath.Separator) || filepath.IsAbs(program) {
		return program, nil
	}
	return filepath.Abs(program)
}

// nextPort is the port after port: the agent port, next to the UI's.
func nextPort(port string) (string, error) {
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65534 {
		return "", fmt.Errorf("--port %q: not a port with a free one after it", port)
	}
	return strconv.Itoa(n + 1), nil
}

// configEvent turns a change on disk into the event the browser gets
// (wiki/product/architecture/api.md#events).
func configEvent(c workspace.Change) events.Event {
	if c.Kind == "projects" {
		return events.Event{Type: "workspace.changed", Data: struct{}{}}
	}
	return events.Event{Type: "config.changed", Data: c}
}

// homeGitConfig is the CEO's ~/.gitconfig, mounted read-only into agents'
// containers when it exists.
func homeGitConfig() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".gitconfig")
}
