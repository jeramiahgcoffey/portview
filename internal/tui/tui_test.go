package tui

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
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

// --- Task 3.5: Filter mode tests ---

func TestUpdate_SlashKey_EntersFilterMode(t *testing.T) {
	m := modelWithServers([]scanner.Server{{Port: 8080}})

	result, _ := m.Update(keyMsg("/"))
	updated := result.(Model)
	if updated.mode != modeFilter {
		t.Errorf("mode = %d, want modeFilter (%d)", updated.mode, modeFilter)
	}
}

func TestUpdate_FilterMode_TypingFilters(t *testing.T) {
	m := modelWithServers([]scanner.Server{
		{Port: 8080, Process: "node"},
		{Port: 3000, Process: "python"},
	})
	m.mode = modeFilter

	// Type "node"
	result, _ := m.Update(keyMsg("n"))
	m = result.(Model)
	result, _ = m.Update(keyMsg("o"))
	m = result.(Model)
	result, _ = m.Update(keyMsg("d"))
	m = result.(Model)
	result, _ = m.Update(keyMsg("e"))
	updated := result.(Model)

	if updated.filterText != "node" {
		t.Errorf("filterText = %q, want %q", updated.filterText, "node")
	}
	if len(updated.filtered) != 1 {
		t.Fatalf("expected 1 filtered server, got %d", len(updated.filtered))
	}
	if updated.filtered[0].Process != "node" {
		t.Errorf("filtered[0].Process = %q, want %q", updated.filtered[0].Process, "node")
	}
}

func TestUpdate_FilterMode_EscExits(t *testing.T) {
	m := modelWithServers([]scanner.Server{{Port: 8080}})
	m.mode = modeFilter
	m.filterText = "test"

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := result.(Model)
	if updated.mode != modeNormal {
		t.Errorf("mode = %d, want modeNormal (%d)", updated.mode, modeNormal)
	}
}

func TestUpdate_FilterMode_BackspaceRemovesChar(t *testing.T) {
	m := modelWithServers([]scanner.Server{
		{Port: 8080, Process: "node"},
		{Port: 3000, Process: "python"},
	})
	m.mode = modeFilter
	m.filterText = "node"
	m.applyFilter() // should filter to 1

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = result.(Model)
	if m.filterText != "nod" {
		t.Errorf("filterText = %q, want %q", m.filterText, "nod")
	}
}

func TestApplyFilter_MatchesPort(t *testing.T) {
	m := modelWithServers([]scanner.Server{
		{Port: 8080, Process: "node"},
		{Port: 3000, Process: "python"},
	})
	m.filterText = "8080"
	m.applyFilter()

	if len(m.filtered) != 1 {
		t.Fatalf("expected 1, got %d", len(m.filtered))
	}
	if m.filtered[0].Port != 8080 {
		t.Errorf("filtered[0].Port = %d, want 8080", m.filtered[0].Port)
	}
}

func TestApplyFilter_MatchesProcess(t *testing.T) {
	m := modelWithServers([]scanner.Server{
		{Port: 8080, Process: "node"},
		{Port: 3000, Process: "python"},
	})
	m.filterText = "python"
	m.applyFilter()

	if len(m.filtered) != 1 {
		t.Fatalf("expected 1, got %d", len(m.filtered))
	}
	if m.filtered[0].Process != "python" {
		t.Errorf("filtered[0].Process = %q, want %q", m.filtered[0].Process, "python")
	}
}

func TestApplyFilter_MatchesLabel(t *testing.T) {
	m := modelWithServers([]scanner.Server{
		{Port: 8080, Process: "node", Label: "web-api"},
		{Port: 3000, Process: "python", Label: "frontend"},
	})
	m.filterText = "web"
	m.applyFilter()

	if len(m.filtered) != 1 {
		t.Fatalf("expected 1, got %d", len(m.filtered))
	}
	if m.filtered[0].Label != "web-api" {
		t.Errorf("filtered[0].Label = %q, want %q", m.filtered[0].Label, "web-api")
	}
}

