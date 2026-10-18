package harness

// Status is what the UI knows of a harness (tygo.yaml); its credentials are
// a profile's (internal/profiles).
type Status struct {
	ID      string `json:"id"`      // claude-code, …
	Command string `json:"command"` // what it runs
	Newline string `json:"newline"` // the keystroke for a new line without sending
}
