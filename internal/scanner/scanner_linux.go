//go:build linux

package scanner

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// linuxScanner discovers listening servers on Linux by reading /proc/net/tcp
// (and tcp6), mapping socket inodes to PIDs via /proc/{pid}/fd, and resolving
// process info from /proc/{pid}/comm and /proc/{pid}/cmdline.
type linuxScanner struct {
	opts     Options
	procRoot string
}

// New returns a Scanner for the host platform (Linux).
func New(opts Options) Scanner {
	return &linuxScanner{opts: opts.withDefaults(), procRoot: "/proc"}
}

// Scan implements Scanner.
func (s *linuxScanner) Scan(_ context.Context) ([]Server, error) {
	var entries []procEntry
	for _, name := range []string{"net/tcp", "net/tcp6"} {
		data, err := os.ReadFile(filepath.Join(s.procRoot, name))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue // tcp6 may be absent on IPv6-disabled hosts
			}
			return nil, err
		}
		entries = append(entries, parseProcNet(data)...)
	}
	entries = dedupeEntriesByPort(entries)

	inodeToPID := s.buildInodeMap()

	servers := make([]Server, 0, len(entries))
	for _, e := range entries {
		if !inRange(e.Port, s.opts.MinPort, s.opts.MaxPort) {
			continue
		}
		pid := inodeToPID[e.Inode]
		if pid == 0 {
			// Owning process not resolvable (e.g. a root-owned socket during an
			// unprivileged scan). Skip it to match the darwin backend, where
			// unprivileged lsof hides such processes entirely.
			continue
		}
		name, command := resolveProcessLinux(s.procRoot, pid)
		servers = append(servers, Server{
			Port:    e.Port,
			PID:     pid,
			Process: name,
			Command: command,
			State:   "LISTEN",
			Healthy: dialHealthy(e.Port, s.opts.HealthTimeout),
		})
	}

	sortByPort(servers)
	return servers, nil
}

// buildInodeMap maps each socket inode to its owning PID by reading every
// /proc/{pid}/fd symlink. Sockets owned by other users are silently skipped
// when their fd directories are not readable (as with lsof run unprivileged).
func (s *linuxScanner) buildInodeMap() map[string]int {
	m := make(map[string]int)
	pidDirs, _ := filepath.Glob(filepath.Join(s.procRoot, "[0-9]*"))
	for _, dir := range pidDirs {
		pid, err := strconv.Atoi(filepath.Base(dir))
		if err != nil {
			continue
		}
		fds, _ := filepath.Glob(filepath.Join(dir, "fd", "*"))
		for _, fd := range fds {
			link, err := os.Readlink(fd)
			if err != nil {
				continue
			}
			if inode, ok := parseInodeFromLink(link); ok {
				if _, exists := m[inode]; !exists {
					m[inode] = pid
				}
			}
		}
	}
	return m
}

// resolveProcessLinux returns the short process name and full command line for
// pid. A zero pid (inode not resolved to a process) yields empty strings.
func resolveProcessLinux(procRoot string, pid int) (name, command string) {
	if pid == 0 {
		return "", ""
	}
	base := filepath.Join(procRoot, strconv.Itoa(pid))
	if b, err := os.ReadFile(filepath.Join(base, "comm")); err == nil {
		name = strings.TrimSpace(string(b))
	}
	if b, err := os.ReadFile(filepath.Join(base, "cmdline")); err == nil {
		command = parseCmdline(b)
	}
	return name, command
}
