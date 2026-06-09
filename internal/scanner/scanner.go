// Package scanner discovers TCP servers listening on localhost and resolves
// the process that owns each port. Platform-specific backends (macOS, Linux)
// satisfy the Scanner interface so that callers never depend on the host OS.
package scanner

import (
	"context"
	"net"
	"sort"
	"strconv"
	"time"
)

// Server describes a single discovered listening server.
type Server struct {
	Port    int    // TCP port number
	PID     int    // OS process ID
	Process string // Short process name (e.g., "node", "python3", "go")
	Command string // Full command line (e.g., "node server.js")
	State   string // TCP state, typically "LISTEN"
	Label   string // User-assigned label from config (e.g., "frontend")
	Healthy bool   // True if port responds to TCP connect
}

// Scanner discovers listening servers. Implementations are platform-specific.
type Scanner interface {
	Scan(ctx context.Context) ([]Server, error)
}

// Defaults for the scan. The port range mirrors the design doc: well-known
// ports below 1024 are skipped to avoid noise from system services.
const (
	DefaultMinPort       = 1024
	DefaultMaxPort       = 65535
	DefaultHealthTimeout = 200 * time.Millisecond
)

// Options configures a platform scanner. Zero values fall back to defaults,
// so callers can pass the zero Options and get sensible behavior.
type Options struct {
	MinPort       int
	MaxPort       int
	HealthTimeout time.Duration
}

// withDefaults returns a copy of o with any zero fields replaced by defaults.
func (o Options) withDefaults() Options {
	if o.MinPort == 0 {
		o.MinPort = DefaultMinPort
	}
	if o.MaxPort == 0 {
		o.MaxPort = DefaultMaxPort
	}
	if o.HealthTimeout == 0 {
		o.HealthTimeout = DefaultHealthTimeout
	}
	return o
}

// listing is the intermediate result of port discovery, before process info
// and health are resolved. Comm is the (possibly truncated) command name as
// reported by the discovery source, used only as a fallback.
type listing struct {
	Port int
	PID  int
	Comm string
}

// inRange reports whether port falls within [lo, hi] inclusive.
func inRange(port, lo, hi int) bool {
	return port >= lo && port <= hi
}

// dedupeByPort collapses listings that share a port (e.g. a server bound on
// both IPv4 and IPv6 produces two rows). The first listing for each port
// wins. Input order is otherwise preserved.
func dedupeByPort(in []listing) []listing {
	seen := make(map[int]struct{}, len(in))
	out := make([]listing, 0, len(in))
	for _, l := range in {
		if _, ok := seen[l.Port]; ok {
			continue
		}
		seen[l.Port] = struct{}{}
		out = append(out, l)
	}
	return out
}

// dialHealthy reports whether a TCP connection to localhost:port succeeds
// within timeout.
func dialHealthy(port int, timeout time.Duration) bool {
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// sortByPort orders servers ascending by port, in place.
func sortByPort(servers []Server) {
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Port < servers[j].Port
	})
}
