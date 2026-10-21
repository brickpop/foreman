// Command foreman runs Foreman on this machine, from its home (~/.foreman):
// `foreman [flags]`. See wiki/product/models/home.md#starting-foreman.
package main

import (
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
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/brickpop/foreman/agentbin"
	"github.com/brickpop/foreman/internal/access"
	"github.com/brickpop/foreman/internal/agentlink"
	"github.com/brickpop/foreman/internal/cos"
	"github.com/brickpop/foreman/internal/events"
	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/harness"
	"github.com/brickpop/foreman/internal/harness/claudecode"
	"github.com/brickpop/foreman/internal/harness/fake"
	"github.com/brickpop/foreman/internal/harness/pi"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/instances"
	"github.com/brickpop/foreman/internal/messaging"
	"github.com/brickpop/foreman/internal/profiles"
	"github.com/brickpop/foreman/internal/repos"
	"github.com/brickpop/foreman/internal/runner/podman"
	"github.com/brickpop/foreman/internal/server"
	"github.com/brickpop/foreman/internal/sessionlog"
	"github.com/brickpop/foreman/internal/setup"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/teamtools"
	"github.com/brickpop/foreman/internal/terminal"
	"github.com/brickpop/foreman/internal/userdata"
	"github.com/brickpop/foreman/internal/wiki"
	"github.com/brickpop/foreman/internal/workflow"
	"github.com/brickpop/foreman/ui"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "foreman:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("foreman", flag.ContinueOnError)
	noOpen := flags.Bool("no-open", false, "print the URL but don't open the browser")
	port := flags.String("port", "8888", "port for the web UI")
	newToken := flags.Bool("new-token", false, "replace the access token (logs every browser out)")
	uiDir := flags.String("ui-dir", "", "development: serve the web UI from this folder instead of the embedded build")
	fakeHarness := flags.String("fake-harness", "", "development: run agents and logins on this program (cmd/fakeharness) instead of Claude Code")
	homeFlag := flags.String("home", "", "Foreman's home on this machine (teams, settings, projects, logins, state), instead of ~/.foreman: for tests and development")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "Usage: foreman [flags]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("foreman takes no folder (%s): it runs from its home, ~/.foreman, and projects are added in its UI", flags.Arg(0))
	}

	// Foreman's home, and the lock that keeps Foreman to one server per machine.
	var err error
	userDir := *homeFlag
	if userDir == "" {
		if userDir, err = userdata.Dir(); err != nil {
			return fmt.Errorf("finding Foreman's home: %w", err)
		}
	}
	// Absolute: its paths are mounted into containers and compared with repos'.
	if userDir, err = filepath.Abs(userDir); err != nil {
		return err
	}
	lock, err := userdata.TakeLock(userDir)
	var other *userdata.RunningError
	if errors.As(err, &other) {
		fmt.Fprintf(stdout, "Foreman is already running on this machine (one per machine)\nOpen: %s\n", other.URL)
		if !*noOpen && other.URL != "" {
			openBrowser(other.URL, stdout)
		}
		return nil
	}
	if err != nil {
		return err
	}
	defer lock.Release()

	h, err := home.Open(userDir)
	if err != nil {
		return err
	}
	token, err := h.AccessToken(*newToken)
	if err != nil {
		return fmt.Errorf("access token: %w", err)
	}
	ctx := context.Background()
	gitCLI := git.New()
	state, err := store.Open(ctx, h.Path("state.db"))
	if err != nil {
		return fmt.Errorf("state: %w", err)
	}
	defer state.Close()
	var manager *instances.Manager
	deployed := func(ctx context.Context, project string) bool {
		active, _ := manager.DeploymentsIn(ctx, store.DeploymentActive)
		return slices.ContainsFunc(active, func(d store.Deployment) bool { return d.ProjectID == project })
	}
	// Repos are added from where Foreman was started (the folder browser's top).
	start, _ := os.Getwd()
	if err := h.Load(ctx, home.Deps{Git: gitCLI, Projects: state.Projects(), Deployed: deployed, Start: start}); err != nil {
		return err
	}
	// The engine is chosen once: a change of the setting takes effect on restart.
	running := h.Settings().Settings.Engine
	// The fake harness is a host binary: a container can't run it.
	if *fakeHarness != "" && running != "direct" {
		return fmt.Errorf("this Foreman runs on %s, where --fake-harness can't run (it is a host program): drop --fake-harness to run Claude Code, or set engine = \"direct\" in settings.toml", running)
	}
	engineRunner, engineImages, pod := engine(running, filepath.Join(userDir, "images"), stdout)
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
	if err := lock.Announce(url); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Foreman is running from %s\nOpen: %s\n", h.Root(), url)
	if !*noOpen {
		openBrowser(url, stdout)
	}

	// The composition root: the only place that picks concrete implementations.
	files := ui.UI()
	if *uiDir != "" {
		files = os.DirFS(*uiDir)
	}
	bus := events.New()
	wikis := wiki.New(h.WikiDir, gitCLI)
	followWikis, err := wikis.Watch(ctx, func() []string {
		var ids []string
		for _, p := range h.Projects() {
			ids = append(ids, p.ID)
		}
		return ids
	}, func(c wiki.Changed) { bus.Publish(events.Event{Type: "wiki.changed", Data: c}) })
	if err != nil {
		return fmt.Errorf("watching the wikis: %w", err)
	}
	// Each agent's logs stay under the log_cap_mb setting, as it is now.
	logs := sessionlog.Dir{Root: h.LogsDir(), Cap: func() int64 { return int64(h.Settings().Settings.LogCapMB) << 20 }}
	openLog := func(instance string) (terminal.Log, error) { return logs.Open(instance) }
	// How a container reaches this machine: the agent port, a model served
	// here. Agents on the Direct engine are on it.
	hostAlias := "host.containers.internal"
	if running == "direct" {
		hostAlias = ""
	}
	claude := claudecode.Adapter{GitConfig: homeGitConfig()}
	adapters := map[string]harness.Adapter{
		"claude-code": claude,
		"pi":          pi.Adapter{GitConfig: homeGitConfig(), HostAlias: hostAlias},
	}
	profileStore := profiles.In(userDir)
	// Always the real harnesses' logins, even on the fake harness, which has
	// none; then the default profile, which every project uses unless it
	// names another.
	if err := profileStore.Ensure("claude-code"); err != nil {
		return fmt.Errorf("the default profile: %w", err)
	}
	if *fakeHarness != "" {
		program, err := resolveProgram(*fakeHarness)
		if err != nil {
			return fmt.Errorf("fake harness: %w", err)
		}
		adapters["claude-code"] = fake.Adapter{Program: program}
		fmt.Fprintf(stdout, "Agents run on the fake harness (%s), not Claude Code\n", program)
	}
	secret, err := h.AgentSecret()
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
		Host: agentHost, Path: "/opt/foreman/foreman-agent", URL: "http://" + hostAlias + ":" + agentPort,
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
	login := profileLogins{store: profileStore, adapters: adapters, images: engineImages, terms: newSessions(engineRunner, openLog),
		project: h.Project}
	// Workflow needs the manager (to stop a closed review's team), and the
	// manager tells Workflow of each fresh session: set below.
	var flow *workflow.Workflow
	manager = instances.New(instances.Deps{
		Home: h.Root(), Runner: engineRunner, Images: engineImages, Git: gitCLI, Config: h,
		Deployments: state.Deployments(), Instances: state.Instances(), Adapters: adapters, Profile: login.Of,
		OpenLog: openLog, Publish: publish, Agent: agent,
		Blocked: func() error { return checklist.Blocked() }, Requests: state.Requests(),
		Started: func(ctx context.Context, inst store.Instance, lead bool) {
			if flow != nil {
				flow.Resumed(ctx, inst, lead)
			}
		},
	})
	// An agent quiet mid-turn for the stuck_after_minutes setting gets a card.
	go func() {
		tick := time.NewTicker(time.Minute)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-tick.C:
				manager.CheckStuck(ctx, now)
			}
		}
	}()
	// Watched from here on: a settings change may let queued agents start.
	if err := h.Watch(ctx, func(c home.Change) {
		bus.Publish(configEvent(c))
		switch c.Kind {
		case "projects":
			followWikis()
			manager.ProjectsChanged(ctx)
		case "project":
			manager.ProjectsChanged(ctx)
		case "settings":
			manager.LimitChanged()
		}
	}); err != nil {
		return fmt.Errorf("watching the home: %w", err)
	}
	// A project gone while Foreman was off stops its deployments too.
	manager.ProjectsChanged(ctx)
	// Messaging and Workflow need each other: a hand-off is a message, and its
	// delivery moves the hand-off's work item.
	messenger = messaging.New(messaging.Deps{
		Messages: state.Messages(), Instances: state.Instances(), Deployments: state.Deployments(),
		Config: h, Sessions: manager, Publish: bus.Publish, Pause: 150 * time.Millisecond,
		Delivered: func(ctx context.Context, msg store.TeamMessage) { flow.Delivered(ctx, msg) },
	})
	flow = workflow.New(workflow.Deps{Git: gitCLI, Config: h, Messages: messenger, Teams: manager,
		Actions: state.Actions(), Publish: bus.Publish, WorkItems: state.WorkItems(),
		MergeRequests: state.MergeRequests(), Wiki: wikis, Requests: state.Requests(),
		Reviews: state.Reviews(), ReviewComments: state.ReviewComments()})
	// A merge request merged outside Foreman (the CEO's own git, a forge) is
	// noticed within half a minute; backups past their time go hourly.
	go func() {
		tick, hourly := time.NewTicker(30*time.Second), time.NewTicker(time.Hour)
		defer tick.Stop()
		defer hourly.Stop()
		flow.PruneBackups(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				flow.Reconcile(ctx)
			case <-hourly.C:
				flow.PruneBackups(ctx)
			}
		}
	}()
	router := &teamtools.Router{Instances: state.Instances(), Deployments: state.Deployments(), Config: h,
		Blocks: []teamtools.Block{messenger, flow}}
	chief := &chiefOfStaff{
		home: h.Root(), dir: h.Path("cos"), teamsDir: h.TeamsDir(), teams: h.Teams,
		profiles: profileStore, adapters: adapters, images: engineImages, terms: login.terms, agent: agent,
		blocked: func() error { return checklist.Blocked() }, pause: 150 * time.Millisecond,
	}
	chief.CoS = cos.New(cos.Deps{Teams: h, Publish: bus.Publish, Tell: chief.tell,
		Deployed: func(ctx context.Context, team string) bool {
			active, _ := manager.DeploymentsIn(ctx, store.DeploymentActive)
			return slices.ContainsFunc(active, func(d store.Deployment) bool { return d.TeamID == team })
		},
	})
	live := func(id string) bool {
		if id == cos.Instance {
			return chief.Live()
		}
		return manager.Live(id)
	}
	link = agentlink.New(secret, live, agentCalls{team: router, cos: chief}, manager)
	agentServer := &http.Server{Handler: link.Handler()}
	go func() {
		if err := agentServer.Serve(agentListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("the agent port stopped", "err", err)
		}
	}()
	// Foreman serves whether or not Podman is there: the setup watcher reaches
	// it, then restores the instances and delivers what waited.
	var podEngine setup.Engine
	if pod != nil {
		podEngine = pod
	}
	checklist = setup.New(setup.Deps{
		Engine: podEngine, Images: engineImages, Config: h, Publish: bus.Publish,
		Profiles: func() []profiles.Profile { return login.Used(h.Projects()) },
		Ready: func(ctx context.Context) error {
			if err := manager.Restore(ctx); err != nil {
				return fmt.Errorf("restoring the instances: %w", err)
			}
			return nil
		},
		Connected: func(ctx context.Context) {
			if err := messenger.Sweep(ctx); err != nil {
				slog.Error("delivering what waited", "err", err)
			}
		},
	})
	// A clean exit (Ctrl-C, SIGTERM) stops the agents; a crash leaves them to
	// the next start's restore.
	quit, stopSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	go checklist.Run(quit)
	views := agents{Manager: manager, logins: login.terms, typed: messenger.Typed}
	handler := server.New(server.Options{
		Guard:        access.NewTokenGuard(token, access.LocalHosts(*port), *port),
		Home:         h,
		Config:       h,
		Events:       bus,
		Agents:       views,
		Messages:     messenger,
		Actions:      state.Actions(),
		Activity:     state.Activity(),
		Repos:        repos.Browser{Config: h, Git: gitCLI},
		Pipeline:     flow,
		Reviews:      flow,
		Requests:     flow,
		Harnesses:    login,
		Profiles:     login,
		Preferences:  userdata.PreferencesIn(userDir),
		Wiki:         wikis,
		Setup:        checklist,
		Maintenance:  maintenanceOf(running, engineRunner, engineImages, podman.Installed),
		Terminals:    views,
		ChiefOfStaff: chief,
		UI:           files,
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
func configEvent(c home.Change) events.Event {
	if c.Kind == "projects" {
		return events.Event{Type: "projects.changed", Data: struct{}{}}
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
