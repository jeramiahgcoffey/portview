package tui

import (
	"errors"
	"testing"

	"github.com/jeramiahcoffey/portview/internal/config"
	"github.com/jeramiahcoffey/portview/internal/scanner"
)

func TestNew_InitializesWithDefaults(t *testing.T) {
	servers := []scanner.Server{
		{Port: 8080, PID: 1234, Process: "node"},
	}
	mock := &scanner.MockScanner{Servers: servers}
	cfg := config.Default()

	m := New(mock, cfg, "/tmp/test-config.yaml")

	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
	if m.mode != modeNormal {
		t.Errorf("mode = %d, want modeNormal (%d)", m.mode, modeNormal)
	}
	if m.servers != nil {
		t.Errorf("servers should be nil initially, got %v", m.servers)
	}
	if m.scanner == nil {
		t.Error("scanner should not be nil")
	}
	if m.configPath != "/tmp/test-config.yaml" {
		t.Errorf("configPath = %q, want %q", m.configPath, "/tmp/test-config.yaml")
	}
	if m.err != nil {
		t.Errorf("err should be nil, got %v", m.err)
	}
}

func TestNew_AcceptsScanner(t *testing.T) {
	mock := &scanner.MockScanner{
		Servers: []scanner.Server{{Port: 3000}},
	}
	cfg := config.Default()

	m := New(mock, cfg, "")

	// Verify the scanner is stored (we can't compare interfaces directly,
	// but we can verify it's not nil)
	if m.scanner == nil {
		t.Error("scanner should be stored in model")
	}
}

func TestInit_ReturnsNonNilCmd(t *testing.T) {
	mock := &scanner.MockScanner{Servers: []scanner.Server{{Port: 8080}}}
	cfg := config.Default()
	m := New(mock, cfg, "")

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() should return a non-nil Cmd (batch of scan + tick)")
	}
}

func TestDoScan_ReturnsScanResultMsg(t *testing.T) {
	servers := []scanner.Server{
		{Port: 8080, PID: 1234, Process: "node"},
		{Port: 3000, PID: 5678, Process: "python"},
	}
	mock := &scanner.MockScanner{Servers: servers}
	cfg := config.Default()
	m := New(mock, cfg, "")

	cmd := m.doScan()
	msg := cmd()

	result, ok := msg.(scanResultMsg)
	if !ok {
		t.Fatalf("expected scanResultMsg, got %T", msg)
	}
	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if len(result.servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(result.servers))
	}
	if result.servers[0].Port != 8080 {
		t.Errorf("servers[0].Port = %d, want 8080", result.servers[0].Port)
	}
}

func TestDoScan_PropagatesError(t *testing.T) {
	expected := errors.New("scan failed")
	mock := &scanner.MockScanner{Err: expected}
	cfg := config.Default()
	m := New(mock, cfg, "")

	cmd := m.doScan()
	msg := cmd()

	result, ok := msg.(scanResultMsg)
	if !ok {
		t.Fatalf("expected scanResultMsg, got %T", msg)
	}
	if result.err != expected {
		t.Errorf("err = %v, want %v", result.err, expected)
	}
}
