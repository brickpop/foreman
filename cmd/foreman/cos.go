package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/brickpop/foreman/internal/agentlink"
	"github.com/brickpop/foreman/internal/cos"
	"github.com/brickpop/foreman/internal/harness"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/instances"
	"github.com/brickpop/foreman/internal/profiles"
	"github.com/brickpop/foreman/internal/runner"
)

// chiefOfStaff is the Chief of Staff's session
// (wiki/product/features/chief-of-staff.md#how-it-runs): an ad-hoc terminal,
// like a profile's login, started and stopped with its page, on the default
// profile. It sees its own folder and the team files, read-only; it changes
// them only through propose_team (internal/cos).
type chiefOfStaff struct {
	*cos.CoS
	home     string // Foreman's home: its process is labelled with it
	dir      string // <home>/cos: its working folder and harness config
	teamsDir string // <home>/config/teams, read-only
	teams    func() []home.Team
	profiles profiles.Store
	adapters map[string]harness.Adapter
	images   runner.Images
	terms    *sessions
	agent    instances.AgentLink
	// blocked is nil when sessions can start, or why not (Podman isn't running).
	blocked func() error
	pause   time.Duration // between a line typed and its Enter (Messaging's)

	typing sync.Mutex // one line typed at a time
}

// Start implements server.ChiefOfStaff: it starts the session, or, when it
// runs, types the opener into it.
func (c *chiefOfStaff) Start(ctx context.Context, opener string) (string, error) {
	if c.terms.running(cos.Instance) {
		if opener != "" {
			c.tell(opener)
		}
		return cos.Instance, nil
	}
	if err := c.blocked(); err != nil {
		return "", fmt.Errorf("%w: %v", instances.ErrNotReady, err)
	}
	pr, err := c.profiles.Get(profiles.Default)
	if err != nil {
		return "", err
	}
	adapter := c.adapters[pr.Harness]
	if adapter == nil {
		return "", fmt.Errorf("the default profile's harness %q isn't supported", pr.Harness)
	}
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return "", err
	}
	session := harness.Session{
		Instance: cos.Instance, Sandbox: c.dir, Prompt: cos.Prompt(c.teams(), c.teamsDir),
		Login: c.profiles.Dir(profiles.Default),
		// Every visit continues the last conversation, and says what it is for.
		Continue: true, Opener: opener, OpenerOnContinue: true,
	}
	mounts := []runner.Mount{
		{Source: c.dir, Target: c.dir},
		{Source: c.teamsDir, Target: c.teamsDir, ReadOnly: true},
	}
	if err := os.MkdirAll(c.teamsDir, 0o755); err != nil {
		return "", err
	}
	if c.agent.Host != "" {
		mounts = append(mounts, runner.Mount{Source: c.agent.Host, Target: c.agent.Path, ReadOnly: true})
		session.Link = harness.Link{Program: c.agent.Path, Env: []string{
			"FOREMAN_AGENT_URL=" + c.agent.URL, "FOREMAN_AGENT_TOKEN=" + c.agent.Token(cos.Instance),
		}}
	}
	l, err := adapter.Launch(session)
	if err != nil {
		return "", err
	}
	if err := harness.WriteFiles(l.Files); err != nil {
		return "", err
	}
	image, err := c.images.Ensure(ctx, "")
	if err != nil {
		return "", err
	}
	err = c.terms.start(ctx, cos.Instance, runner.Spec{
		Name: instances.ProcessName(c.home, cos.Instance), Image: image, Command: l.Command, Dir: c.dir, Env: l.Env,
		Mounts: append(mounts, l.Mounts...),
		Labels: map[string]string{"foreman.cos": "true", instances.HomeLabel: c.home},
	})
	if errors.Is(err, errRunning) {
		return cos.Instance, nil // another tab started it meanwhile
	}
	return cos.Instance, err
}

// Stop implements server.ChiefOfStaff: the CEO left the page.
func (c *chiefOfStaff) Stop() error {
	c.Drop()
	return c.terms.stop(cos.Instance)
}

// Live reports whether the session runs: its token is good only then.
func (c *chiefOfStaff) Live() bool { return c.terms.running(cos.Instance) }

// tell types a line into the session and sends it, as the CEO would; the
// Enter after a pause, so the harness takes it as a key, not as a paste.
func (c *chiefOfStaff) tell(text string) {
	go func() {
		c.typing.Lock()
		defer c.typing.Unlock()
		if err := c.terms.input(cos.Instance, []byte(text)); err != nil {
			return
		}
		time.Sleep(c.pause)
		_ = c.terms.input(cos.Instance, []byte("\r"))
	}()
}

// agentCalls routes the agent port's tool calls by caller: the Chief of
// Staff's to its one tool, every instance's to the team tools. Ids can't
// clash: an instance's is agent@project.
type agentCalls struct {
	team agentlink.Tools
	cos  agentlink.Tools
}

func (a agentCalls) of(caller string) agentlink.Tools {
	if caller == cos.Instance {
		return a.cos
	}
	return a.team
}

func (a agentCalls) List(caller string) []agentlink.Tool { return a.of(caller).List(caller) }

func (a agentCalls) Call(ctx context.Context, caller, name string, args json.RawMessage) agentlink.Result {
	return a.of(caller).Call(ctx, caller, name, args)
}
