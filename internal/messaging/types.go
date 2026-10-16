package messaging

// Draft is the instance.draft event's data: what the pane shows while
// messages wait on the CEO's draft in an agent's terminal.
type Draft struct {
	ID       string `json:"id"`
	Waiting  int    `json:"waiting"`  // 0: nothing waits (the notice goes away)
	LeftMs   int64  `json:"leftMs"`   // the countdown
	Counting bool   `json:"counting"` // false while the CEO types, or paused
	Paused   bool   `json:"paused"`
}
