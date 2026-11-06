package harness

// Status is what the UI knows of a harness (tygo.yaml); its credentials are
// a profile's (internal/profiles).
type Status struct {
	ID      string `json:"id"`      // claude-code, codex, pi
	Command string `json:"command"` // what it runs
	Newline string `json:"newline"` // the keystroke for a new line without sending
	// Connect is how a profile gets its credentials: "login", the harness's
	// own login in a terminal (Claude Code, Codex); "endpoint", a model
	// server in a form (Pi).
	Connect string `json:"connect"`
}

// Endpoint is a profile's model server and the model its agents run: the
// credentials of a harness that runs on one (Pi). The key is never sent back.
type Endpoint struct {
	// URL is the server's OpenAI-compatible API as this machine reaches it
	// (http://localhost:11434/v1); agents in containers reach localhost
	// through the host alias.
	URL           string `json:"url"`
	Model         string `json:"model"`
	ContextWindow int    `json:"contextWindow"` // tokens
	MaxTokens     int    `json:"maxTokens"`     // the most a reply may take
	APIKeySet     bool   `json:"apiKeySet"`
}

// EndpointChange sets a profile's endpoint; an empty APIKey keeps the one set.
type EndpointChange struct {
	URL           string `json:"url"`
	APIKey        string `json:"apiKey"`
	Model         string `json:"model"`
	ContextWindow int    `json:"contextWindow"`
	MaxTokens     int    `json:"maxTokens"`
}
