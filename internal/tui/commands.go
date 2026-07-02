package tui

import (
	"context"
	"strconv"
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
	err error
}

// detailResultMsg carries the on-demand insight for one server.
type detailResultMsg struct {
	port   int
	detail scanner.Detail
	probe  scanner.HTTPProbe
	err    error
}

// scanCmd runs a single scan, resolves docker containers, and merges config
// (hidden filter + labels) into the result. It is fired both by the poll
// ticker and by manual refresh.
func scanCmd(s scanner.Scanner, cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		servers, err := s.Scan(ctx)
		if err == nil {
			servers = docker.Enrich(ctx, servers)
			servers = applyConfig(servers, cfg)
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

// saveCmd persists the config to disk (lazily creating the file).
func saveCmd(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		return saveResultMsg{err: cfg.Save()}
	}
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

// applyConfig is the app-logic merge step: drop hidden ports and attach the
// user's saved label to each remaining server. It delegates to config.Decorate
// so the TUI and CLI share one filtering implementation.
func applyConfig(in []scanner.Server, cfg config.Config) []scanner.Server {
	return cfg.Decorate(in, false)
}
