package profiles

// Profile is a harness and its credentials, under a name (tygo.yaml).
type Profile struct {
	Name    string `json:"name"`
	Harness string `json:"harness"` // claude-code, …
	// Connected: its folder holds the harness's credentials (a login).
	Connected bool `json:"connected"`
}
