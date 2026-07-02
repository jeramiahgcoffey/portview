// Package cli implements portview's non-TUI subcommands: list, kill, and
// open. These make portview scriptable — `portview list --json` in a pipeline,
// `portview kill 3000` in an alias — without opening the full-screen UI.
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	"github.com/jeramiahgcoffey/portview/internal/action"
	"github.com/jeramiahgcoffey/portview/internal/config"
	"github.com/jeramiahgcoffey/portview/internal/docker"
	"github.com/jeramiahgcoffey/portview/internal/scanner"
)

const usage = `portview — discover and manage localhost dev servers

Usage:
  portview              launch the TUI
  portview list         print listening servers
      --json            output as JSON
      --all             include ports hidden by config
  portview kill <port>  stop the server on <port> (docker stop for containers)
  portview open <port>  open localhost:<port> in the browser
  portview version      print version
`

// Commands lists the subcommand names main dispatches to this package.
var Commands = []string{"list", "kill", "open", "version", "help"}

// fprintf writes formatted CLI output, ignoring write errors: output goes to
// a terminal or pipe, where a failed write has no recovery path.
func fprintf(w io.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(w, format, a...)
}

// Run executes a subcommand and returns the process exit code.
func Run(args []string, version string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fprintf(stderr, "%s", usage)
		return 2
	}
	switch args[0] {
	case "list":
		return runList(args[1:], stdout, stderr)
	case "kill":
		return runKill(args[1:], stdout, stderr)
	case "open":
		return runOpen(args[1:], stdout, stderr)
	case "version":
		fprintf(stdout, "portview %s\n", version)
		return 0
	case "help":
		fprintf(stdout, "%s", usage)
		return 0
	default:
		fprintf(stderr, "portview: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

// scan runs one full discovery pass with the user's config applied: platform
// scan, docker enrichment, then hidden-port filtering and labels (unless all).
func scan(ctx context.Context, all bool) ([]scanner.Server, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	s := scanner.New(scanner.Options{MinPort: cfg.PortRange.Min, MaxPort: cfg.PortRange.Max})
	servers, err := s.Scan(ctx)
	if err != nil {
		return nil, err
	}
	servers = docker.Enrich(ctx, servers)

	out := make([]scanner.Server, 0, len(servers))
	for _, sv := range servers {
		if !all && cfg.IsHidden(sv.Port) {
			continue
		}
		sv.Label = cfg.LabelFor(sv.Port)
		out = append(out, sv)
	}
	return out, nil
}

func runList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "output as JSON")
	all := fs.Bool("all", false, "include ports hidden by config")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	servers, err := scan(context.Background(), *all)
	if err != nil {
		fprintf(stderr, "portview: %v\n", err)
		return 1
	}

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(servers); err != nil {
			fprintf(stderr, "portview: %v\n", err)
			return 1
		}
		return 0
	}

	w := tabwriter.NewWriter(stdout, 2, 4, 2, ' ', 0)
	fprintf(w, "PORT\tPROCESS\tPID\tSTATE\tLABEL\tCONTAINER\tCOMMAND\n")
	for _, s := range servers {
		state := "up"
		if !s.Healthy {
			state = "down"
		}
		fprintf(w, "%d\t%s\t%d\t%s\t%s\t%s\t%s\n",
			s.Port, s.Process, s.PID, state, s.Label, s.Container, s.Command)
	}
	_ = w.Flush()
	return 0
}

func runKill(args []string, stdout, stderr io.Writer) int {
	port, ok := portArg("kill", args, stderr)
	if !ok {
		return 2
	}

	ctx := context.Background()
	servers, err := scan(ctx, true)
	if err != nil {
		fprintf(stderr, "portview: %v\n", err)
		return 1
	}
	for _, s := range servers {
		if s.Port != port {
			continue
		}
		if s.Container != "" {
			if err := docker.Stop(ctx, s.Container); err != nil {
				fprintf(stderr, "portview: %v\n", err)
				return 1
			}
			fprintf(stdout, "stopped container %s (:%d)\n", s.Container, port)
			return 0
		}
		if err := action.Kill(s.PID); err != nil {
			fprintf(stderr, "portview: kill pid %d: %v\n", s.PID, err)
			return 1
		}
		fprintf(stdout, "sent SIGTERM to %s (pid %d, :%d)\n", s.Process, s.PID, port)
		return 0
	}
	fprintf(stderr, "portview: nothing listening on port %d\n", port)
	return 1
}

func runOpen(args []string, stdout, stderr io.Writer) int {
	port, ok := portArg("open", args, stderr)
	if !ok {
		return 2
	}
	if err := action.OpenBrowser(port); err != nil {
		fprintf(stderr, "portview: %v\n", err)
		return 1
	}
	fprintf(stdout, "opened localhost:%d\n", port)
	return 0
}

// portArg parses the single required <port> argument of kill/open.
func portArg(cmd string, args []string, stderr io.Writer) (int, bool) {
	if len(args) != 1 {
		fprintf(stderr, "usage: portview %s <port>\n", cmd)
		return 0, false
	}
	port, err := strconv.Atoi(args[0])
	if err != nil || port < 1 || port > 65535 {
		fprintf(stderr, "portview: invalid port %q\n", args[0])
		return 0, false
	}
	return port, true
}
