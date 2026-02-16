//go:build linux

package scanner

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jeramiahcoffey/portview/internal/config"
)

// linuxScanner discovers listening TCP servers on Linux by reading
// /proc/net/tcp and resolving process details from /proc.
type linuxScanner struct {
	portRange config.PortRange
}

// New returns a Scanner that reads /proc/net/tcp to discover listening TCP
// ports on Linux.
func New(portRange config.PortRange) Scanner {
	return &linuxScanner{portRange: portRange}
}

// Scan discovers listening TCP servers, resolves PIDs and process details,
// and filters by the configured port range.
func (l *linuxScanner) Scan(ctx context.Context) ([]Server, error) {
	data, err := os.ReadFile("/proc/net/tcp")
	if err != nil {
		return nil, err
	}

	servers := parseProcNetTCP(string(data))
	if len(servers) == 0 {
		return nil, nil
	}

	// Build a port-to-PID map using ss -tlnp (single invocation).
	portPIDs := resolvePortPIDs(ctx)

	var result []Server
	for _, s := range servers {
		// Filter by port range (skip filtering if both Min and Max are zero).
		if l.portRange.Min != 0 || l.portRange.Max != 0 {
			if s.Port < l.portRange.Min || s.Port > l.portRange.Max {
				continue
			}
		}

		if pid, ok := portPIDs[s.Port]; ok {
			s.PID = pid
			s.Process = readProcFile(pid, "comm")
			s.Command = readProcCmdline(pid)
		}

		result = append(result, s)
	}

	return result, nil
}

// resolvePortPIDs runs `ss -tlnp` and parses the output to build a map from
// listening port number to the owning PID. If ss fails or a line cannot be
// parsed, it is silently skipped.
func resolvePortPIDs(ctx context.Context) map[int]int {
	out, err := exec.CommandContext(ctx, "ss", "-tlnp").Output()
	if err != nil {
		return nil
	}
	return parseSSOutput(string(out))
}

// parseSSOutput parses `ss -tlnp` output and returns a port-to-PID map.
//
// Example ss output lines (after the header):
//
//	LISTEN  0  128  0.0.0.0:8080  0.0.0.0:*  users:(("node",pid=1234,fd=12))
//	LISTEN  0  128     [::]:22     [::]:*     users:(("sshd",pid=567,fd=3))
func parseSSOutput(output string) map[int]int {
	portPIDs := make(map[int]int)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) <= 1 {
		return portPIDs
	}

	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		// The local address:port is typically field index 3.
		localAddr := fields[3]
		lastColon := strings.LastIndex(localAddr, ":")
		if lastColon < 0 {
			continue
		}

		port, err := strconv.Atoi(localAddr[lastColon+1:])
		if err != nil {
			continue
		}

		// Extract PID from the users:(...) field. Look for "pid=NNNN" in the
		// remaining fields.
		pid := extractPIDFromSS(fields[4:])
		if pid > 0 {
			portPIDs[port] = pid
		}
	}

	return portPIDs
}

// extractPIDFromSS searches the trailing fields of an ss line for a
// "pid=NNNN" substring and returns the PID, or 0 if not found.
func extractPIDFromSS(fields []string) int {
	joined := strings.Join(fields, " ")
	const marker = "pid="
	idx := strings.Index(joined, marker)
	if idx < 0 {
		return 0
	}

	rest := joined[idx+len(marker):]
	// PID is terminated by ',' or ')'.
	end := strings.IndexAny(rest, ",)")
	if end < 0 {
		return 0
	}

	pid, err := strconv.Atoi(rest[:end])
	if err != nil {
		return 0
	}
	return pid
}

// readProcFile reads a single-line file from /proc/[pid]/<name> and returns
// its trimmed content, or an empty string on any error.
func readProcFile(pid int, name string) string {
	path := filepath.Join("/proc", strconv.Itoa(pid), name)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// readProcCmdline reads /proc/[pid]/cmdline which is NUL-delimited and
// returns a space-separated command line string.
func readProcCmdline(pid int) string {
	path := filepath.Join("/proc", strconv.Itoa(pid), "cmdline")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	// cmdline entries are separated by NUL bytes; replace with spaces and trim.
	cleaned := bytes.ReplaceAll(data, []byte{0}, []byte{' '})
	return strings.TrimSpace(string(cleaned))
}
