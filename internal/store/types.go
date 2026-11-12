package store

import "time"

// The rows, as the UI reads them too (tygo.yaml).

// Deployment is a team assigned to a project (wiki/product/models/deployment.md).
type Deployment struct {
	ID         string    `json:"id"` // team@project
	TeamID     string    `json:"teamId"`
	ProjectID  string    `json:"projectId"`
	Status     string    `json:"status"` // DeploymentActive or DeploymentStopped
	CreatedAt  time.Time `json:"createdAt"`
	StoppedAt  time.Time `json:"stoppedAt"`  // zero while active
	StopReason string    `json:"stopReason"` // why Foreman stopped it on its own; "" when the CEO did, or while active
}

// Instance is one agent running in one deployment
// (wiki/product/models/instance.md). Empty strings and zero values are
// stored as NULL.
type Instance struct {
	ID           string `json:"id"` // agent@project
	DeploymentID string `json:"deploymentId"`
	AgentID      string `json:"agentId"`
	Status       string `json:"status"` // asleep, queued, starting, running, idle, awaiting, done or stopped
	Task         string `json:"task"`   // the current task note, for a restart
	PID          int    `json:"-"`
	PtyID        string `json:"-"` // the engine's handle (e.g. the container id)
	SandboxPath  string `json:"sandboxPath"`
	Harness      string `json:"harness"`
	// Profile is the one its current run started on (Harness is its harness);
	// "" for a run from before it was kept.
	Profile          string `json:"profile"`
	HarnessSessionID string `json:"-"` // for the harness's own resume
	// PromptHash is the hash of the session prompt its current run started
	// with; "" for a run from before it was kept.
	PromptHash string `json:"-"`
	// Fresh: its next start begins a new conversation rather than continuing
	// its last one on the project; set at deploy, cleared once it starts.
	Fresh bool `json:"-"`
	// Behind says its run started on an older prompt than its team, project
	// and settings compose now: the instance manager works it out on each
	// list, never stored.
	Behind    bool      `json:"behind"`
	StartedAt time.Time `json:"startedAt"`
	Restarts  int       `json:"restarts"`
	// StopReason is why Foreman stopped it on its own (a start that failed, a
	// crash loop); "" otherwise, cleared on each start.
	StopReason string `json:"stopReason"`
	// Doing is what its harness last reported doing in this run, nil until
	// it says: kept by the instance manager in memory, never stored.
	Doing *Doing `json:"doing"`
}

// Doing is one line on what an instance is doing, from its harness's hooks
// (the instance.doing event).
type Doing struct {
	Line string    `json:"line"`
	At   time.Time `json:"at"`
}

// Request is something waiting for the CEO (wiki/product/models/request.md).
type Request struct {
	ID           string    `json:"id"`
	Kind         string    `json:"kind"` // approval, question, review, finding or blocked
	ProjectID    string    `json:"projectId"`
	DeploymentID string    `json:"deploymentId"` // "" with no instance
	InstanceID   string    `json:"instanceId"`   // who raised it, or who it is about; "" when Foreman raised it about no instance
	Summary      string    `json:"summary"`      // its title: one line
	Body         string    `json:"body"`         // the rest, when it needs more; "" for none
	Options      []string  `json:"options"`      // the first is the primary action
	Source       string    `json:"source"`       // agent, harness or foreman
	Status       string    `json:"status"`       // open, resolved, expired or dismissed
	Answer       string    `json:"answer"`
	CreatedAt    time.Time `json:"createdAt"`
	ResolvedAt   time.Time `json:"resolvedAt"`
	ReviewID     string    `json:"reviewId"` // for a review
	Links        []Link    `json:"links"`
}

// Link is an extra link on a request's panel (a branch, a wiki entry).
type Link struct {
	Label string `json:"label"`
	Href  string `json:"href"`
}

// TeamMessage is one message to an instance of a deployment, from a teammate,
// Foreman or the CEO (wiki/product/models/team-message.md).
type TeamMessage struct {
	ID           int64     `json:"id"`
	DeploymentID string    `json:"deploymentId"`
	Ref          string    `json:"ref"`          // msg-N, per deployment
	FromInstance string    `json:"fromInstance"` // an instance, or foreman / ceo
	ToInstance   string    `json:"toInstance"`
	Kind         string    `json:"kind"`      // ask, reply, handoff, contribute, notice or answer (the CEO's, to a request)
	InReplyTo    string    `json:"inReplyTo"` // a reply's ask; an answer's request
	Body         string    `json:"body"`
	ArtifactRef  string    `json:"artifactRef"` // a branch or a wiki path
	State        string    `json:"state"`       // queued, delivered, expired or failed
	Reason       string    `json:"reason"`      // why it expired or failed
	CreatedAt    time.Time `json:"createdAt"`
	DeliveredAt  time.Time `json:"deliveredAt"` // zero while queued
}

