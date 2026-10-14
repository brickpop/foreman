package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// The start storm: what makes Podman fail when many containers start together?
// Experiment B saw starts fail (`ping_group_range … are all the IDs mapped`,
// `uid_map: Operation not permitted`) and creates lose their API connection
// (EOF) with 12 at once, even with starts serialized. This isolates it with
// tiny alpine containers, no Claude: all at once vs one at a time, with and
// without the keep-id user mapping.

func proveStorm(r *report, n int) {
	p := connectPodman(r)
	if p == nil {
		return
	}
	if !r.check(p.pull(cImage), "pulled "+cImage) {
		return
	}
	for _, c := range []struct {
		label    string
		parallel bool
		userns   *namespace
	}{
		{"one at a time, default mapping", false, nil},
		{"one at a time, keep-id", false, keepID},
		{"all at once, default mapping", true, nil},
		{"all at once, keep-id", true, keepID},
	} {
		removeLabelled(r, p, "storm")
		created, started, exited, errs, took := storm(p, n, c.parallel, c.userns)
		if created == n && started == n && exited == n {
			r.pass("%s: %d/%d created, started and ran (%v)", c.label, n, n, took.Round(time.Millisecond))
		} else {
			r.fail("%s: created %d, started %d, ran %d of %d (%v)", c.label, created, started, exited, n, took.Round(time.Millisecond))
			for msg, count := range errs {
				r.info("  %d× %s", count, msg)
			}
		}
	}
	removeLabelled(r, p, "storm")
}

// storm creates and starts n containers (in parallel or one after another),
// waits for them, and counts each stage's successes and the distinct errors.
func storm(p *podman, n int, parallel bool, userns *namespace) (created, started, exited int, errs map[string]int, took time.Duration) {
	errs = map[string]int{}
	var mu sync.Mutex
	fail := func(stage string, err error) {
		mu.Lock()
		defer mu.Unlock()
		msg := err.Error()
		if i := strings.Index(msg, `"message":"`); i >= 0 { // keep the runtime's own message
			msg = msg[i+11:]
			if j := strings.Index(msg, `"`); j >= 0 {
				msg = msg[:j]
			}
		}
		errs[stage+": "+msg]++
	}
	ids := make([]string, n)
	begin := time.Now()
	one := func(i int) {
		id, err := p.create(createSpec{
			Name:    fmt.Sprintf("foreman-prove-storm-%d-%s", i, newToken()[:6]),
			Image:   cImage,
			Command: []string{"/bin/sh", "-c", "sleep 2; echo ok"},
			Labels:  map[string]string{"foreman.prove": "storm"},
			Userns:  userns,
		})
		if err != nil {
			fail("create", err)
			return
		}
		mu.Lock()
		created++
		ids[i] = id
		mu.Unlock()
		if err := p.start(id); err != nil {
			fail("start", err)
			return
		}
		mu.Lock()
		started++
		mu.Unlock()
	}
	if parallel {
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) { defer wg.Done(); one(i) }(i)
		}
		wg.Wait()
	} else {
		for i := 0; i < n; i++ {
			one(i)
		}
	}
	for _, id := range ids {
		if id == "" {
			continue
		}
		// Only containers that printed "ok" ran: waiting on one that never started
		// returns at once with code 0.
		if code, err := p.wait(id); err == nil && code == 0 && startedOK(p, id) {
			exited++
		}
		_ = p.remove(id)
	}
	return created, started, exited, errs, time.Since(begin)
}

// startedOK reports whether the container printed its "ok", i.e. it really ran.
func startedOK(p *podman, id string) bool {
	out, _ := p.logs(id)
	return strings.Contains(out, "ok")
}
