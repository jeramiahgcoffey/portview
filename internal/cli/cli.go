// Package cli implements portview's non-TUI subcommands. These make portview
// scriptable — `portview list --json` in a pipeline, `portview kill 3000` in an
// alias, or `portview hide 5432` to persistently declutter the default view —
// without opening the full-screen UI.
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"sort"
	"strconv"
	"text/tabwriter"
	"time"

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
  portview hide <port>  hide <port> from the default list
  portview unhide <port> show <port> in the default list again
  portview hidden       list all configured hidden ports
      --json            output as JSON
  portview version      print version
`

// Commands lists the subcommand names main dispatches to this package.
var Commands = []string{"list", "kill", "open", "hide", "unhide", "hidden", "version", "help"}

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
	case "hide":
		return runVisibility("hide", args[1:], true, stdout, stderr)
	case "unhide":
		return runVisibility("unhide", args[1:], false, stdout, stderr)
	case "hidden":
		return runHidden(args[1:], stdout, stderr)
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

// scanTimeout bounds a CLI discovery pass. The CLI is meant to be scriptable,
// so an unresponsive lsof/proc or docker daemon must not hang it forever.
const scanTimeout = 10 * time.Second

// scan runs one full discovery pass with the user's config applied: platform
// scan, docker enrichment, then hidden-port filtering and labels (unless all).
func scan(ctx context.Context, all bool) ([]scanner.Server, error) {
	ctx, cancel := context.WithTimeout(ctx, scanTimeout)
	defer cancel()

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
	return cfg.Decorate(servers, all), nil
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

	writeServerTable(stdout, servers, *all)
	return 0
}

func writeServerTable(stdout io.Writer, servers []scanner.Server, showHiddenColumn bool) {
	w := tabwriter.NewWriter(stdout, 2, 4, 2, ' ', 0)
	if showHiddenColumn {
		fprintf(w, "PORT\tPROCESS\tPID\tSTATE\tLABEL\tHIDDEN\tCONTAINER\tCOMMAND\n")
	} else {
		fprintf(w, "PORT\tPROCESS\tPID\tSTATE\tLABEL\tCONTAINER\tCOMMAND\n")
	}
	for _, s := range servers {
		state := "up"
		if !s.Healthy {
			state = "down"
		}
		hidden := ""
		if s.Hidden {
			hidden = "yes"
		}
		if showHiddenColumn {
			fprintf(w, "%d\t%s\t%d\t%s\t%s\t%s\t%s\t%s\n",
				s.Port, s.Process, s.PID, state, s.Label, hidden, s.Container, s.Command)
		} else {
			fprintf(w, "%d\t%s\t%d\t%s\t%s\t%s\t%s\n",
				s.Port, s.Process, s.PID, state, s.Label, s.Container, s.Command)
		}
	}
	_ = w.Flush()
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

func runVisibility(cmd string, args []string, hide bool, stdout, stderr io.Writer) int {
	port, ok := portArg(cmd, args, stderr)
	if !ok {
		return 2
	}

	cfg, err := config.Load()
	if err != nil {
		fprintf(stderr, "portview: %v\n", err)
		return 1
	}

	wasHidden := cfg.IsHidden(port)
	switch {
	case hide && wasHidden:
		fprintf(stdout, "port %d is already hidden\n", port)
		return 0
	case !hide && !wasHidden:
		fprintf(stdout, "port %d is not hidden\n", port)
		return 0
	case hide:
		cfg.Hide(port)
	case !hide:
		cfg.Unhide(port)
	}

	if err := cfg.Save(); err != nil {
		fprintf(stderr, "portview: %v\n", err)
		return 1
	}
	if hide {
		fprintf(stdout, "hid port %d\n", port)
	} else {
		fprintf(stdout, "unhid port %d\n", port)
	}
	return 0
}

func runHidden(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("hidden", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "output as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fprintf(stderr, "usage: portview hidden\n")
		return 2
	}
	cfg, err := config.Load()
	if err != nil {
		fprintf(stderr, "portview: %v\n", err)
		return 1
	}
	type hiddenPort struct {
		Port  int    `json:"port"`
		Label string `json:"label,omitempty"`
	}

	ports := append([]int(nil), cfg.Hidden...)
	sort.Ints(ports)
	hidden := make([]hiddenPort, 0, len(ports))
	for _, port := range ports {
		hidden = append(hidden, hiddenPort{Port: port, Label: cfg.LabelFor(port)})
	}

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(hidden); err != nil {
			fprintf(stderr, "portview: %v\n", err)
			return 1
		}
		return 0
	}
	if len(hidden) == 0 {
		fprintf(stdout, "no hidden ports\n")
		return 0
	}

	w := tabwriter.NewWriter(stdout, 2, 4, 2, ' ', 0)
	fprintf(w, "PORT\tLABEL\n")
	for _, item := range hidden {
		fprintf(w, "%d\t%s\n", item.Port, item.Label)
	}
	_ = w.Flush()
	return 0
}

// portArg parses the single required <port> argument for a port subcommand.
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
