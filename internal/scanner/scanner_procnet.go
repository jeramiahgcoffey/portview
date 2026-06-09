package scanner

import (
	"strconv"
	"strings"
)

// This file holds the pure parsers for the Linux /proc-based backend. They live
// in an untagged file (no //go:build linux) so they can be unit-tested on any
// platform; the OS-specific I/O that feeds them lives in scanner_linux.go.

// tcpStateListen is the hex code for the TCP LISTEN state in /proc/net/tcp.
const tcpStateListen = "0A"

// procEntry is a listening socket discovered from /proc/net/tcp{,6}.
type procEntry struct {
	Port  int
	Inode string
}

// parseProcNet parses /proc/net/tcp or /proc/net/tcp6 content and returns the
// sockets in LISTEN state. The columns of interest are:
//
//	sl  local_address rem_address st ... inode
//	 0: 0100007F:1538 00000000:0000 0A ... 12345
//
// local_address is HEXIP:HEXPORT, st is the connection state, and inode (field
// index 9) links the socket to an owning process.
func parseProcNet(data []byte) []procEntry {
	var out []procEntry
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		if fields[0] == "sl" { // header row
			continue
		}
		if fields[3] != tcpStateListen {
			continue
		}
		port, ok := portFromHexAddr(fields[1])
		if !ok {
			continue
		}
		out = append(out, procEntry{Port: port, Inode: fields[9]})
	}
	return out
}

// portFromHexAddr extracts the port from a /proc local_address token such as
// "0100007F:1538" (IPv4) or a 32-hex-char IPv6 address followed by ":PORT".
// The port is the hexadecimal value after the final colon.
func portFromHexAddr(addr string) (int, bool) {
	i := strings.LastIndex(addr, ":")
	if i < 0 || i == len(addr)-1 {
		return 0, false
	}
	v, err := strconv.ParseInt(addr[i+1:], 16, 32)
	if err != nil {
		return 0, false
	}
	return int(v), true
}

// parseInodeFromLink extracts the socket inode from an fd symlink target of the
// form "socket:[12345]".
func parseInodeFromLink(link string) (string, bool) {
	const prefix = "socket:["
	if !strings.HasPrefix(link, prefix) || !strings.HasSuffix(link, "]") {
		return "", false
	}
	return link[len(prefix) : len(link)-1], true
}

// parseCmdline turns the NUL-separated /proc/{pid}/cmdline into a space-joined
// command string.
func parseCmdline(b []byte) string {
	s := strings.TrimRight(string(b), "\x00")
	s = strings.ReplaceAll(s, "\x00", " ")
	return strings.TrimSpace(s)
}

// dedupeEntriesByPort collapses entries sharing a port (a server bound on both
// IPv4 and IPv6 appears in both tcp and tcp6). The first entry per port wins.
func dedupeEntriesByPort(in []procEntry) []procEntry {
	seen := make(map[int]struct{}, len(in))
	out := make([]procEntry, 0, len(in))
	for _, e := range in {
		if _, ok := seen[e.Port]; ok {
			continue
		}
		seen[e.Port] = struct{}{}
		out = append(out, e)
	}
	return out
}
