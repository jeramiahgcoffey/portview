package scanner

// Pure parsing helpers for the Linux uptime path: /proc/uptime and
// /proc/{pid}/stat. Kept free of build tags (like scanner_procnet.go) so the
// parsers are unit-tested on every platform; only detail_linux.go wires them
// to the real /proc.

import (
	"fmt"
	"strconv"
	"strings"
)

// parseProcUptime extracts seconds-since-boot from /proc/uptime, whose first
// field is e.g. "35045.12".
func parseProcUptime(data []byte) (float64, error) {
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, fmt.Errorf("empty /proc/uptime")
	}
	secs, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("parse /proc/uptime %q: %w", fields[0], err)
	}
	return secs, nil
}

// parseStatStartTicks extracts field 22 (starttime, in clock ticks since
// boot) from /proc/{pid}/stat. The comm field (2) may contain spaces and
// parentheses, so fields are counted from after the last ')': state is field
// 3 of the full line, making starttime index 19 of the remainder.
func parseStatStartTicks(data []byte) (int64, error) {
	s := string(data)
	i := strings.LastIndexByte(s, ')')
	if i < 0 {
		return 0, fmt.Errorf("malformed /proc stat line")
	}
	fields := strings.Fields(s[i+1:])
	if len(fields) < 20 {
		return 0, fmt.Errorf("short /proc stat line: %d fields after comm", len(fields))
	}
	ticks, err := strconv.ParseInt(fields[19], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse starttime %q: %w", fields[19], err)
	}
	return ticks, nil
}
