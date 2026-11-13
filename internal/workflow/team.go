package workflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/brickpop/foreman/internal/agentlink"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/prompt"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/teamtools"
)

// The lead's team(): where each teammate stands, so it needn't ask (an ask
// interrupts, and wakes an asleep member). Only what Foreman itself knows
// crosses: the status, when the harness last reported, and each worktree's
// branch, read on the host. Nothing from inside a teammate's sandbox
// (wiki/product/architecture/protocol.md#team).

// team lists the caller's teammates in roster order, one line each.
func (t tool) team(ctx context.Context) agentlink.Result {
	if !t.c.Lead {
		return teamtools.Refusal("not_lead", "only the lead sees the team")
	}
	all, err := t.w.Teams.List(ctx)
	if err != nil {
		return teamtools.Refusal("internal", "%v", err)
	}
	insts := map[string]store.Instance{}
	for _, inst := range all {
		if inst.DeploymentID == t.c.Deployment.ID {
			insts[inst.AgentID] = inst
		}
	}
	var lines []string
	for _, id := range t.c.Team.Config.Roster() {
		if id == t.c.Instance.AgentID {
			continue
		}
		soul := t.c.Team.Config.Agents[id]
		line := fmt.Sprintf("%s, %s: ", soul.Name, prompt.RoleLabel(soul.Role))
		inst, ok := insts[id]
		if !ok {
			lines = append(lines, line+"not in this deployment (joined the team since it was deployed).")
			continue
		}
		line += t.w.status(inst)
		if repos := t.w.checkouts(ctx, t.project, inst.SandboxPath); len(repos) > 0 {
			line += "\n  " + strings.Join(repos, " · ")
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return agentlink.Result{Text: "You have no teammates."}
	}
	return agentlink.Result{Text: "Your teammates now:\n\n" + strings.Join(lines, "\n")}
}

// status is an instance's status, with when its harness last reported (kept
// in memory: none until it reports after a Foreman restart) or why Foreman
// stopped it.
func (w *Workflow) status(inst store.Instance) string {
	s := inst.Status
	if inst.Status == "stopped" && inst.StopReason != "" {
		s += " (" + inst.StopReason + ")"
	}
	if inst.Doing != nil && inst.Status != "stopped" && inst.Status != "asleep" {
		s += ", last report " + ago(w.Now().Sub(inst.Doing.At))
	}
	return s + "."
}

// checkouts are each worktree's branch, and what it holds not committed: a
// merge under way, or changes.
func (w *Workflow) checkouts(ctx context.Context, project home.Project, sandbox string) []string {
	var each []string
	for _, r := range worktreesIn(project, sandbox) {
		on, err := w.Git.Branch(ctx, r.dir)
		if err != nil {
			each = append(each, r.name+" unreadable: "+err.Error())
			continue
		}
		s := r.name + " " + onBranch(on)
		if merging, err := w.Git.Merging(ctx, r.dir); err == nil && merging {
			s += ", merge under way"
		} else if clean, err := w.Git.Clean(ctx, r.dir); err == nil && !clean {
			s += ", uncommitted changes"
		}
		each = append(each, s)
	}
	return each
}

// ago is a duration as the lead reads it: 40s ago, 12m ago, 3h ago.
func ago(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", max(0, int(d.Seconds())))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}
