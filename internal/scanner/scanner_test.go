package scanner

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestMockScanner_ReturnsCannedData(t *testing.T) {
	servers := []Server{
		{Port: 8080, PID: 1234, Process: "node", Command: "node server.js", State: "LISTEN"},
		{Port: 3000, PID: 5678, Process: "python", Command: "python app.py", State: "LISTEN"},
	}

	var s Scanner = &MockScanner{Servers: servers}
	result, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(result))
	}
	if result[0].Port != 8080 {
		t.Errorf("result[0].Port = %d, want 8080", result[0].Port)
	}
	if result[1].Process != "python" {
		t.Errorf("result[1].Process = %q, want %q", result[1].Process, "python")
	}
}

func TestMockScanner_ReturnsError(t *testing.T) {
	expected := errors.New("scan failed")
	var s Scanner = &MockScanner{Err: expected}

	_, err := s.Scan(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != expected {
		t.Errorf("error = %v, want %v", err, expected)
	}
}

func TestParseProcNetTCP_SingleListen(t *testing.T) {
	content := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 12345 1 0000000000000000 100 0 0 10 0`

	servers := parseProcNetTCP(content)
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0].Port != 8080 {
		t.Errorf("Port = %d, want 8080", servers[0].Port)
	}
	if servers[0].State != "LISTEN" {
		t.Errorf("State = %q, want %q", servers[0].State, "LISTEN")
	}
}

func TestParseProcNetTCP_FiltersByListenState(t *testing.T) {
	content := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 12345 1 0000000000000000 100 0 0 10 0
   1: 0100007F:0050 00000000:0000 01 00000000:00000000 00:00000000 00000000     0        0 12346 1 0000000000000000 100 0 0 10 0`

	servers := parseProcNetTCP(content)
	if len(servers) != 1 {
		t.Fatalf("expected 1 server (LISTEN only), got %d", len(servers))
	}
	if servers[0].Port != 8080 {
		t.Errorf("Port = %d, want 8080", servers[0].Port)
	}
}

func TestParseProcNetTCP_HexPortConversion(t *testing.T) {
	content := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 12345 1 0000000000000000 100 0 0 10 0
   1: 00000000:0050 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 12347 1 0000000000000000 100 0 0 10 0`

	servers := parseProcNetTCP(content)
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}

	ports := map[int]bool{}
	for _, s := range servers {
		ports[s.Port] = true
	}
	if !ports[8080] {
		t.Error("expected port 8080 (from hex 1F90)")
	}
	if !ports[80] {
		t.Error("expected port 80 (from hex 0050)")
	}
}

func TestParseProcNetTCP_EmptyInput(t *testing.T) {
	servers := parseProcNetTCP("")
	if servers != nil {
		t.Errorf("expected nil for empty input, got %v", servers)
	}
}

func TestParseProcNetTCP_HeaderOnly(t *testing.T) {
	content := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode`

	servers := parseProcNetTCP(content)
	if servers != nil {
		t.Errorf("expected nil for header-only input, got %v", servers)
	}
}

// --- parseLsofOutput tests ---

func TestParseLsofOutput_SingleEntry(t *testing.T) {
	input := "COMMAND   PID USER   FD   TYPE DEVICE SIZE/OFF NODE NAME\nnode     1234 user   12u  IPv4 0x1234    0t0  TCP *:8080 (LISTEN)\n"

	servers := parseLsofOutput(input)

	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	s := servers[0]
	if s.Port != 8080 {
		t.Errorf("Port = %d, want 8080", s.Port)
	}
	if s.PID != 1234 {
		t.Errorf("PID = %d, want 1234", s.PID)
	}
	if s.Process != "node" {
		t.Errorf("Process = %q, want %q", s.Process, "node")
	}
	if s.State != "LISTEN" {
		t.Errorf("State = %q, want %q", s.State, "LISTEN")
	}
}

func TestParseLsofOutput_MultipleEntries(t *testing.T) {
	input := `COMMAND   PID USER   FD   TYPE DEVICE SIZE/OFF NODE NAME
node     1234 user   12u  IPv4 0x1234    0t0  TCP *:8080 (LISTEN)
python   5678 user   3u   IPv6 0x5678    0t0  TCP 127.0.0.1:3000 (LISTEN)
nginx    9012 root   6u   IPv4 0xabcd    0t0  TCP *:443 (LISTEN)
`

	servers := parseLsofOutput(input)

	if len(servers) != 3 {
		t.Fatalf("expected 3 servers, got %d", len(servers))
	}

	tests := []struct {
		index   int
		port    int
		pid     int
		process string
	}{
		{0, 8080, 1234, "node"},
		{1, 3000, 5678, "python"},
		{2, 443, 9012, "nginx"},
	}

	for _, tt := range tests {
		s := servers[tt.index]
		if s.Port != tt.port {
			t.Errorf("servers[%d].Port = %d, want %d", tt.index, s.Port, tt.port)
		}
		if s.PID != tt.pid {
			t.Errorf("servers[%d].PID = %d, want %d", tt.index, s.PID, tt.pid)
		}
		if s.Process != tt.process {
			t.Errorf("servers[%d].Process = %q, want %q", tt.index, s.Process, tt.process)
		}
		if s.State != "LISTEN" {
			t.Errorf("servers[%d].State = %q, want %q", tt.index, s.State, "LISTEN")
		}
	}
}

func TestParseLsofOutput_EmptyOutput(t *testing.T) {
	servers := parseLsofOutput("")

	if servers != nil {
		t.Errorf("expected nil, got %v", servers)
	}
}

func TestParseLsofOutput_HeaderOnly(t *testing.T) {
	input := "COMMAND   PID USER   FD   TYPE DEVICE SIZE/OFF NODE NAME\n"

	servers := parseLsofOutput(input)

	if servers != nil {
		t.Errorf("expected nil, got %v", servers)
	}
}

func TestParseLsofOutput_IPv4AndIPv6(t *testing.T) {
	input := `COMMAND   PID USER   FD   TYPE DEVICE SIZE/OFF NODE NAME
node     1234 user   12u  IPv4 0x1234    0t0  TCP 127.0.0.1:8080 (LISTEN)
node     1235 user   13u  IPv6 0x5678    0t0  TCP *:9090 (LISTEN)
`

	servers := parseLsofOutput(input)

	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}

	if servers[0].Port != 8080 {
		t.Errorf("servers[0].Port = %d, want 8080 (IPv4 127.0.0.1:8080)", servers[0].Port)
	}
	if servers[1].Port != 9090 {
		t.Errorf("servers[1].Port = %d, want 9090 (IPv6 *:9090)", servers[1].Port)
	}
}