func TestApplyFilter_CaseInsensitive(t *testing.T) {
	m := modelWithServers([]scanner.Server{
		{Port: 8080, Process: "Node"},
		{Port: 3000, Process: "Python"},
	})
	m.filterText = "node"
	m.applyFilter()

	if len(m.filtered) != 1 {
		t.Fatalf("expected 1, got %d", len(m.filtered))
	}
}

func TestApplyFilter_EmptyShowsAll(t *testing.T) {
	m := modelWithServers([]scanner.Server{
		{Port: 8080}, {Port: 3000},
	})
	m.filterText = ""
	m.applyFilter()

	if len(m.filtered) != 2 {
		t.Errorf("expected 2, got %d", len(m.filtered))
	}
}

func TestApplyFilter_CursorClamped(t *testing.T) {
	m := modelWithServers([]scanner.Server{
		{Port: 8080, Process: "node"},
		{Port: 3000, Process: "python"},
		{Port: 443, Process: "nginx"},
	})
	m.cursor = 2 // pointing at nginx

	m.filterText = "node"
	m.applyFilter()

	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped after filter)", m.cursor)
	}
}

// --- Task 3.6: Kill confirmation tests ---

func TestUpdate_KillKey_EntersConfirmMode(t *testing.T) {
	m := modelWithServers([]scanner.Server{{Port: 8080, PID: 1234}})

	result, _ := m.Update(keyMsg("x"))
	updated := result.(Model)
	if updated.mode != modeConfirmKill {
		t.Errorf("mode = %d, want modeConfirmKill (%d)", updated.mode, modeConfirmKill)
	}
}

func TestUpdate_KillKey_NoServers_DoesNothing(t *testing.T) {
	m := modelWithServers([]scanner.Server{})

	result, _ := m.Update(keyMsg("x"))
	updated := result.(Model)
	if updated.mode != modeNormal {
		t.Errorf("mode = %d, want modeNormal (no servers to kill)", updated.mode)
	}
}

func TestUpdate_ConfirmKill_YConfirms(t *testing.T) {
	m := modelWithServers([]scanner.Server{{Port: 8080, PID: 1234}})
	m.mode = modeConfirmKill

	result, cmd := m.Update(keyMsg("y"))
	updated := result.(Model)
	if updated.mode != modeNormal {
		t.Errorf("mode = %d, want modeNormal after confirm", updated.mode)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd (kill command)")
	}
}

func TestUpdate_ConfirmKill_NCancels(t *testing.T) {
	m := modelWithServers([]scanner.Server{{Port: 8080, PID: 1234}})
	m.mode = modeConfirmKill

	result, _ := m.Update(keyMsg("n"))
	updated := result.(Model)
	if updated.mode != modeNormal {
		t.Errorf("mode = %d, want modeNormal after cancel", updated.mode)
	}
}

func TestUpdate_ConfirmKill_EscCancels(t *testing.T) {
	m := modelWithServers([]scanner.Server{{Port: 8080, PID: 1234}})
	m.mode = modeConfirmKill

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := result.(Model)
	if updated.mode != modeNormal {
		t.Errorf("mode = %d, want modeNormal after esc", updated.mode)
	}
}

func TestUpdate_KillResult_TriggersRefresh(t *testing.T) {
	mock := &scanner.MockScanner{Servers: []scanner.Server{{Port: 8080}}}
	cfg := config.Default()
	m := New(mock, cfg, "")

	_, cmd := m.Update(killResultMsg{pid: 1234})
	if cmd == nil {
		t.Fatal("expected non-nil cmd (rescan after kill)")
	}
}

// --- Task 3.7: Label editing tests ---

func TestUpdate_LabelKey_EntersLabelMode(t *testing.T) {
	m := modelWithServers([]scanner.Server{{Port: 8080, PID: 1234}})

	result, _ := m.Update(keyMsg("l"))
	updated := result.(Model)
	if updated.mode != modeLabel {
		t.Errorf("mode = %d, want modeLabel (%d)", updated.mode, modeLabel)
	}
}