// TeamAction is one git tool an agent called (switch, commit, merge,
// abort_merge, discard, set_submodule, request_merge, withdraw_merge) and
// what it did (wiki/product/architecture/protocol.md#team-actions).
type TeamAction struct {
	ID           int64        `json:"id"`
	DeploymentID string       `json:"deploymentId"`
	Instance     string       `json:"instance"`
	Tool         string       `json:"tool"`
	Branch       string       `json:"branch"` // switch: to; merge: from
	Repos        []ActionRepo `json:"repos"`
	Error        string       `json:"error"` // the refusal the agent got; "" when it went through
	CreatedAt    time.Time    `json:"createdAt"`
}

// ActionRepo is what a git tool did in one repo.
type ActionRepo struct {
	Repo      string   `json:"repo"`
	Branch    string   `json:"branch"`    // checked out there after it
	Message   string   `json:"message"`   // commit: the message given for this repo
	Commit    string   `json:"commit"`    // a commit made
	Created   bool     `json:"created"`   // switch: the branch was new
	Conflicts []string `json:"conflicts"` // merge: files left with markers
	// request_merge: the merge request made (or refreshed) for this repo.
	MergeRequest string `json:"mergeRequest,omitempty"`
}

// WorkItem is a member's piece of work on its way to a target
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

// MergeRequest asks the CEO to merge a branch into a branch of the CEO's, its target
// (wiki/product/models/merge-request.md).
type MergeRequest struct {
	ID             string `json:"id"`
	ProjectID      string `json:"projectId"`
	DeploymentID   string `json:"deploymentId"`
	AuthorInstance string `json:"authorInstance"`
	Repo           string `json:"repo"` // one of the project's code repos
	Branch         string `json:"branch"`
	// Target is the branch the lead asked it to merge into (its repo's default
	// or any other but an agent's), fixed: the diff is always against it.
	Target string `json:"target"`
	// Landing is where it lands instead, "" for Target itself: a branch made
	// at Target's head on Merge (or one already there), named by the lead
	// (request_merge's land, under the project's prefixes) or by the CEO at
	// Merge.
	Landing       string    `json:"landing"`
	Entry         string    `json:"entry"` // the wiki card explaining the work
	State         string    `json:"state"` // open, merged, rejected or withdrawn
	ChecksStatus  string    `json:"checksStatus"`
	Conflicting   bool      `json:"conflicting"`
	ForgePRURL    string    `json:"forgePrUrl"`
	ForgePRNumber int       `json:"forgePrNumber"`
	Note          string    `json:"note"`
	CreatedAt     time.Time `json:"createdAt"`
	MergedAt      time.Time `json:"mergedAt"`
	RejectedAt    time.Time `json:"rejectedAt"`
	ReviewID      string    `json:"reviewId"`     // the review it is decided in
	ReviewedHead  string    `json:"reviewedHead"` // the branch head at the CEO's last Request changes
	// Where the branch stood when it left open (merged, dismissed, withdrawn):
	// its head, kept under refs/foreman/decided/<id>, and the commit it parted
	// from its target at ("" after a fast-forward), so its review still shows it.
	Head string `json:"head"`
	Base string `json:"base"`
}

// Review groups the merge requests one lead branch opened, one per repo, for
// the CEO to read and decide at once (wiki/product/models/review.md).
type Review struct {
	ID                 string    `json:"id"`
	ProjectID          string    `json:"projectId"`
	Branch             string    `json:"branch"`
	Title              string    `json:"title"` // the lead's, one line
	Body               string    `json:"body"`  // what the CEO should know about the work; "" for none
	CreatedAt          time.Time `json:"createdAt"`
	ChangesRequestedAt time.Time `json:"changesRequestedAt"` // zero unless it waits on the lead
	ChangesNote        string    `json:"changesNote"`        // the CEO's note with the last Request changes
	WithdrawnAt        time.Time `json:"withdrawnAt"`        // zero unless the lead or Foreman withdrew it
	WithdrawnReason    string    `json:"withdrawnReason"`
	SupersededBy       string    `json:"supersededBy"` // the review that holds all of it, when one does
}

// ReviewComment is the CEO's comment on a review: on a line of a repo's
// diff, or general (no repo).
type ReviewComment struct {
	ID        string    `json:"id"`
	ReviewID  string    `json:"reviewId"`
	Repo      string    `json:"repo"`
	Path      string    `json:"path"`
	Side      string    `json:"side"` // new (the branch) or old (the target); "" for a general one
	Line      int       `json:"line"` // on its side, in the commit it was written on
	Commit    string    `json:"commit"`
	Snippet   string    `json:"snippet"` // the line's text then
	Target    string    `json:"target"`  // the target its repo merged into then, which its old side was read on
	Body      string    `json:"body"`
	State     string    `json:"state"` // draft or sent
	CreatedAt time.Time `json:"createdAt"`
	SentAt    time.Time `json:"sentAt"`
}

// ActivityItem is one item of a project's activity: a message or a git action.
type ActivityItem struct {
	Message *TeamMessage `json:"message,omitempty"`
	Action  *TeamAction  `json:"action,omitempty"`
}

// ActivityPage is one page of a project's activity, newest first; Next is the
// cursor of the following page, "" when there is none.
type ActivityPage struct {
	Items []ActivityItem `json:"items"`
	Next  string         `json:"next"`
}
