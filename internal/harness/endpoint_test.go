package harness

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestEndpointCheck(t *testing.T) {
	good := EndpointChange{URL: "http://localhost:11434/v1", Model: "qwen3", ContextWindow: 262144, MaxTokens: 131072}
	if err := good.Check(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*EndpointChange){
		"no scheme":        func(c *EndpointChange) { c.URL = "localhost:11434/v1" },
		"another scheme":   func(c *EndpointChange) { c.URL = "file:///etc/passwd" },
		"no host":          func(c *EndpointChange) { c.URL = "http:///v1" },
		"no model":         func(c *EndpointChange) { c.Model = "  " },
		"no context":       func(c *EndpointChange) { c.ContextWindow = 0 },
		"negative reply":   func(c *EndpointChange) { c.MaxTokens = -1 },
		"reply over limit": func(c *EndpointChange) { c.MaxTokens = c.ContextWindow + 1 },
	} {
		c := good
		change(&c)
		if err := c.Check(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestModelsListsTheServers(t *testing.T) {
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/v1/models":
			w.Write([]byte(`{"object":"list","data":[{"id":"qwen3:30b"},{"id":""},{"id":"llama"}]}`))
		case "/bad/models":
			w.Write([]byte(`<html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	ids, err := Models(context.Background(), srv.Client(), srv.URL+"/v1/", "sk-1")
	if err != nil || !reflect.DeepEqual(ids, []string{"qwen3:30b", "llama"}) || auth != "Bearer sk-1" {
		t.Errorf("models %v %v, auth %q", ids, err, auth)
	}
	if _, err := Models(context.Background(), srv.Client(), srv.URL+"/v1", ""); err != nil || auth != "" {
		t.Errorf("without a key: %v, auth %q", err, auth)
	}
	for _, base := range []string{srv.URL + "/missing", srv.URL + "/bad"} {
		if _, err := Models(context.Background(), srv.Client(), base, ""); err == nil {
			t.Errorf("%s: no error", base)
		}
	}
}