func TestUpdate_LabelKey_NoServers_DoesNothing(t *testing.T) {
	m := modelWithServers([]scanner.Server{})

	result, _ := m.Update(keyMsg("l"))
	updated := result.(Model)
	if updated.mode != modeNormal {
		t.Errorf("mode = %d, want modeNormal (no servers to label)", updated.mode)
	}
}

func TestUpdate_LabelMode_EscCancels(t *testing.T) {
	m := modelWithServers([]scanner.Server{{Port: 8080}})
	m.mode = modeLabel

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := result.(Model)
	if updated.mode != modeNormal {
		t.Errorf("mode = %d, want modeNormal after esc", updated.mode)
	}
}

func TestUpdate_LabelMode_EnterSavesLabel(t *testing.T) {
	m := modelWithServers([]scanner.Server{{Port: 8080}})
	m.mode = modeLabel
	m.configPath = "/tmp/test-label-config.yaml"
	m.labelInput.SetValue("my-api")

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(Model)
	if updated.mode != modeNormal {
		t.Errorf("mode = %d, want modeNormal after enter", updated.mode)
	}
	if updated.config.Labels[8080] != "my-api" {
		t.Errorf("Labels[8080] = %q, want %q", updated.config.Labels[8080], "my-api")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd (save config)")
	}
}

func TestUpdate_LabelMode_PrePopulatesExistingLabel(t *testing.T) {
	m := modelWithServers([]scanner.Server{{Port: 8080}})
	m.config.SetLabel(8080, "existing-label")

	// Trigger label mode
	result, _ := m.Update(keyMsg("l"))
	updated := result.(Model)

	if updated.labelInput.Value() != "existing-label" {
		t.Errorf("labelInput.Value() = %q, want %q", updated.labelInput.Value(), "existing-label")
	}
}

// --- Task 3.8: Help, open, refresh tests ---

func TestUpdate_HelpKey_TogglesHelpMode(t *testing.T) {
	m := modelWithServers([]scanner.Server{{Port: 8080}})

	result, _ := m.Update(keyMsg("?"))
	updated := result.(Model)
	if updated.mode != modeHelp {
		t.Errorf("mode = %d, want modeHelp (%d)", updated.mode, modeHelp)
	}
}

func TestUpdate_HelpMode_AnyKeyExits(t *testing.T) {
	m := modelWithServers([]scanner.Server{{Port: 8080}})
	m.mode = modeHelp

	result, _ := m.Update(keyMsg("a"))
	updated := result.(Model)
	if updated.mode != modeNormal {
		t.Errorf("mode = %d, want modeNormal after key in help mode", updated.mode)
	}
}

func TestUpdate_OpenKey_ReturnsCommand(t *testing.T) {
	m := modelWithServers([]scanner.Server{{Port: 8080}})

	_, cmd := m.Update(keyMsg("o"))
	if cmd == nil {
		t.Fatal("expected non-nil cmd (open browser)")
	}
}

func TestUpdate_OpenKey_NoServers_DoesNothing(t *testing.T) {
	m := modelWithServers([]scanner.Server{})

	_, cmd := m.Update(keyMsg("o"))
	if cmd != nil {
		t.Error("expected nil cmd when no servers")
	}
}

func TestUpdate_RefreshKey_TriggersScan(t *testing.T) {
	mock := &scanner.MockScanner{Servers: []scanner.Server{{Port: 8080}}}
	cfg := config.Default()
	m := New(mock, cfg, "")
	m.servers = []scanner.Server{{Port: 8080}}
	m.filtered = []scanner.Server{{Port: 8080}}

	_, cmd := m.Update(keyMsg("r"))
	if cmd == nil {
		t.Fatal("expected non-nil cmd (manual refresh)")
	}
}

// --- Task 4.1: View tests ---

func TestView_ShowsHeader(t *testing.T) {
	m := modelWithServers([]scanner.Server{{Port: 8080, Process: "node"}})
	view := m.View()

	if !strings.Contains(view, "portview") {
		t.Error("view should contain 'portview' header")
	}
	if !strings.Contains(view, "PORT") {
		t.Error("view should contain 'PORT' column header")
	}
	if !strings.Contains(view, "PID") {
		t.Error("view should contain 'PID' column header")
	}
	if !strings.Contains(view, "PROCESS") {
		t.Error("view should contain 'PROCESS' column header")
	}
}

