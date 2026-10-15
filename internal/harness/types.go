package harness

// Status is a harness's state, for Settings → Harnesses (tygo.yaml).
type Status struct {
	ID        string `json:"id"`        // claude-code, …
	Command   string `json:"command"`   // what it runs when the settings name none
	Connected bool   `json:"connected"` // it has a login
}
