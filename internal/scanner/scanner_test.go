package scanner

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

// mockScanner returns canned data and satisfies Scanner. It is the seam used
// by higher layers (app logic, TUI) to test without touching the host OS.
type mockScanner struct {
	servers []Server
	err     error
}

func (m mockScanner) Scan(context.Context) ([]Server, error) {
	return m.servers, m.err
}

func TestMockScannerSatisfiesInterface(t *testing.T) {
	var s Scanner = mockScanner{
		servers: []Server{{Port: 3000, PID: 1, Process: "node"}},
	}
	got, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Port != 3000 {
		t.Fatalf("got %+v, want one server on port 3000", got)
	}
}

func TestMockScannerError(t *testing.T) {
	want := errors.New("boom")
	s := mockScanner{err: want}
	if _, err := s.Scan(context.Background()); !errors.Is(err, want) {
		t.Fatalf("got err %v, want %v", err, want)
	}
}

func TestInRange(t *testing.T) {
	tests := []struct {
		name           string
		port, min, max int
		want           bool
	}{
		{"below", 80, 1024, 65535, false},
		{"at min", 1024, 1024, 65535, true},
		{"middle", 3000, 1024, 65535, true},
		{"at max", 65535, 1024, 65535, true},
		{"above", 70000, 1024, 65535, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inRange(tt.port, tt.min, tt.max); got != tt.want {
				t.Errorf("inRange(%d, %d, %d) = %v, want %v", tt.port, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

func TestDedupeByPort(t *testing.T) {
	in := []listing{
		{Port: 3000, PID: 1, Comm: "node"}, // IPv4
		{Port: 3000, PID: 1, Comm: "node"}, // IPv6 duplicate
		{Port: 8080, PID: 2, Comm: "go"},
		{Port: 3000, PID: 9, Comm: "other"}, // same port, different pid -> dropped
	}
	got := dedupeByPort(in)
	if len(got) != 2 {
		t.Fatalf("got %d listings, want 2: %+v", len(got), got)
	}
	if got[0].Port != 3000 || got[0].PID != 1 {
		t.Errorf("first listing = %+v, want port 3000 pid 1 (first wins)", got[0])
	}
	if got[1].Port != 8080 {
		t.Errorf("second listing = %+v, want port 8080", got[1])
	}
}

func TestSortByPort(t *testing.T) {
	servers := []Server{{Port: 8080}, {Port: 1024}, {Port: 3000}}
	sortByPort(servers)
	want := []int{1024, 3000, 8080}
	for i, w := range want {
		if servers[i].Port != w {
			t.Errorf("servers[%d].Port = %d, want %d", i, servers[i].Port, w)
		}
	}
}

func TestDialHealthy(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	if !dialHealthy(port, 500*time.Millisecond) {
		t.Errorf("dialHealthy(%d) = false, want true for an open listener", port)
	}

	ln.Close()
	if dialHealthy(port, 200*time.Millisecond) {
		t.Errorf("dialHealthy(%d) = true, want false after listener closed", port)
	}
}
