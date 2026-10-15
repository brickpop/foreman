// Command fakeharness stands in for a coding harness in tests and development:
// it prints a coloured banner, shows a prompt, and answers each input line.
// Commands: `spam N` prints N numbered lines, `exit N` exits with code N.
// Anything else is echoed back; a line ending in `\` continues on the next
// (as in Claude Code), so one input can span lines. An argument is the
// session's opener, answered first (as Claude Code's prompt argument). It writes plain "\n", as real programs do (the
// terminal adds the "\r"). It grows with the features that need it (calling
// team tools, going idle); see wiki/product/architecture/tech-stack.md.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func main() {
	os.Exit(run(strings.Join(os.Args[1:], " "), os.Stdin, os.Stdout))
}

const prompt = "\x1b[38;5;214m❯\x1b[0m "

// run is the harness loop; it returns the exit code.
func run(opener string, in io.Reader, out io.Writer) int {
	fmt.Fprint(out, "\x1b[1mfake harness\x1b[0m · \x1b[32mready\x1b[0m\n"+prompt)
	if opener != "" {
		fmt.Fprintf(out, "%s\nyou said: %s\n%s", opener, opener, prompt)
	}
	lines := bufio.NewScanner(in)
	var held []string // the lines so far of an input continued with a trailing `\`
	for lines.Scan() {
		if text, more := strings.CutSuffix(lines.Text(), "\\"); more {
			held = append(held, text)
			continue
		}
		line := strings.TrimSpace(strings.Join(append(held, lines.Text()), "\n"))
		held = nil
		cmd, arg, _ := strings.Cut(line, " ")
		switch cmd {
		case "exit":
			code, _ := strconv.Atoi(arg)
			fmt.Fprintf(out, "bye (%d)\n", code)
			return code
		case "spam":
			n, _ := strconv.Atoi(arg)
			for i := 1; i <= n; i++ {
				fmt.Fprintf(out, "line %d\n", i)
			}
		case "":
		default:
			fmt.Fprintf(out, "you said: %s\n", line)
		}
		fmt.Fprint(out, prompt)
	}
	return 0
}
