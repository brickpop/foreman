// Package agentlink is the agent link (wiki/product/architecture/building-blocks.md#4-agent-link-agentlink--foreman-agent):
// the agent port foreman-agent reaches from inside every container, carrying
// team-tool calls and harness hook events to Foreman. A caller is known only by
// its token, never by anything it says about itself.
package agentlink

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// Tool is one team tool as foreman-agent offers it to the harness.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"` // a JSON Schema of type object
}

// Result is a tool call's answer, as text for the agent; Error marks a refusal
// the agent should read and correct (not a transport failure).
type Result struct {
	Text  string `json:"text"`
	Error bool   `json:"error"`
}

// Tools serves the team tools to one instance: only those its role allows.
type Tools interface {
	List(instance string) []Tool
	Call(ctx context.Context, instance, name string, args json.RawMessage) Result
}

// Hooks hears an instance's harness hook events.
type Hooks interface {
	Hook(instance, event string, payload []byte)
}

// Link serves the agent port.
type Link struct {
	secret []byte
	live   func(instance string) bool // a token is good only while its instance runs
	tools  Tools
	hooks  Hooks
}

// New returns the link. secret signs the tokens; it survives Foreman
// restarts, so containers that kept running keep their tokens.
func New(secret []byte, live func(instance string) bool, tools Tools, hooks Hooks) *Link {
	return &Link{secret: secret, live: live, tools: tools, hooks: hooks}
}

// Token is the instance's identity on the agent port, passed in its
// container's environment.
func (l *Link) Token(instance string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(instance)) + "." + l.sign(instance)
}

func (l *Link) sign(instance string) string {
	mac := hmac.New(sha256.New, l.secret)
	mac.Write([]byte(instance))
	return hex.EncodeToString(mac.Sum(nil))
}

// caller is the instance a request's token names, if the token is genuine
// and the instance runs.
func (l *Link) caller(r *http.Request) (string, bool) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		return "", false
	}
	name, sig, ok := strings.Cut(token, ".")
	if !ok {
		return "", false
	}
	id, err := base64.RawURLEncoding.DecodeString(name)
	if err != nil {
		return "", false
	}
	instance := string(id)
	if !hmac.Equal([]byte(sig), []byte(l.sign(instance))) || !l.live(instance) {
		return "", false
	}
	return instance, true
}

// maxBody caps what an agent may send in one request.
const maxBody = 1 << 20

// Handler serves the agent port: GET /tools, POST /tools/{name} (the
// arguments as the body), POST /hooks/{event} (the hook's payload).
func (l *Link) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tools", func(w http.ResponseWriter, r *http.Request) {
		instance, _ := r.Context().Value(callerKey{}).(string)
		writeJSON(w, l.tools.List(instance))
	})
	mux.HandleFunc("POST /tools/{name}", func(w http.ResponseWriter, r *http.Request) {
		instance, _ := r.Context().Value(callerKey{}).(string)
		args, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
		if err != nil {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		writeJSON(w, l.tools.Call(r.Context(), instance, r.PathValue("name"), args))
	})
	mux.HandleFunc("POST /hooks/{event}", func(w http.ResponseWriter, r *http.Request) {
		instance, _ := r.Context().Value(callerKey{}).(string)
		payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
		if err != nil {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		l.hooks.Hook(instance, r.PathValue("event"), payload)
		w.WriteHeader(http.StatusNoContent)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		instance, ok := l.caller(r)
		if !ok {
			http.Error(w, "unknown caller", http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), callerKey{}, instance)))
	})
}

type callerKey struct{}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