func TestView_ShowsServers(t *testing.T) {
	m := modelWithServers([]scanner.Server{
		{Port: 8080, PID: 1234, Process: "node"},
		{Port: 3000, PID: 5678, Process: "python"},
	})
	view := m.View()

	if !strings.Contains(view, "8080") {
		t.Error("view should contain port 8080")
	}
	if !strings.Contains(view, "3000") {
		t.Error("view should contain port 3000")
	}
	if !strings.Contains(view, "node") {
		t.Error("view should contain process 'node'")
	}
	if !strings.Contains(view, "python") {
		t.Error("view should contain process 'python'")
	}
}

func TestView_CursorIndicator(t *testing.T) {
	m := modelWithServers([]scanner.Server{
		{Port: 8080}, {Port: 3000},
	})
	m.cursor = 0
	view := m.View()

	if !strings.Contains(view, ">") {
		t.Error("view should contain cursor indicator '>'")
	}
}

func TestView_EmptyServerList(t *testing.T) {
	m := modelWithServers([]scanner.Server{})
	view := m.View()

	if !strings.Contains(view, "No servers found") {
		t.Error("view should show 'No servers found' for empty list")
	}
}

func TestView_TruncatesLongCommand(t *testing.T) {
	longCmd := strings.Repeat("a", 100)
	m := modelWithServers([]scanner.Server{
		{Port: 8080, PID: 1234, Process: "node", Command: longCmd},
	})
	view := m.View()

	// The full 100-char command should not appear
	if strings.Contains(view, longCmd) {
		t.Error("view should truncate long commands")
	}
}

// --- Task 4.2: Status bar tests ---

func TestStatusBar_ShowsServerCount(t *testing.T) {
	m := modelWithServers([]scanner.Server{
		{Port: 8080}, {Port: 3000}, {Port: 443},
	})
	bar := m.statusBar()

	if !strings.Contains(bar, "3 servers") {
		t.Errorf("status bar should contain '3 servers', got: %s", bar)
	}
}

func TestStatusBar_ConfirmKillMode(t *testing.T) {
	m := modelWithServers([]scanner.Server{{Port: 8080, PID: 1234}})
	m.mode = modeConfirmKill
	bar := m.statusBar()

	if !strings.Contains(bar, "Kill PID 1234") {
		t.Errorf("status bar should show kill confirmation, got: %s", bar)
	}
	if !strings.Contains(bar, "y/n") {
		t.Errorf("status bar should show y/n prompt, got: %s", bar)
	}
}

func TestStatusBar_ShowsKeybindHints(t *testing.T) {
	m := modelWithServers([]scanner.Server{{Port: 8080}})
	bar := m.statusBar()

	for _, hint := range []string{"j/k:nav", "o:open", "x:kill", "q:quit"} {
		if !strings.Contains(bar, hint) {
			t.Errorf("status bar should contain hint %q, got: %s", hint, bar)
		}
	}
}

// --- Task 4.3: Filter bar tests ---

func TestView_FilterMode_ShowsFilterBar(t *testing.T) {
	m := modelWithServers([]scanner.Server{{Port: 8080}})
	m.mode = modeFilter
	m.filterText = "node"
	view := m.View()

	if !strings.Contains(view, "Filter:") {
		t.Error("view should show filter bar in filter mode")
	}
	if !strings.Contains(view, "node") {
		t.Error("view should show filter text")
	}
}

func TestView_ActiveFilter_ShowsIndicator(t *testing.T) {
	m := modelWithServers([]scanner.Server{{Port: 8080}})
	m.mode = modeNormal
	m.filterText = "test"
	view := m.View()

	if !strings.Contains(view, "Filter:") {
		t.Error("view should show active filter indicator when filterText is set")
	}
}

func TestView_NoFilter_NoFilterBar(t *testing.T) {
	m := modelWithServers([]scanner.Server{{Port: 8080}})
	m.mode = modeNormal
	m.filterText = ""
	view := m.View()

	if strings.Contains(view, "Filter:") {
		t.Error("view should not show filter bar when no filter is active")
	}
}

