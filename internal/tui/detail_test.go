package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeramiahgcoffey/portview/internal/config"
	"github.com/jeramiahgcoffey/portview/internal/scanner"
)

func TestInspectFlow(t *testing.T) {
	m := newModelWithServers() // cursor 0 -> port 3000

	m, cmd := drive(m, runes("i"))
	if m.mode != modeDetail {
		t.Fatalf("mode = %v, want detail", m.mode)
	}
	if m.detailTarget.Port != 3000 {
		t.Fatalf("detailTarget.Port = %d, want 3000", m.detailTarget.Port)
	}
	if !m.detailLoading {
		t.Fatal("detailLoading = false, want true while inspecting")
	}
	if cmd == nil {
		t.Fatal("inspect produced no command")
	}

	// Result for the open pane populates it.
	next, _ := m.Update(detailResultMsg{
		port:   3000,
		detail: scanner.Detail{CWD: "/proj", Uptime: 90 * time.Second, CPUPercent: 1.5, RSSKB: 2048},
		probe:  scanner.HTTPProbe{OK: true, Status: 200, Latency: 12 * time.Millisecond},
	})
	m = next.(Model)
	if m.detailLoading {
		t.Fatal("detailLoading = true after result, want false")
	}
	if m.detail.CWD != "/proj" {
		t.Errorf("detail.CWD = %q, want /proj", m.detail.CWD)
	}
	if !m.detailProbe.OK || m.detailProbe.Status != 200 {
		t.Errorf("probe = %+v, want OK/200", m.detailProbe)
	}

	// The pane renders the insight.
	view := m.View()
	for _, want := range []string{":3000", "/proj", "1m30s", "2 MB", "200"} {
		if !strings.Contains(view, want) {
			t.Errorf("detail view missing %q\n%s", want, view)
		}
	}

	// Esc closes the pane.
	m, _ = drive(m, keyMsg(tea.KeyEsc))
	if m.mode != modeNormal {
		t.Fatalf("after esc, mode = %v, want normal", m.mode)
	}
}

func TestInspectStaleResultIgnored(t *testing.T) {
	m := newModelWithServers()
	m, _ = drive(m, runes("i")) // inspecting port 3000

	next, _ := m.Update(detailResultMsg{port: 9999, detail: scanner.Detail{CWD: "/stale"}})
	m = next.(Model)
	if !m.detailLoading {
		t.Fatal("stale result cleared loading state")
	}
	if m.detail.CWD == "/stale" {
		t.Fatal("stale result populated the pane")
	}
}

func TestInspectErrorRenders(t *testing.T) {
	m := newModelWithServers()
	m, _ = drive(m, runes("i"))

	next, _ := m.Update(detailResultMsg{port: 3000, err: errors.New("boom")})
	m = next.(Model)
	if !strings.Contains(m.View(), "inspect failed: boom") {
		t.Errorf("view missing inspect error:\n%s", m.View())
	}
}

func TestInspectOnEmptyListIsNoop(t *testing.T) {
	m := New(mockScanner{}, config.Default())
	m, _ = drive(m, runes("i"))
	if m.mode != modeNormal {
		t.Errorf("mode = %v, want normal (nothing to inspect)", m.mode)
	}
}

func dockerServers() []scanner.Server {
	return []scanner.Server{
		{Port: 5432, PID: 42, Process: "com.docker.backend", Command: "/opt/docker",
			Container: "my-postgres", Image: "postgres:16", Healthy: true},
	}
}

func TestDockerRowShowsContainer(t *testing.T) {
	m := New(mockScanner{}, config.Default())
	m.servers = dockerServers()

	view := m.View()
	if !strings.Contains(view, "my-postgres") {
		t.Errorf("view missing container name:\n%s", view)
	}
	if !strings.Contains(view, "docker: postgres:16") {
		t.Errorf("view missing docker image:\n%s", view)
	}
}

func TestDockerKillPromptSaysStopContainer(t *testing.T) {
	m := New(mockScanner{}, config.Default())
	m.servers = dockerServers()

	m, _ = drive(m, runes("x"))
	if m.mode != modeConfirmKill {
		t.Fatalf("mode = %v, want confirmKill", m.mode)
	}
	view := m.View()
	if !strings.Contains(view, "Stop container my-postgres") {
		t.Errorf("prompt should offer docker stop, got:\n%s", view)
	}
	if strings.Contains(view, "Kill PID") {
		t.Errorf("docker rows must not prompt a raw PID kill:\n%s", view)
	}
}

func TestFilterMatchesContainerAndImage(t *testing.T) {
	s := scanner.Server{Port: 5432, Process: "com.docker.backend", Container: "my-postgres", Image: "postgres:16"}
	if !matchesFilter(s, "my-post") {
		t.Error("filter should match container name")
	}
	if !matchesFilter(s, "postgres:16") {
		t.Error("filter should match image")
	}
	if matchesFilter(s, "redis") {
		t.Error("filter should not match unrelated query")
	}
}