func TestParseLsofOutput_SkipsMalformedLines(t *testing.T) {
	input := `COMMAND   PID USER   FD   TYPE DEVICE SIZE/OFF NODE NAME
node     1234 user   12u  IPv4 0x1234    0t0  TCP *:8080 (LISTEN)
this is garbage
short
python   notapid user  3u   IPv6 0x5678    0t0  TCP *:3000 (LISTEN)
node     5678 user   3u   IPv6 0x5678    0t0  TCP *:4000 (LISTEN)
`

	servers := parseLsofOutput(input)

	if len(servers) != 2 {
		t.Fatalf("expected 2 servers (skipping malformed), got %d", len(servers))
	}
	if servers[0].Port != 8080 {
		t.Errorf("servers[0].Port = %d, want 8080", servers[0].Port)
	}
	if servers[1].Port != 4000 {
		t.Errorf("servers[1].Port = %d, want 4000", servers[1].Port)
	}
}

// --- CheckHealth tests ---

func TestCheckHealth_ResponsivePort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	servers := []Server{{Port: port, Process: "test"}}

	result := CheckHealth(servers, 2*time.Second)

	if len(result) != 1 {
		t.Fatalf("expected 1 server, got %d", len(result))
	}
	if !result[0].Healthy {
		t.Errorf("expected Healthy=true for responsive port %d", port)
	}
	// Verify original slice was not mutated
	if servers[0].Healthy {
		t.Error("original slice should not be mutated")
	}
}

func TestCheckHealth_UnresponsivePort(t *testing.T) {
	servers := []Server{{Port: 59999, Process: "ghost"}}

	result := CheckHealth(servers, 200*time.Millisecond)

	if len(result) != 1 {
		t.Fatalf("expected 1 server, got %d", len(result))
	}
	if result[0].Healthy {
		t.Errorf("expected Healthy=false for unresponsive port 59999")
	}
}

func TestCheckHealth_ConcurrentChecks(t *testing.T) {
	ln1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener 1: %v", err)
	}
	defer ln1.Close()

	ln2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener 2: %v", err)
	}
	defer ln2.Close()

	port1 := ln1.Addr().(*net.TCPAddr).Port
	port2 := ln2.Addr().(*net.TCPAddr).Port

	servers := []Server{
		{Port: port1, Process: "svc1"},
		{Port: port2, Process: "svc2"},
		{Port: 59999, Process: "ghost"},
	}

	result := CheckHealth(servers, 500*time.Millisecond)

	if len(result) != 3 {
		t.Fatalf("expected 3 servers, got %d", len(result))
	}
	if !result[0].Healthy {
		t.Errorf("expected result[0] (port %d) Healthy=true", port1)
	}
	if !result[1].Healthy {
		t.Errorf("expected result[1] (port %d) Healthy=true", port2)
	}
	if result[2].Healthy {
		t.Errorf("expected result[2] (port 59999) Healthy=false")
	}
}