// --- Task 4.4: Help overlay tests ---

func TestView_HelpMode_ShowsOverlay(t *testing.T) {
	m := modelWithServers([]scanner.Server{{Port: 8080}})
	m.mode = modeHelp
	view := m.View()

	if !strings.Contains(view, "Help") {
		t.Error("view should show 'Help' in help mode")
	}
}

func TestView_HelpMode_ShowsAllBindings(t *testing.T) {
	m := modelWithServers([]scanner.Server{{Port: 8080}})
	m.mode = modeHelp
	view := m.View()

	for _, binding := range []string{"Navigate", "Open in browser", "Kill process", "Edit label", "Refresh", "Filter", "Quit"} {
		if !strings.Contains(view, binding) {
			t.Errorf("help overlay should contain %q", binding)
		}
	}
}

func TestView_NormalMode_NoHelpOverlay(t *testing.T) {
	m := modelWithServers([]scanner.Server{{Port: 8080}})
	m.mode = modeNormal
	view := m.View()

	if strings.Contains(view, "Press any key to close") {
		t.Error("help overlay should not appear in normal mode")
	}
}

// --- Task 5.1: Merge labels and filter hidden tests ---

func TestMergeLabels_AppliesConfigLabels(t *testing.T) {
	m := modelWithServers([]scanner.Server{
		{Port: 8080, Process: "node"},
		{Port: 3000, Process: "python"},
	})
	m.config.SetLabel(8080, "web-api")
	m.mergeLabels()

	if m.servers[0].Label != "web-api" {
		t.Errorf("servers[0].Label = %q, want %q", m.servers[0].Label, "web-api")
	}
	if m.servers[1].Label != "" {
		t.Errorf("servers[1].Label = %q, want empty", m.servers[1].Label)
	}
}

func TestMergeLabels_NoLabel_LeavesEmpty(t *testing.T) {
	m := modelWithServers([]scanner.Server{
		{Port: 8080, Process: "node"},
	})
	m.mergeLabels()

	if m.servers[0].Label != "" {
		t.Errorf("servers[0].Label = %q, want empty", m.servers[0].Label)
	}
}

func TestFilterHidden_RemovesHiddenPorts(t *testing.T) {
	m := modelWithServers([]scanner.Server{
		{Port: 8080}, {Port: 22}, {Port: 3000},
	})
	m.config.ToggleHidden(22)
	m.filterHidden()

	if len(m.servers) != 2 {
		t.Fatalf("expected 2 servers after hiding port 22, got %d", len(m.servers))
	}
	for _, s := range m.servers {
		if s.Port == 22 {
			t.Error("port 22 should be hidden")
		}
	}
}

func TestFilterHidden_NoHidden_KeepsAll(t *testing.T) {
	m := modelWithServers([]scanner.Server{
		{Port: 8080}, {Port: 3000},
	})
	m.filterHidden()

	if len(m.servers) != 2 {
		t.Errorf("expected 2 servers (none hidden), got %d", len(m.servers))
	}
}

// --- Task 7.1: Integration test ---

func TestTUI_FullFlow(t *testing.T) {
	mock := &scanner.MockScanner{
		Servers: []scanner.Server{
			{Port: 8080, PID: 1234, Process: "node", Command: "node server.js", State: "LISTEN", Healthy: true},
			{Port: 3000, PID: 5678, Process: "python", Command: "python app.py", State: "LISTEN"},
			{Port: 443, PID: 9012, Process: "nginx", Command: "nginx -g daemon off", State: "LISTEN", Healthy: true},
		},
	}
	cfg := config.Default()
	m := New(mock, cfg, "")

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))

	// Wait for servers to appear in output
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("8080")) &&
			bytes.Contains(bts, []byte("3000")) &&
			bytes.Contains(bts, []byte("443"))
	}, teatest.WithCheckInterval(100*time.Millisecond),
		teatest.WithDuration(5*time.Second))

	// Navigate down
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})

	// Quit
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}
