package tui

import (
	"context"
	"strconv"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeramiahgcoffey/portview/internal/action"
	"github.com/jeramiahgcoffey/portview/internal/config"
	"github.com/jeramiahgcoffey/portview/internal/docker"
	"github.com/jeramiahgcoffey/portview/internal/scanner"
)

// scanResultMsg carries the outcome of a scan back to the model.
type scanResultMsg struct {
	servers []scanner.Server
	err     error
	at      time.Time
}

// tickMsg fires on each poll-loop interval.
type tickMsg time.Time

// openResultMsg reports the outcome of opening a port in the browser.
type openResultMsg struct {
	port int
	err  error
}

// killResultMsg reports the outcome of terminating a server — either a
// SIGTERM to a process or a docker stop. desc describes what was done, for
// the status line.
type killResultMsg struct {
	desc string
	err  error
}

// saveResultMsg reports the outcome of persisting config.
type saveResultMsg struct {
	cfg       config.Config
	revision  uint64
	hasConfig bool
	err       error
}

// detailResultMsg carries the on-demand insight for one server.
type detailResultMsg struct {
	port   int
	detail scanner.Detail
	probe  scanner.HTTPProbe
	err    error
}

// scanCmd runs a single scan and resolves docker containers. The model applies
// its current config when it receives the result, avoiding stale decoration
// when a label or hidden-port edit races an in-flight scan.
func scanCmd(s scanner.Scanner) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		servers, err := s.Scan(ctx)
		if err == nil {
			servers = docker.Enrich(ctx, servers)
		}
		return scanResultMsg{servers: servers, err: err, at: time.Now()}
	}
}

// tickCmd schedules the next poll-loop tick.
func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// openCmd opens http://localhost:<port> in the default browser.
func openCmd(port int) tea.Cmd {
	return func() tea.Msg {
		return openResultMsg{port: port, err: action.OpenBrowser(port)}
	}
}

// killCmd terminates a server: docker stop when it is a container-published
// port (SIGTERM to Docker's proxy would take down the daemon, not the
// container), SIGTERM to the owning process otherwise.
func killCmd(target scanner.Server) tea.Cmd {
	return func() tea.Msg {
		if target.Container != "" {
			err := docker.Stop(context.Background(), target.Container)
			return killResultMsg{desc: "stopped container " + target.Container, err: err}
		}
		err := action.Kill(target.PID)
		return killResultMsg{desc: "sent SIGTERM to PID " + strconv.Itoa(target.PID), err: err}
	}
}

type configEditKind uint8

const (
	configEditHide configEditKind = iota
	configEditUnhide
	configEditLabel
)

type configEdit struct {
	kind  configEditKind
	port  int
	label string
}

func (e configEdit) apply(cfg *config.Config) {
	switch e.kind {
	case configEditHide:
		cfg.Hide(e.port)
	case configEditUnhide:
		cfg.Unhide(e.port)
	case configEditLabel:
		cfg.SetLabel(e.port, e.label)
	}
}

type versionedConfigEdit struct {
	revision uint64
	edit     configEdit
}

// configSaver queues preference-level edits, then serializes locked
// read-modify-write transactions. Any Tea command may execute first; it drains
// all edits scheduled so far in revision order, preserving both rapid in-TUI
// changes and unrelated writes from other portview processes.
type configSaver struct {
	mu                sync.Mutex
	nextRevision      uint64
	persistedRevision uint64
	pending           []versionedConfigEdit
}

// command queues one immutable config edit and returns a command that can drain
// the queue. The revision lets the model ignore an older save result when a
// newer local edit is already pending.
func (s *configSaver) command(edit configEdit) (tea.Cmd, uint64) {
	s.mu.Lock()
	s.nextRevision++
	revision := s.nextRevision
	s.pending = append(s.pending, versionedConfigEdit{revision: revision, edit: edit})
	s.mu.Unlock()

	return func() tea.Msg {
		s.mu.Lock()
		defer s.mu.Unlock()
		if revision <= s.persistedRevision {
			return saveResultMsg{}
		}

		pending := append([]versionedConfigEdit(nil), s.pending...)
		lastRevision := pending[len(pending)-1].revision
		updated, _, err := config.Update(func(current *config.Config) bool {
			for _, item := range pending {
				item.edit.apply(current)
			}
			return len(pending) > 0
		})
		if err != nil {
			return saveResultMsg{revision: lastRevision, err: err}
		}

		s.pending = nil
		s.persistedRevision = lastRevision
		return saveResultMsg{
			cfg:       updated,
			revision:  lastRevision,
			hasConfig: true,
		}
	}, revision
}

func hideEdit(port int, hidden bool) configEdit {
	if hidden {
		return configEdit{kind: configEditHide, port: port}
	}
	return configEdit{kind: configEditUnhide, port: port}
}

// inspectCmd gathers the insight pane's data for one server: process detail
// via the scanner and a one-shot HTTP probe of the port. Both are on-demand
// only — the poll loop never runs them.
func inspectCmd(s scanner.Server) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		detail, err := scanner.Inspect(ctx, s.PID)
		probe := scanner.ProbeHTTP(ctx, s.Port, scanner.DefaultProbeTimeout)
		return detailResultMsg{port: s.Port, detail: detail, probe: probe, err: err}
	}
}

// applyConfig is the app-logic merge step: retain hidden ports while attaching
// saved labels and hidden state. Model.visibleServers owns display filtering.
func applyConfig(in []scanner.Server, cfg config.Config) []scanner.Server {
	return cfg.Decorate(in, true)
}
