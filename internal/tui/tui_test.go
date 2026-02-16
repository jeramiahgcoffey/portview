package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

// --- helpers for creating a model with servers already loaded ---

func modelWithServers(servers []scanner.Server) Model {
	mock := &scanner.MockScanner{Servers: servers}
	cfg := config.Default()
	m := New(mock, cfg, "")
	// Simulate a scan result arriving
	m.servers = servers
	m.filtered = make([]scanner.Server, len(servers))
	copy(m.filtered, servers)
	return m
}

func keyMsg(k string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}

// --- Task 3.4: Update tests ---

func TestUpdate_DownKey_IncrementsCursor(t *testing.T) {
	m := modelWithServers([]scanner.Server{
		{Port: 8080}, {Port: 3000}, {Port: 443},
	})

	result, _ := m.Update(keyMsg("j"))
	updated := result.(Model)
	if updated.cursor != 1 {
		t.Errorf("cursor = %d, want 1", updated.cursor)
	}
}

func TestUpdate_UpKey_DecrementsCursor(t *testing.T) {
	m := modelWithServers([]scanner.Server{
		{Port: 8080}, {Port: 3000},
	})
	m.cursor = 1

	result, _ := m.Update(keyMsg("k"))
	updated := result.(Model)
	if updated.cursor != 0 {
		t.Errorf("cursor = %d, want 0", updated.cursor)
	}
}

func TestUpdate_DownKey_StopsAtEnd(t *testing.T) {
	m := modelWithServers([]scanner.Server{
		{Port: 8080}, {Port: 3000},
	})
	m.cursor = 1 // already at last item

	result, _ := m.Update(keyMsg("j"))
	updated := result.(Model)
	if updated.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (should not go past end)", updated.cursor)
	}
}

func TestUpdate_UpKey_StopsAtZero(t *testing.T) {
	m := modelWithServers([]scanner.Server{{Port: 8080}})
	m.cursor = 0

	result, _ := m.Update(keyMsg("k"))
	updated := result.(Model)
	if updated.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (should not go below zero)", updated.cursor)
	}
}

func TestUpdate_QuitKey_ReturnsQuitCmd(t *testing.T) {
	m := modelWithServers([]scanner.Server{{Port: 8080}})

	_, cmd := m.Update(keyMsg("q"))
	if cmd == nil {
		t.Fatal("expected non-nil cmd for quit")
	}
	// Execute the cmd and check it produces a QuitMsg
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestUpdate_ScanResult_UpdatesServers(t *testing.T) {
	mock := &scanner.MockScanner{}
	cfg := config.Default()
	m := New(mock, cfg, "")

	servers := []scanner.Server{
		{Port: 8080, Process: "node"},
		{Port: 3000, Process: "python"},
	}

	result, _ := m.Update(scanResultMsg{servers: servers})
	updated := result.(Model)

	if len(updated.servers) != 2 {
		t.Fatalf("servers length = %d, want 2", len(updated.servers))
	}
	if len(updated.filtered) != 2 {
		t.Fatalf("filtered length = %d, want 2", len(updated.filtered))
	}
	if updated.err != nil {
		t.Errorf("err = %v, want nil", updated.err)
	}
}

func TestUpdate_ScanResult_WithError_SetsError(t *testing.T) {
	mock := &scanner.MockScanner{}
	cfg := config.Default()
	m := New(mock, cfg, "")

	expected := errors.New("scan failed")
	result, _ := m.Update(scanResultMsg{err: expected})
	updated := result.(Model)

	if updated.err != expected {
		t.Errorf("err = %v, want %v", updated.err, expected)
	}
}

func TestUpdate_WindowSizeMsg_UpdatesDimensions(t *testing.T) {
	mock := &scanner.MockScanner{}
	cfg := config.Default()
	m := New(mock, cfg, "")

	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	updated := result.(Model)

	if updated.width != 120 {
		t.Errorf("width = %d, want 120", updated.width)
	}
	if updated.height != 40 {
		t.Errorf("height = %d, want 40", updated.height)
	}
}

func TestUpdate_TickMsg_ReturnsCmd(t *testing.T) {
	mock := &scanner.MockScanner{Servers: []scanner.Server{{Port: 8080}}}
	cfg := config.Default()
	m := New(mock, cfg, "")

	_, cmd := m.Update(tickMsg{})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from tickMsg (should trigger scan + next tick)")
	}
}
