package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// EndpointAdapter is an adapter whose profiles are a model server rather
// than a login (Pi): Connect is a form, and Login isn't used.
type EndpointAdapter interface {
	Adapter
	// Endpoint is the one set in the profile folder dir; zero when none.
	Endpoint(dir string) (Endpoint, error)
	// SetEndpoint checks and saves it (ErrInvalid).
	SetEndpoint(dir string, c EndpointChange) error
	// Detect lists the models the server at url offers, with key, or the
	// key set in dir when key is "".
	Detect(ctx context.Context, dir, url, key string) ([]string, error)
}

// ErrInvalid means an endpoint isn't one.
var ErrInvalid = errors.New("harness: invalid endpoint")

// CheckURL refuses a server URL that isn't http:// or https:// with a host.
func CheckURL(s string) error {
	if u, err := url.Parse(s); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("%w: the URL must be http:// or https://, with a host", ErrInvalid)
	}
	return nil
}

// Check refuses an endpoint agents can't run on.
func (c EndpointChange) Check() error {
	if err := CheckURL(c.URL); err != nil {
		return err
	}
	switch {
	case strings.TrimSpace(c.Model) == "":
		return fmt.Errorf("%w: choose a model", ErrInvalid)
	case c.ContextWindow <= 0 || c.MaxTokens <= 0:
		return fmt.Errorf("%w: the context window and the most a reply may take are tokens, above 0", ErrInvalid)
	case c.MaxTokens > c.ContextWindow:
		return fmt.Errorf("%w: a reply can't take more than the context window", ErrInvalid)
	}
	return nil
}

// Models lists the models an OpenAI-compatible server offers (GET
// <url>/models), in the order it sends them.
func Models(ctx context.Context, client *http.Client, base, key string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the server answered %s", resp.Status)
	}
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&list); err != nil {
		return nil, fmt.Errorf("not a list of models: %v", err)
	}
	ids := []string{}
	for _, m := range list.Data {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	return ids, nil
}
