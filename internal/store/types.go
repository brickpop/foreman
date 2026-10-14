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
