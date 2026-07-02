//go:build linux

package scanner

import (
	"context"
	"os"
	"strconv"
)

// processCWD returns pid's working directory by resolving the /proc symlink.
// Empty on any failure — the insight pane simply omits the row.
func processCWD(_ context.Context, pid int) string {
	cwd, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/cwd")
	if err != nil {
		return ""
	}
	return cwd
}
