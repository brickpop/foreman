package store

import "time"

// The rows, as the UI reads them too (tygo.yaml).

// Deployment is a team assigned to a project with a goal
// (wiki/product/models/deployment.md).
type Deployment struct {
	ID        string    `json:"id"` // team@project
	TeamID    string    `json:"teamId"`
	ProjectID string    `json:"projectId"`
	Goal      string    `json:"goal"`
	Status    string    `json:"status"` // DeploymentActive or DeploymentStopped
	CreatedAt time.Time `json:"createdAt"`
	StoppedAt time.Time `json:"stoppedAt"` // zero while active
}

// Instance is one agent running in one deployment
// (wiki/product/models/instance.md). Empty strings and zero values are
// stored as NULL.
type Instance struct {
	ID               string    `json:"id"` // agent@project
	DeploymentID     string    `json:"deploymentId"`
	AgentID          string    `json:"agentId"`
	Status           string    `json:"status"` // queued, starting, running, idle, awaiting, done or stopped
	Task             string    `json:"task"`   // the current task note, for a restart
	PID              int       `json:"-"`
	PtyID            string    `json:"-"` // the engine's handle (e.g. the container id)
	SandboxPath      string    `json:"sandboxPath"`
	Harness          string    `json:"harness"`
	HarnessSessionID string    `json:"-"` // for the harness's own resume
	StartedAt        time.Time `json:"startedAt"`
	LastOutputAt     time.Time `json:"lastOutputAt"`
	Restarts         int       `json:"restarts"`
}

// Request is something waiting for the CEO (wiki/product/models/request.md).
type Request struct {
	ID             string    `json:"id"`
	Kind           string    `json:"kind"` // approval, question, review, finding or blocked
	ProjectID      string    `json:"projectId"`
	DeploymentID   string    `json:"deploymentId"` // "" for the Chief of Staff
	InstanceID     string    `json:"instanceId"`   // who raised it, or who it is about; "" for the Chief of Staff
	Summary        string    `json:"summary"`
	Options        []string  `json:"options"` // the first is the primary action
	Source         string    `json:"source"`  // agent, harness, foreman or cos
	Status         string    `json:"status"`  // open, resolved or expired
	Answer         string    `json:"answer"`
	CreatedAt      time.Time `json:"createdAt"`
	ResolvedAt     time.Time `json:"resolvedAt"`
	MergeRequestID string    `json:"mergeRequestId"` // for a review
	Links          []Link    `json:"links"`
}

// Link is an extra link on a request's panel (a branch, a wiki entry).
type Link struct {
	Label string `json:"label"`
	Href  string `json:"href"`
}

// TeamMessage is one message between two instances of a deployment
// (wiki/product/models/team-message.md).
type TeamMessage struct {
	ID           int64     `json:"id"`
	DeploymentID string    `json:"deploymentId"`
	FromInstance string    `json:"fromInstance"`
	ToInstance   string    `json:"toInstance"`
	Kind         string    `json:"kind"`       // ask, reply, handoff or contribute
	RequestRef   string    `json:"requestRef"` // req-N, per deployment
	Body         string    `json:"body"`
	ArtifactRef  string    `json:"artifactRef"` // a branch or a wiki path
	CreatedAt    time.Time `json:"createdAt"`
	DeliveredAt  time.Time `json:"deliveredAt"` // zero while queued for a busy target
}

// WorkItem is a member's piece of work on its way to main
// (wiki/product/models/work-item.md).
type WorkItem struct {
	ID             string `json:"id"`
	DeploymentID   string `json:"deploymentId"`
	AuthorInstance string `json:"authorInstance"`
	Branch         string `json:"branch"`
	CardRef        string `json:"cardRef"` // the board card it implements
	Stage          int    `json:"stage"`   // 0 in worktree, 1 handed to lead, 2 lead reviewing, 3 integrated
	Note           string `json:"note"`
	ChecksStatus   string `json:"checksStatus"`
	MergeRequestID string `json:"mergeRequestId"`
}

// MergeRequest asks the CEO to merge a branch into a repo's main
// (wiki/product/models/merge-request.md).
type MergeRequest struct {
	ID             string    `json:"id"`
	ProjectID      string    `json:"projectId"`    // "" for a Chief of Staff change to the workspace repo
	DeploymentID   string    `json:"deploymentId"` // "" for a Chief of Staff change
	AuthorInstance string    `json:"authorInstance"`
	Repo           string    `json:"repo"` // a code repo, "project" or "workspace"
	Branch         string    `json:"branch"`
	Entry          string    `json:"entry"` // the wiki card explaining the work
	State          string    `json:"state"` // open, merged or rejected
	ChecksStatus   string    `json:"checksStatus"`
	Conflicting    bool      `json:"conflicting"`
	ForgePRURL     string    `json:"forgePrUrl"`
	ForgePRNumber  int       `json:"forgePrNumber"`
	Note           string    `json:"note"`
	CreatedAt      time.Time `json:"createdAt"`
	MergedAt       time.Time `json:"mergedAt"`
	RejectedAt     time.Time `json:"rejectedAt"`
}
