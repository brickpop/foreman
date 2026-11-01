package messaging

// Draft is the instance.draft event's data: what the pane shows while
// messages wait on the CEO's draft in an agent's terminal, and then, once the
// line went out, until they are delivered.
type Draft struct {
	ID       string `json:"id"`
	Waiting  int    `json:"waiting"`  // 0: nothing waits (the notice goes away)
	OnDraft  bool   `json:"onDraft"`  // false: the draft went out, they wait for the agent's turn to end
	LeftMs   int64  `json:"leftMs"`   // the countdown
	Counting bool   `json:"counting"` // false while the CEO types, or paused
	Paused   bool   `json:"paused"`
}
