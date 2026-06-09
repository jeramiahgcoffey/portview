// Command portview discovers and manages localhost dev servers.
//
// Phase 1 scaffolding: this main performs a single scan and prints the
// discovered servers as a table. It is replaced by the Bubble Tea TUI in a
// later phase.
package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/jeramiahgcoffey/portview/internal/scanner"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s := scanner.New(scanner.Options{})
	servers, err := s.Scan(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan failed: %v\n", err)
		os.Exit(1)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "PORT\tPID\tPROCESS\tHEALTHY\tCOMMAND")
	for _, srv := range servers {
		fmt.Fprintf(w, "%d\t%d\t%s\t%t\t%s\n",
			srv.Port, srv.PID, srv.Process, srv.Healthy, srv.Command)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "flush failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\n%d servers\n", len(servers))
}
