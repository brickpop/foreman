package access

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	token = "s3cret-token-value"
	port  = "8888"
	host  = "127.0.0.1:8888"
)

// guarded is a handler behind the guard that answers 200 "reached" when let through.
func guarded() http.Handler {
	reached := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("reached"))
	})
	return NewTokenGuard(token, LocalHosts(port), port).Wrap(reached)
}

func do(t *testing.T, h http.Handler, method, target string, mod func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, target, nil)
	r.Host = host
	if mod != nil {
		mod(r)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func withCookie(value string) func(*http.Request) {
	return func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "foreman_" + port, Value: value}) }
}

func TestNoTokenIsRejected(t *testing.T) {
	w := do(t, guarded(), "GET", "/", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", w.Code)
	}
	if !strings.Contains(w.Body.String(), "URL printed in the terminal") {
		t.Errorf("401 page doesn't tell the CEO what to do: %q", w.Body.String())
	}
}

func TestURLTokenBecomesCookieAndLeavesTheURL(t *testing.T) {
	w := do(t, guarded(), "GET", "/projects/ledger?tab=board&token="+token, nil)
	if w.Code != http.StatusFound {
		t.Fatalf("got %d, want a redirect", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/projects/ledger?tab=board" {
		t.Errorf("redirect to %q: token not stripped or other params lost", loc)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cookies))
	}
	c := cookies[0]
	if c.Name != "foreman_"+port || c.Value != token || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/" {
		t.Errorf("cookie %+v isn't HttpOnly, SameSite=Strict, path /, named per port", c)
	}
}

func TestWrongURLTokenSetsNothing(t *testing.T) {
	w := do(t, guarded(), "GET", "/?token=guess", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", w.Code)
	}
	if len(w.Result().Cookies()) != 0 {
		t.Error("a wrong token still set a cookie")
	}
}

func TestCookieAndBearerAreAccepted(t *testing.T) {
	h := guarded()
	if w := do(t, h, "GET", "/anything", withCookie(token)); w.Code != http.StatusOK {
		t.Errorf("cookie: got %d", w.Code)
	}
	bearer := func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token) }
	w := do(t, h, "GET", "/anything", bearer)
	if w.Code != http.StatusOK {
		t.Fatalf("bearer: got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "reached") {
		t.Errorf("bearer request didn't reach the handler: %q", w.Body.String())
	}
}

func TestWrongCookieOrPrefixIsRejected(t *testing.T) {
	h := guarded()
	for name, mod := range map[string]func(*http.Request){
		"wrong cookie":          withCookie("nope"),
		"prefix of token":       withCookie(token[:5]),
		"token plus suffix":     withCookie(token + "x"),
		"cookie of other port":  func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "foreman_9999", Value: token}) },
		"bearer without scheme": func(r *http.Request) { r.Header.Set("Authorization", token) },
	} {
		if w := do(t, h, "GET", "/anything", mod); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: got %d, want 401", name, w.Code)
		}
	}
}

// DNS rebinding: a hostile domain resolving to 127.0.0.1 arrives with its own Host.
func TestUnknownHostIsRejectedEvenWithToken(t *testing.T) {
	h := guarded()
	for _, bad := range []string{"evil.example:8888", "127.0.0.1:9999", "127.0.0.1", ""} {
		w := do(t, h, "GET", "/anything", func(r *http.Request) {
			r.Host = bad
			withCookie(token)(r)
		})
		if w.Code != http.StatusForbidden {
			t.Errorf("Host %q: got %d, want 403", bad, w.Code)
		}
	}
	for _, good := range LocalHosts(port) {
		w := do(t, h, "GET", "/anything", func(r *http.Request) {
			r.Host = good
			withCookie(token)(r)
		})
		if w.Code != http.StatusOK {
			t.Errorf("Host %q: got %d, want 200", good, w.Code)
		}
	}
}

// CSRF: a website the CEO visits posts to Foreman; SameSite=Strict already keeps
// the cookie away, and the Origin check is the second line of defence.
func TestForeignOriginCannotChangeState(t *testing.T) {
	h := guarded()
	for _, origin := range []string{"https://evil.example", "http://evil.example", "null", "http://127.0.0.1:9999", "https://127.0.0.1:8888"} {
		w := do(t, h, "POST", "/api/anything", func(r *http.Request) {
			r.Header.Set("Origin", origin)
			withCookie(token)(r)
		})
		if w.Code != http.StatusForbidden {
			t.Errorf("POST from %q: got %d, want 403", origin, w.Code)
		}
	}
	w := do(t, h, "POST", "/api/anything", func(r *http.Request) {
		r.Header.Set("Origin", "http://"+host)
		withCookie(token)(r)
	})
	if w.Code == http.StatusForbidden {
		t.Error("POST from Foreman's own origin was rejected")
	}
}

// WebSockets ignore the same-origin policy, so the upgrade itself is checked.
func TestForeignOriginCannotOpenWebSocket(t *testing.T) {
	w := do(t, guarded(), "GET", "/ws/pty/x", func(r *http.Request) {
		r.Header.Set("Upgrade", "websocket")
		r.Header.Set("Origin", "https://evil.example")
		withCookie(token)(r)
	})
	if w.Code != http.StatusForbidden {
		t.Errorf("got %d, want 403", w.Code)
	}
}
