//go:build integration

package scanner

import (
	"context"
	"net"
	"testing"
)

// TestScanFindsLocalListener exercises the real platform backend (lsof on
// macOS, /proc on Linux) against an actual listening socket. It is gated behind
// the `integration` build tag and runs on CI runners.
func TestScanFindsLocalListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	s := New(Options{MinPort: 1, MaxPort: 65535})
	servers, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	for _, srv := range servers {
		if srv.Port == port {
			if !srv.Healthy {
				t.Errorf("listener on %d reported unhealthy", port)
			}
			return
		}
	}
	t.Errorf("listener on port %d not discovered among %d servers", port, len(servers))
}
