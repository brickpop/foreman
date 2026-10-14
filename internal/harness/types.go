package harness

// Status is a harness's state, for Settings → Harnesses (tygo.yaml).
type Status struct {
	ID        string `json:"id"`        // claude-code, …
	Connected bool   `json:"connected"` // it has a login
}
