package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeramiahgcoffey/portview/internal/config"
)

// runes builds a rune key message (typed characters).
func runes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// key builds a special key message.
func keyMsg(t tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: t}
}

// drive applies a key to the model and returns the updated model and command.
func drive(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

func newModelWithServers() Model {
	m := New(mockScanner{servers: testServers()}, config.Default())
	m.servers = testServers()
	return m
}

func TestKillConfirmFlow(t *testing.T) {
	m := newModelWithServers() // cursor 0 -> port 3000, pid 1

	m, _ = drive(m, runes("x"))
	if m.mode != modeConfirmKill {
		t.Fatalf("mode = %v, want confirmKill", m.mode)
	}
	if m.killTarget.PID != 1 {
		t.Fatalf("killTarget PID = %d, want 1", m.killTarget.PID)
	}

	// Decline.
	m, _ = drive(m, runes("n"))
	if m.mode != modeNormal {
		t.Fatalf("after n, mode = %v, want normal", m.mode)
	}

	// Confirm fires a kill command.
	m, _ = drive(m, runes("x"))
	m, cmd := drive(m, runes("y"))
	if m.mode != modeNormal {
		t.Fatalf("after y, mode = %v, want normal", m.mode)
	}
	if cmd == nil {
		t.Fatal("confirm produced no kill command")
	}
}

func TestKillOnEmptyListIsNoop(t *testing.T) {
	m := New(mockScanner{}, config.Default()) // no servers
	m, _ = drive(m, runes("x"))
	if m.mode != modeNormal {
		t.Errorf("mode = %v, want normal (nothing to kill)", m.mode)
	}
}

func TestLabelEditSavesConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := newModelWithServers() // cursor 0 -> port 3000

	m, _ = drive(m, runes("l"))
	if m.mode != modeLabel || m.editPort != 3000 {
		t.Fatalf("mode = %v editPort = %d, want label/3000", m.mode, m.editPort)
	}

	m, _ = drive(m, runes("frontend"))
	m, cmd := drive(m, keyMsg(tea.KeyEnter))

	if m.mode != modeNormal {
		t.Fatalf("after enter, mode = %v, want normal", m.mode)
	}
	if m.cfg.LabelFor(3000) != "frontend" {
		t.Errorf("cfg label = %q, want frontend", m.cfg.LabelFor(3000))
	}
	if m.servers[0].Label != "frontend" {
		t.Errorf("server[0].Label = %q, want frontend (re-decorated)", m.servers[0].Label)
	}

	// Persisting the config should write the file.
	if cmd == nil {
		t.Fatal("label save produced no command")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("save command returned nil message")
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if loaded.LabelFor(3000) != "frontend" {
		t.Errorf("persisted label = %q, want frontend", loaded.LabelFor(3000))
	}
}

func TestLabelEditEscCancels(t *testing.T) {
	m := newModelWithServers()
	m, _ = drive(m, runes("l"))
	m, _ = drive(m, runes("oops"))
	m, _ = drive(m, keyMsg(tea.KeyEsc))

	if m.mode != modeNormal {
		t.Fatalf("mode = %v, want normal", m.mode)
	}
	if m.cfg.LabelFor(3000) != "" {
		t.Errorf("label = %q, want unchanged (cancelled)", m.cfg.LabelFor(3000))
	}
}

func TestHideSelectedPersistsAndRemovesFromDefaultView(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := newModelWithServers() // cursor 0 -> port 3000

	m, cmd := drive(m, runes("h"))
	if !m.cfg.IsHidden(3000) {
		t.Fatalf("config hidden = %v, want 3000", m.cfg.Hidden)
	}
	if !m.servers[0].Hidden {
		t.Fatal("decorated server not marked hidden")
	}
	if len(m.visibleServers()) != 2 {
		t.Fatalf("visible servers = %d, want 2 after hiding 3000", len(m.visibleServers()))
	}
	if cmd == nil {
		t.Fatal("hide produced no save command")
	}
	_ = cmd()

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !loaded.IsHidden(3000) {
		t.Fatalf("persisted hidden = %v, want 3000", loaded.Hidden)
	}
}

func TestConfigSaverNewestRevisionWins(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	saver := &configSaver{}

	hidden := config.Default()
	hidden.Hide(3000)
	staleCmd := saver.command(hidden)

	visible := config.Default()
	latestCmd := saver.command(visible)

	// Execute in the worst possible order: newest first, then the stale command
	// that would overwrite it without revision ordering.
	latestResult := latestCmd().(saveResultMsg)
	if latestResult.err != nil {
		t.Fatalf("latest save: %v", latestResult.err)
	}
	staleResult := staleCmd().(saveResultMsg)
	if staleResult.err != nil {
		t.Fatalf("stale save: %v", staleResult.err)
	}

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.IsHidden(3000) {
		t.Fatalf("stale save won: hidden = %v", loaded.Hidden)
	}
}

func TestConfigSaverCapturesImmutableSnapshot(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	saver := &configSaver{}
	cfg := config.Default()
	cfg.Hide(3000)
	cfg.SetLabel(3000, "frontend")
	cmd := saver.command(cfg)

	// Mutating the model config after scheduling must not alter the command's
	// snapshot through shared slice/map backing storage.
	cfg.Unhide(3000)
	cfg.SetLabel(3000, "changed")
	result := cmd().(saveResultMsg)
	if result.err != nil {
		t.Fatalf("save: %v", result.err)
	}

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !loaded.IsHidden(3000) || loaded.LabelFor(3000) != "frontend" {
		t.Fatalf("saved snapshot = hidden %v, label %q; want hidden/frontend",
			loaded.Hidden, loaded.LabelFor(3000))
	}
}

func TestShowHiddenAndUnhideSelected(t *testing.T) {
	cfg := config.Default()
	cfg.Hide(5432)
	m := New(mockScanner{servers: testServers()}, cfg)
	m.servers = applyConfig(testServers(), cfg)

	if len(m.visibleServers()) != 2 {
		t.Fatalf("default visible = %d, want 2", len(m.visibleServers()))
	}
	m, _ = drive(m, runes("a"))
	if !m.showHidden || len(m.visibleServers()) != 3 {
		t.Fatalf("showHidden = %v, visible = %d; want true/3", m.showHidden, len(m.visibleServers()))
	}

	m, _ = drive(m, runes("j")) // port 5432
	selected, ok := m.selected()
	if !ok || selected.Port != 5432 || !selected.Hidden {
		t.Fatalf("selected = %+v, want hidden port 5432", selected)
	}
	m, cmd := drive(m, runes("h"))
	if cmd == nil {
		t.Fatal("unhide produced no save command")
	}
	if m.cfg.IsHidden(5432) || m.servers[1].Hidden {
		t.Fatalf("port remained hidden: cfg=%v server=%+v", m.cfg.Hidden, m.servers[1])
	}
	if len(m.visibleServers()) != 3 {
		t.Fatalf("visible = %d, want 3 after unhide", len(m.visibleServers()))
	}
}

func TestHideAndShowHiddenOnEmptyListAreNoops(t *testing.T) {
	m := New(mockScanner{}, config.Default())
	m, cmd := drive(m, runes("h"))
	if cmd != nil || len(m.cfg.Hidden) != 0 {
		t.Fatalf("hide empty produced cmd/config mutation: cmd=%v hidden=%v", cmd != nil, m.cfg.Hidden)
	}
	m, _ = drive(m, runes("a"))
	if m.showHidden {
		t.Fatal("showHidden = true with no hidden listeners")
	}
}

func TestHiddenRowIsExplicitlyMarked(t *testing.T) {
	cfg := config.Default()
	cfg.Hide(3000)
	m := New(mockScanner{}, cfg)
	m.servers = applyConfig(testServers(), cfg)
	m.showHidden = true

	view := m.View()
	if !strings.Contains(view, "[hidden]") {
		t.Fatalf("hidden row lacks explicit marker:\n%s", view)
	}
}

func TestFilterNarrowsList(t *testing.T) {
	m := newModelWithServers() // node, postgres, go

	m, _ = drive(m, runes("/"))
	if m.mode != modeFilter {
		t.Fatalf("mode = %v, want filter", m.mode)
	}

	m, _ = drive(m, runes("postgres"))
	vis := m.visibleServers()
	if len(vis) != 1 || vis[0].Process != "postgres" {
		t.Fatalf("filtered = %+v, want only postgres", vis)
	}

	// Enter keeps the filter applied in normal mode.
	m, _ = drive(m, keyMsg(tea.KeyEnter))
	if m.mode != modeNormal || m.filter != "postgres" {
		t.Fatalf("mode = %v filter = %q, want normal/postgres", m.mode, m.filter)
	}

	// Esc in normal mode clears the active filter.
	m, _ = drive(m, keyMsg(tea.KeyEsc))
	if m.filter != "" {
		t.Errorf("filter = %q, want cleared", m.filter)
	}
	if len(m.visibleServers()) != 3 {
		t.Errorf("visible = %d, want all 3 after clear", len(m.visibleServers()))
	}
}

func TestFilterByPort(t *testing.T) {
	m := newModelWithServers()
	m, _ = drive(m, runes("/"))
	m, _ = drive(m, runes("8080"))
	vis := m.visibleServers()
	if len(vis) != 1 || vis[0].Port != 8080 {
		t.Fatalf("filtered = %+v, want only 8080", vis)
	}
}

func TestFilterEscDuringInputClears(t *testing.T) {
	m := newModelWithServers()
	m, _ = drive(m, runes("/"))
	m, _ = drive(m, runes("node"))
	m, _ = drive(m, keyMsg(tea.KeyEsc))
	if m.mode != modeNormal || m.filter != "" {
		t.Errorf("mode = %v filter = %q, want normal/empty", m.mode, m.filter)
	}
}

func TestHelpToggle(t *testing.T) {
	m := newModelWithServers()
	m, _ = drive(m, runes("?"))
	if !m.showHelp {
		t.Fatal("showHelp = false, want true")
	}
	m, _ = drive(m, runes("?"))
	if m.showHelp {
		t.Fatal("showHelp = true, want toggled off")
	}
	// Esc also closes help.
	m, _ = drive(m, runes("?"))
	m, _ = drive(m, keyMsg(tea.KeyEsc))
	if m.showHelp {
		t.Fatal("showHelp = true after esc, want false")
	}
}

func TestOpenIssuesCommand(t *testing.T) {
	m := newModelWithServers()
	_, cmd := drive(m, runes("o"))
	if cmd == nil {
		t.Fatal("open key produced no command")
	}
}
