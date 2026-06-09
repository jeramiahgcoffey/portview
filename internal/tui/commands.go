package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeramiahgcoffey/portview/internal/config"
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

// scanCmd runs a single scan and merges config (hidden filter + labels) into
// the result. It is fired both by the poll ticker and by manual refresh.
func scanCmd(s scanner.Scanner, cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		servers, err := s.Scan(context.Background())
		if err == nil {
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

// applyConfig is the app-logic merge step: drop hidden ports and attach the
// user's saved label to each remaining server.
func applyConfig(in []scanner.Server, cfg config.Config) []scanner.Server {
	out := make([]scanner.Server, 0, len(in))
	for _, s := range in {
		if cfg.IsHidden(s.Port) {
			continue
		}
		s.Label = cfg.LabelFor(s.Port)
		out = append(out, s)
	}
	return out
}
