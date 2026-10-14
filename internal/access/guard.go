// Package access decides who may talk to Foreman's HTTP server. TokenGuard
// implements the rules in wiki/product/architecture/security.md: Host
// allowlist (DNS rebinding), Origin check on anything that changes state or
// opens a WebSocket (CSRF, WebSocket hijacking), and the per-install token.
package access

import (
	"crypto/subtle"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// TokenGuard guards a handler with the per-install access token.
type TokenGuard struct {
	token  string
	hosts  map[string]bool // allowed Host header values, with port
	cookie string          // cookie name; includes the port because cookies ignore ports
}

// NewTokenGuard accepts requests addressed to one of hosts (Host header values,
// with port) that carry token. The cookie name includes the port, because
// cookies ignore ports and two Foreman servers must not share one.
func NewTokenGuard(token string, hosts []string, port string) *TokenGuard {
	g := &TokenGuard{token: token, hosts: map[string]bool{}, cookie: "foreman_" + port}
	for _, h := range hosts {
		g.hosts[strings.ToLower(h)] = true
	}
	return g
}

// Wrap returns next behind the guard.
func (g *TokenGuard) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !g.hosts[strings.ToLower(r.Host)] {
			http.Error(w, "unknown host", http.StatusForbidden)
			return
		}
		if (changesState(r) || isWebSocket(r)) && !g.originAllowed(r) {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}

		// The token in the URL is exchanged for a cookie, then dropped from the
		// address bar and the history by a redirect to the same URL without it.
		if q := r.URL.Query(); q.Has("token") && r.Method == http.MethodGet {
			if !g.matches(q.Get("token")) {
				unauthorized(w)
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name:     g.cookie,
				Value:    g.token,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
			})
			q.Del("token")
			clean := *r.URL
			clean.RawQuery = q.Encode()
			http.Redirect(w, r, clean.RequestURI(), http.StatusFound)
			return
		}

		if !g.authorized(r) {
			unauthorized(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (g *TokenGuard) authorized(r *http.Request) bool {
	if c, err := r.Cookie(g.cookie); err == nil && g.matches(c.Value) {
		return true
	}
	// Non-browser clients (scripts, tests) send the token as a bearer header.
	auth := r.Header.Get("Authorization")
	if token, ok := strings.CutPrefix(auth, "Bearer "); ok && g.matches(token) {
		return true
	}
	return false
}

func (g *TokenGuard) matches(candidate string) bool {
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(g.token)) == 1
}

// originAllowed accepts a request without an Origin header: browsers always
// send one on cross-site state-changing requests and WebSocket upgrades, so a
// missing Origin means a non-browser client, which still needs the token.
func (g *TokenGuard) originAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" {
		return false
	}
	return g.hosts[strings.ToLower(u.Host)]
}

func changesState(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return true
}

func isWebSocket(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte("Foreman: open the URL printed in the terminal where `foreman` is running.\n"))
}

// LocalHosts are the Host values accepted when listening on this machine only.
func LocalHosts(port string) []string {
	return []string{
		net.JoinHostPort("127.0.0.1", port),
		net.JoinHostPort("localhost", port),
		net.JoinHostPort("::1", port),
	}
}
