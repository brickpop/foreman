package cos

import "github.com/brickpop/foreman/internal/workspace"

// Proposal is a team as the Chief of Staff proposes it, shown to the CEO
// until they create it, it is replaced, or the session ends.
type Proposal struct {
	ID   int                  `json:"id"`
	Team workspace.TeamConfig `json:"team"`
	// Current is the team as it is now, so the page marks what changes;
	// null for a new team.
	Current *workspace.TeamConfig `json:"current"`
}
