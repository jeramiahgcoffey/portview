package tui

import (
	"bytes"
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/jeramiahgcoffey/portview/internal/config"
	"github.com/jeramiahgcoffey/portview/internal/scanner"
)

// mockScanner returns canned data for TUI tests.
type mockScanner struct {
	servers []scanner.Server
	err     error
}

func (m mockScanner) Scan(context.Context) ([]scanner.Server, error) {
	return m.servers, m.err
}

func testServers() []scanner.Server {
	return []scanner.Server{
		{Port: 3000, PID: 1, Process: "node", Command: "next dev", Healthy: true},
		{Port: 5432, PID: 2, Process: "postgres", Command: "postgres -D /data", Healthy: true},
		{Port: 8080, PID: 3, Process: "go", Command: "go run main.go", Healthy: false},
	}
}

func newTestModel(t *testing.T, srv []scanner.Server) *teatest.TestModel {
	t.Helper()
	m := New(mockScanner{servers: srv}, config.Default())
	return teatest.NewTestModel(t, m, teatest.WithInitialTermSize(100, 24))
}

// quit sends q and waits for the program to finish.
func quit(t *testing.T, tm *teatest.TestModel) {
	t.Helper()
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// containsAll reports whether b contains every substring in subs.
func containsAll(b []byte, subs ...string) bool {
	for _, s := range subs {
		if !bytes.Contains(b, []byte(s)) {
			return false
		}
	}
	return true
}

func TestRendersDiscoveredServers(t *testing.T) {
	tm := newTestModel(t, testServers())

	// A single reader, accumulated by WaitFor, avoids draining the stream.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return containsAll(b, "portview", "PORT", "PROCESS", "3000", "node", "8080", "3 servers")
	}, teatest.WithDuration(3*time.Second))

	quit(t, tm)
}

func TestEmptyListRendersMessage(t *testing.T) {
	tm := newTestModel(t, nil)

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return containsAll(b, "0 servers", "no servers found")
	}, teatest.WithDuration(3*time.Second))

	quit(t, tm)
}

func TestQuitWithCtrlC(t *testing.T) {
	tm := newTestModel(t, testServers())
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("portview"))
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestCursorMovement exercises navigation deterministically at the model level,
// without depending on render timing.
func TestCursorMovement(t *testing.T) {
	m := New(mockScanner{servers: testServers()}, config.Default())
	m.servers = testServers() // 3 rows, cursor starts at 0

	down := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	up := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}

	step := func(msg tea.KeyMsg) {
		next, _ := m.Update(msg)
		m = next.(Model)
	}

	step(down)
	if m.cursor != 1 {
		t.Fatalf("after 1 down, cursor = %d, want 1", m.cursor)
	}
	step(down)
	step(down) // clamp at last index (2)
	if m.cursor != 2 {
		t.Fatalf("after 3 downs, cursor = %d, want 2 (clamped)", m.cursor)
	}
	step(up)
	if m.cursor != 1 {
		t.Fatalf("after up, cursor = %d, want 1", m.cursor)
	}
	step(up)
	step(up) // clamp at 0
	if m.cursor != 0 {
		t.Fatalf("after extra ups, cursor = %d, want 0 (clamped)", m.cursor)
	}
}

// TestRefreshIssuesCommand verifies the refresh key fires a scan command.
func TestRefreshIssuesCommand(t *testing.T) {
	m := New(mockScanner{servers: testServers()}, config.Default())
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatal("refresh key produced no command")
	}
}

// TestScanResultUpdatesModel verifies scan results populate the model and clamp
// a stale cursor.
func TestScanResultUpdatesModel(t *testing.T) {
	m := New(mockScanner{}, config.Default())
	m.cursor = 10 // stale, beyond any result

	next, _ := m.Update(scanResultMsg{servers: testServers(), at: time.Unix(1, 0)})
	m = next.(Model)

	if len(m.servers) != 3 {
		t.Fatalf("servers = %d, want 3", len(m.servers))
	}
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want clamped to 2", m.cursor)
	}
}

func TestApplyConfigHidesAndLabels(t *testing.T) {
	cfg := config.Default()
	cfg.SetLabel(3000, "frontend")
	cfg.Hide(5432)

	got := applyConfig(testServers(), cfg)
	if len(got) != 2 {
		t.Fatalf("got %d servers, want 2 (5432 hidden)", len(got))
	}
	if got[0].Port != 3000 || got[0].Label != "frontend" {
		t.Errorf("server[0] = %+v, want port 3000 labeled frontend", got[0])
	}
	for _, s := range got {
		if s.Port == 5432 {
			t.Errorf("port 5432 should be hidden, got %+v", s)
		}
	}
}
