// Command prove runs the experiments of the Prove epic
// (wiki/backlog/epics/prove.md): small, standalone checks of the runtime facts
// the design depends on, run on the CEO's machines. Each check prints PASS or
// FAIL with what it observed. Throwaway: removed when the epic ends.
//
//	go run ./cmd/prove a start      # experiment A, first process
//	go run ./cmd/prove a reattach   # experiment A, a second process
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: prove <experiment> <step>   (see the justfile's prove-* recipes)")
		os.Exit(2)
	}
	r := &report{}
	switch os.Args[1] + " " + os.Args[2] {
	case "a start":
		proveAStart(r)
	case "a reattach":
		proveAReattach(r)
	case "c run":
		proveC(r)
	case "image build":
		proveImage(r)
	case "b login":
		proveBLogin(r)
	case "e run":
		proveE(r)
	case "storm run":
		n := 12
		if len(os.Args) > 3 {
			fmt.Sscan(os.Args[3], &n)
		}
		proveStorm(r, n)
	case "d run":
		seconds := 330
		if len(os.Args) > 3 {
			fmt.Sscan(os.Args[3], &seconds)
		}
		proveD(r, seconds)
	case "b race":
		variant, n := "file", 12
		if len(os.Args) > 3 {
			variant = os.Args[3]
		}
		if len(os.Args) > 4 {
			fmt.Sscan(os.Args[4], &n)
		}
		proveBRace(r, variant, n)
	default:
		fmt.Fprintf(os.Stderr, "unknown experiment step %q\n", os.Args[1]+" "+os.Args[2])
		os.Exit(2)
	}
	os.Exit(r.done())
}

// report prints each check as it happens and sums them up.
type report struct{ passed, failed int }

func (r *report) pass(format string, a ...any) {
	r.passed++
	fmt.Printf("PASS  "+format+"\n", a...)
}

func (r *report) fail(format string, a ...any) {
	r.failed++
	fmt.Printf("FAIL  "+format+"\n", a...)
}

func (r *report) info(format string, a ...any) { fmt.Printf("      "+format+"\n", a...) }

// check records a pass when err is nil, a failure otherwise; it returns ok.
func (r *report) check(err error, what string) bool {
	if err != nil {
		r.fail("%s: %v", what, err)
		return false
	}
	r.pass("%s", what)
	return true
}

func (r *report) done() int {
	fmt.Printf("\n%d passed, %d failed\n", r.passed, r.failed)
	if r.failed > 0 {
		return 1
	}
	return 0
}
