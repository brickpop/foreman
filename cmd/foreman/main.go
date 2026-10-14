// Command foreman serves a workspace: `foreman [<workspace-dir>]`.
// See wiki/product/models/workspace.md#starting-foreman.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/brickpop/foreman/internal/access"
	"github.com/brickpop/foreman/internal/server"
	"github.com/brickpop/foreman/internal/workspace"
	"github.com/brickpop/foreman/ui"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "foreman:", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	flags := flag.NewFlagSet("foreman", flag.ContinueOnError)
	yes := flags.Bool("yes", false, "create .foreman/ without asking if the folder isn't a workspace yet")
	noOpen := flags.Bool("no-open", false, "print the URL but don't open the browser")
	port := flags.String("port", "8888", "port for the web UI")
	newToken := flags.Bool("new-token", false, "replace the access token (logs every browser out)")
	uiDir := flags.String("ui-dir", "", "development: serve the web UI from this folder instead of the embedded build")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "Usage: foreman [<workspace-dir>] [flags]")
		flags.PrintDefaults()
	}
	dir, err := parseArgs(flags, args)
	if err != nil {
		return err
	}

	ws, err := openOrCreate(dir, *yes, stdin, stdout)
	if err != nil {
		return err
	}
	token, err := ws.AccessToken(*newToken)
	if err != nil {
		return fmt.Errorf("access token: %w", err)
	}

	// Listen before printing the URL, so a busy port fails before the browser opens.
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", *port))
	if err != nil {
		return err
	}
	url := fmt.Sprintf("http://127.0.0.1:%s/?token=%s", *port, token)
	fmt.Fprintf(stdout, "Foreman is serving %s\nOpen: %s\n", ws.Root(), url)
	if !*noOpen {
		openBrowser(url, stdout)
	}

	files := ui.UI()
	if *uiDir != "" {
		files = os.DirFS(*uiDir)
	}
	// The composition root: the only place that picks concrete implementations.
	handler := server.New(server.Options{
		Guard:     access.NewTokenGuard(token, access.LocalHosts(*port), *port),
		Workspace: ws,
		UI:        files,
	})
	return http.Serve(listener, handler)
}

// parseArgs accepts flags before and after the workspace folder (the flag
// package alone stops at the first positional argument).
func parseArgs(flags *flag.FlagSet, args []string) (string, error) {
	var positional []string
	for {
		if err := flags.Parse(args); err != nil {
			return "", err
		}
		if flags.NArg() == 0 {
			break
		}
		positional = append(positional, flags.Arg(0))
		args = flags.Args()[1:]
	}
	switch len(positional) {
	case 0:
		return ".", nil
	case 1:
		return positional[0], nil
	default:
		return "", errors.New("expected at most one workspace folder")
	}
}

func openOrCreate(dir string, yes bool, stdin io.Reader, stdout io.Writer) (*workspace.Workspace, error) {
	ws, err := workspace.Open(dir)
	if !errors.Is(err, workspace.ErrNotInitialized) {
		return ws, err
	}
	if !yes {
		fmt.Fprintf(stdout, "%s isn't a Foreman workspace yet. Create %s/ here? [y/N] ", dir, workspace.DirName)
		answer, _ := bufio.NewReader(stdin).ReadString('\n')
		if strings.ToLower(strings.TrimSpace(answer)) != "y" {
			return nil, errors.New("not a workspace; nothing was created")
		}
	}
	return workspace.Init(dir)
}

// openBrowser is best effort: the URL is printed either way.
func openBrowser(url string, stdout io.Writer) {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	if err := exec.Command(name, url).Start(); err != nil {
		fmt.Fprintln(stdout, "(couldn't open a browser; use the URL above)")
	}
}
