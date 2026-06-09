//go:build darwin

package scanner

import (
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// darwinScanner discovers listening servers on macOS by shelling out to lsof
// for port/PID discovery and ps for process resolution.
type darwinScanner struct {
	opts Options
}

// New returns a Scanner for the host platform (macOS).
func New(opts Options) Scanner {
	return &darwinScanner{opts: opts.withDefaults()}
}

// Scan implements Scanner. It discovers listening ports via lsof, resolves
// each owning process via ps, and health-checks each port.
func (s *darwinScanner) Scan(ctx context.Context) ([]Server, error) {
	out, err := exec.CommandContext(ctx, "lsof", "-iTCP", "-sTCP:LISTEN", "-nP").Output()
	if err != nil {
		// lsof exits non-zero when nothing is listening; treat as empty.
		if _, ok := err.(*exec.ExitError); ok {
			return nil, nil
		}
		return nil, err
	}

	listings := dedupeByPort(parseLsof(out))

	servers := make([]Server, 0, len(listings))
	for _, l := range listings {
		if !inRange(l.Port, s.opts.MinPort, s.opts.MaxPort) {
			continue
		}
		name, command := resolveProcessDarwin(ctx, l.PID)
		if name == "" {
			name = l.Comm
		}
		servers = append(servers, Server{
			Port:    l.Port,
			PID:     l.PID,
			Process: name,
			Command: command,
			State:   "LISTEN",
			Healthy: dialHealthy(l.Port, s.opts.HealthTimeout),
		})
	}

	sortByPort(servers)
	return servers, nil
}

// parseLsof extracts listings from `lsof -iTCP -sTCP:LISTEN -nP` output.
// Each data line looks like:
//
//	node   1234 user   23u  IPv4 0x...  0t0  TCP 127.0.0.1:3000 (LISTEN)
//	node   1234 user   24u  IPv6 0x...  0t0  TCP [::1]:3000 (LISTEN)
//	node   1234 user   25u  IPv4 0x...  0t0  TCP *:8080 (LISTEN)
//
// The address (second-to-last token) yields the port; the trailing token must
// be "(LISTEN)". The header line and any malformed lines are skipped.
func parseLsof(out []byte) []listing {
	var listings []listing
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[0] == "COMMAND" { // header
			continue
		}
		if fields[len(fields)-1] != "(LISTEN)" {
			continue
		}
		pid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		port, ok := portFromAddr(fields[len(fields)-2])
		if !ok {
			continue
		}
		listings = append(listings, listing{Port: port, PID: pid, Comm: fields[0]})
	}
	return listings
}

// portFromAddr extracts the port from an lsof address token such as
// "127.0.0.1:3000", "[::1]:3000", or "*:8080". The port is the integer after
// the final colon.
func portFromAddr(addr string) (int, bool) {
	i := strings.LastIndex(addr, ":")
	if i < 0 || i == len(addr)-1 {
		return 0, false
	}
	port, err := strconv.Atoi(addr[i+1:])
	if err != nil {
		return 0, false
	}
	return port, true
}

// resolveProcessDarwin returns the short process name and full command line for
// pid using ps. On any failure it returns empty strings and the caller falls
// back to the name lsof reported.
func resolveProcessDarwin(ctx context.Context, pid int) (name, command string) {
	comm, err := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
	if err == nil {
		name = procName(string(comm))
	}
	args, err := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "args=").Output()
	if err == nil {
		command = strings.TrimSpace(string(args))
	}
	return name, command
}

// procName reduces a ps comm value (often a full executable path on macOS) to
// a short process name.
func procName(comm string) string {
	return filepath.Base(strings.TrimSpace(comm))
}
