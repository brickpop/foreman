package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Experiment C (wiki/backlog/prove-c-agent-port.md): can a process in a
// rootless container reach a port Foreman opens only on the host's loopback,
// through host.containers.internal? Each container gets its own token, as
// foreman-agent will; the server must tell them apart and refuse a wrong one.

const (
	cImage      = "docker.io/library/alpine:latest"
	cContainers = 12
	cRequests   = 10
)

func proveC(r *report) {
	p := connectPodman(r)
	if p == nil {
		return
	}
	removeLabelled(r, p, "c")
	if !r.check(p.pull(cImage), "pulled "+cImage) {
		return
	}

	// The agent port: loopback only.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if !r.check(err, "opened a port on the host's loopback only") {
		return
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	r.info("listening on %s", ln.Addr())

	tokens := map[string]int{} // token → container number
	var mu sync.Mutex
	seen := map[int]int{} // container number → accepted requests
	refused := 0
	var first, last time.Time
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		token := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
		mu.Lock()
		defer mu.Unlock()
		n, ok := tokens[token]
		if !ok {
			refused++
			http.Error(w, "unknown token", http.StatusUnauthorized)
			return
		}
		now := time.Now()
		if first.IsZero() {
			first = now
		}
		last = now
		seen[n]++
		fmt.Fprintf(w, "hello container %d\n", n)
	})}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	// One container first, with a wrong token as well as its own.
	tok := newToken()
	tokens[tok] = 0
	script := fmt.Sprintf(`set -e
wget -q -O - --header "Authorization: Bearer %s" http://host.containers.internal:%d/
if wget -q -O - --header "Authorization: Bearer wrong" http://host.containers.internal:%d/ 2>/dev/null; then echo WRONG-TOKEN-ACCEPTED; exit 3; fi
echo done`, tok, port, port)
	code, logs, err := runToEnd(p, cImage, "c", script)
	if r.check(err, "ran a container calling the loopback port through host.containers.internal") {
		if code == 0 && strings.Contains(logs, "hello container 0") {
			r.pass("the container reached the host's loopback port and was recognised by its token")
		} else {
			r.fail("the container couldn't reach it (exit %d): %s", code, strings.TrimSpace(logs))
			r.info("on FAIL see the card: bind to the address Podman's network reaches instead")
			return
		}
	}
	mu.Lock()
	if refused == 1 {
		r.pass("a wrong token was refused")
	} else {
		r.fail("wrong-token requests refused: %d, want 1", refused)
	}
	mu.Unlock()

	// The LAN must not reach it: the port is bound to loopback.
	if ip := lanAddress(); ip == "" {
		r.info("no LAN address found; skipping the LAN check")
	} else if conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), time.Second); err == nil {
		conn.Close()
		r.fail("the port is reachable on the LAN address %s", ip)
	} else {
		r.pass("the port is NOT reachable on the LAN address %s", ip)
	}

	// Many containers at once, each with its own token.
	mu.Lock()
	first, last = time.Time{}, time.Time{}
	mu.Unlock()
	var wg sync.WaitGroup
	errs := make(chan string, cContainers)
	for n := 1; n <= cContainers; n++ {
		tok := newToken()
		mu.Lock()
		tokens[tok] = n
		mu.Unlock()
		script := fmt.Sprintf(`for i in $(seq %d); do wget -q -O /dev/null --header "Authorization: Bearer %s" http://host.containers.internal:%d/ || exit 1; done`, cRequests, tok, port)
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			code, logs, err := runToEnd(p, cImage, "c", script)
			if err != nil || code != 0 {
				errs <- fmt.Sprintf("container %d: exit %d, %v: %s", n, code, err, strings.TrimSpace(logs))
			}
		}(n)
	}
	wg.Wait()
	close(errs)
	failed := 0
	for e := range errs {
		failed++
		r.info("%s", e)
	}
	mu.Lock()
	defer mu.Unlock()
	total := 0
	for n := 1; n <= cContainers; n++ {
		total += seen[n]
	}
	if failed == 0 && total == cContainers*cRequests {
		r.pass("%d containers at once, %d requests each, every one attributed to the right container", cContainers, cRequests)
	} else {
		r.fail("%d of %d containers failed; %d of %d requests arrived", failed, cContainers, total, cContainers*cRequests)
	}
	if total > 1 {
		r.info("about %v per request (wall clock over all of them, each a new wget process and connection)",
			(last.Sub(first) / time.Duration(total)).Round(time.Microsecond))
	}
}

// runToEnd runs script in a fresh labelled container of image, waits for it to
// exit, and returns its exit code and output; the container is removed.
func runToEnd(p *podman, image, label, script string) (int, string, error) {
	id, err := p.create(createSpec{
		Name:    fmt.Sprintf("foreman-prove-%s-%s", label, newToken()[:8]),
		Image:   image,
		Command: []string{"/bin/sh", "-c", script},
		Labels:  map[string]string{"foreman.prove": label},
	})
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = p.remove(id) }()
	if err := p.start(id); err != nil {
		return 0, "", err
	}
	code, err := p.wait(id)
	if err != nil {
		return 0, "", err
	}
	logs, _ := p.logs(id)
	return code, logs, nil
}

func removeLabelled(r *report, p *podman, value string) {
	if old, _ := p.listByLabel("foreman.prove", value); len(old) > 0 {
		for _, id := range old {
			_ = p.remove(id)
		}
		r.info("removed %d leftover container(s) from an earlier run", len(old))
	}
}

func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// lanAddress is the host's first non-loopback IPv4 address, if any.
func lanAddress() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return ""
}
