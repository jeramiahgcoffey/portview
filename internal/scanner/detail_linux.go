//go:build linux

package scanner

import (
	"context"
	"os"
	"strconv"
	"time"
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

// linuxClockTick is the kernel USER_HZ constant that /proc stat times are
// reported in. It is fixed at 100 on Linux (sysconf(_SC_CLK_TCK)).
const linuxClockTick = 100

// processUptime computes pid's age from /proc/uptime and the starttime field
// of /proc/{pid}/stat. Both are measured relative to boot, so the result is
// immune to the wall-clock/boot-time skew that makes `ps etime` report
// garbage on some virtualized hosts (seen on GitHub Actions runners).
func processUptime(_ context.Context, pid int) (time.Duration, bool) {
	up, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, false
	}
	bootSecs, err := parseProcUptime(up)
	if err != nil {
		return 0, false
	}
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, false
	}
	ticks, err := parseStatStartTicks(stat)
	if err != nil {
		return 0, false
	}
	age := bootSecs - float64(ticks)/linuxClockTick
	if age < 0 {
		return 0, false
	}
	return time.Duration(age * float64(time.Second)), true
}
