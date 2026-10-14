package events

// Event is one thing that happened; Type names it as in
// wiki/product/architecture/api.md#events.
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data" tstype:"unknown"`
}
