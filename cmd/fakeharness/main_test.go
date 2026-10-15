package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestFakeHarness(t *testing.T) {
	var out bytes.Buffer
	code := run(strings.NewReader("hello\n\nspam 3\nexit 7\nnever reached\n"), &out)
	if code != 7 {
		t.Errorf("exit code %d, want 7", code)
	}
	got := out.String()
	for _, want := range []string{"\x1b[32mready", "you said: hello", "line 1\nline 2\nline 3\n", "bye (7)"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "never reached") {
		t.Error("kept reading after exit")
	}
}

func TestFakeHarnessEndOfInput(t *testing.T) {
	if code := run(strings.NewReader("hi\n"), &bytes.Buffer{}); code != 0 {
		t.Errorf("exit code %d at end of input, want 0", code)
	}
}

// A trailing `\` continues the input on the next line, as in Claude Code.
func TestFakeHarnessContinuesLines(t *testing.T) {
	var out bytes.Buffer
	run(strings.NewReader("first\\\nsecond\\\nthird\nalone\n"), &out)
	got := out.String()
	if !strings.Contains(got, "you said: first\nsecond\nthird\n") || !strings.Contains(got, "you said: alone\n") {
		t.Errorf("got %q", got)
	}
	if strings.Count(got, "you said:") != 2 {
		t.Errorf("a continued line was answered on its own: %q", got)
	}
}
