//go:build darwin

package scanner

import (
	"context"
	"os/exec"
	"strconv"
	"strings"

	"github.com/jeramiahcoffey/portview/internal/config"
)

// darwinScanner discovers listening TCP servers on macOS using lsof.
type darwinScanner struct {
	portRange config.PortRange
}

// New returns a Scanner that uses lsof to discover listening TCP ports on macOS.
func New(portRange config.PortRange) Scanner {
	return &darwinScanner{portRange: portRange}
}

// Scan discovers listening TCP servers, resolves process details via ps,
// and filters by the configured port range.
func (d *darwinScanner) Scan(ctx context.Context) ([]Server, error) {
	out, err := exec.CommandContext(ctx, "lsof", "-iTCP", "-sTCP:LISTEN", "-nP").Output()
	if err != nil {
		return nil, err
	}

	servers := parseLsofOutput(string(out))

	var result []Server
	for _, s := range servers {
		// Resolve process details via ps.
		psOut, err := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(s.PID), "-o", "comm=,args=").Output()
		if err == nil {
			line := strings.TrimSpace(string(psOut))
			if line != "" {
				// ps output format: "comm args..."
				// The comm field is the first whitespace-delimited token,
				// and args is the remainder.
				parts := strings.SplitN(line, " ", 2)
				comm := parts[0]
				if len(parts) == 2 {
					s.Command = strings.TrimSpace(parts[1])
				}
				// Use comm as Process if it is more detailed (longer) than
				// what lsof provided.
				if len(comm) > len(s.Process) {
					s.Process = comm
				}
			}
		}

		// Filter by port range (skip filtering if both Min and Max are zero).
		if d.portRange.Min != 0 || d.portRange.Max != 0 {
			if s.Port < d.portRange.Min || s.Port > d.portRange.Max {
				continue
			}
		}

		result = append(result, s)
	}

	return result, nil
}
