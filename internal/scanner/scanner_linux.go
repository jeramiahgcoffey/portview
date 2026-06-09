//go:build linux

package scanner

import (
	"context"
	"errors"
)

// errLinuxNotImplemented is returned by the placeholder Linux backend. The
// real /proc/net/tcp implementation lands in a later phase; until then the
// backend fails loudly rather than silently reporting zero servers.
var errLinuxNotImplemented = errors.New("scanner: linux backend not yet implemented")

// linuxScanner is a placeholder until the /proc/net/tcp backend is built.
type linuxScanner struct {
	opts Options
}

// New returns a Scanner for the host platform (Linux).
func New(opts Options) Scanner {
	return &linuxScanner{opts: opts.withDefaults()}
}

// Scan implements Scanner.
func (s *linuxScanner) Scan(_ context.Context) ([]Server, error) {
	return nil, errLinuxNotImplemented
}
