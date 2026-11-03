package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/brickpop/foreman/internal/agentlink"
	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/teamtools"
)

// fetch is the lead's fetch(): every remote of each code repo, fetched in the
// CEO's repo (the worktrees share its refs), so the team can merge in what
// was pushed upstream. A repo that fails leaves the others fetched; the call
// fails when any did, saying which (wiki/backlog/lead-fetch.md).
func (t tool) fetch(ctx context.Context) agentlink.Result {
	var lines []string
	failed, moved := false, false
	for _, r := range t.project.Repos {
		changes, err := t.w.Git.FetchRemotes(ctx, r.Path)
		var each []string
		for _, c := range changes {
			switch {
			case c.From == "":
				each = append(each, c.Branch+" new")
			case c.To == "":
				each = append(each, c.Branch+" gone upstream")
			default:
				each = append(each, fmt.Sprintf("%s %.7s..%.7s", c.Branch, c.From, c.To))
			}
		}
		moved = moved || len(changes) > 0
		switch {
		case errors.Is(err, git.ErrNoRemote):
			each = append(each, "no remote")
		case err != nil:
			failed = true
			each = append(each, "failed: "+strings.TrimPrefix(err.Error(), "git fetch: "))
		case len(changes) == 0:
			each = append(each, "up to date")
		}
		lines = append(lines, r.Name+": "+strings.Join(each, ", "))
	}
	text := strings.Join(lines, "\n")
	if moved {
		text += "\nMerge one into your branch with merge(\"origin/main\"), naming it."
	}
	if failed {
		return teamtools.Refusal("fetch_failed", "%s", text)
	}
	return agentlink.Result{Text: text}
}
