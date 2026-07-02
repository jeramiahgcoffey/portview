//go:build darwin

package scanner

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// processUptime reports no override on macOS: `ps etime` is reliable here, so
// Inspect keeps the value it already parsed. (Linux overrides it from /proc.)
func processUptime(_ context.Context, _ int) (time.Duration, bool) {
	return 0, false
}

// processCWD returns pid's working directory via lsof. Empty on any failure —
// the insight pane simply omits the row.
func processCWD(ctx context.Context, pid int) string {
	// -Fn requests machine-readable output: one field per line, values
	// prefixed by a single type character ('n' is the file name).
	out, err := exec.CommandContext(ctx, "lsof", "-a", "-p", strconv.Itoa(pid), "-d", "cwd", "-Fn").Output()
	if err != nil {
		return ""
	}
	return parseLsofCWD(out)
}

// parseLsofCWD extracts the cwd path from `lsof -Fn` output such as:
//
//	p1234
//	fcwd
//	n/Users/me/projects/app
func parseLsofCWD(out []byte) string {
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "n") {
			return line[1:]
		}
	}
	return ""
}
