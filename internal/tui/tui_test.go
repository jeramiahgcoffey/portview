package tui

import (
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
