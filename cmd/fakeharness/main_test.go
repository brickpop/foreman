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
