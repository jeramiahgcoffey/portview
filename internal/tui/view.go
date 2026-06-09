package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeramiahgcoffey/portview/internal/scanner"
)

// Layout constants. The COMMAND column flexes to fill remaining width.
const (
	defaultWidth = 72
	colCursor    = 2
	colPort      = 6
	colProcess   = 14
	colLabel     = 12
	colGap       = 1 // space between columns
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true)
	headerStyle  = lipgloss.NewStyle().Bold(true).Faint(true)
	cursorStyle  = lipgloss.NewStyle().Bold(true)
	healthyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green
	deadStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
	labelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("6")) // cyan
	statusStyle  = lipgloss.NewStyle().Faint(true)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1")) // red
	promptStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
	borderStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
)

// View renders the full TUI per the design doc's layout sketch.
func (m Model) View() string {
	inner := m.innerWidth()

	var b strings.Builder
	b.WriteString(titleStyle.Render("portview"))
	b.WriteByte('\n')

	if m.showHelp {
		b.WriteString(m.help.FullHelpView(m.keys.FullHelp()))
		return borderStyle.Width(inner).Render(b.String())
	}

	if m.mode == modeFilter {
		b.WriteString(promptStyle.Render(m.filterInput.View()))
		b.WriteByte('\n')
	}

	b.WriteString(m.renderHeader(inner))
	b.WriteByte('\n')
	b.WriteString(m.renderList(inner))
	b.WriteByte('\n')
	b.WriteString(strings.Repeat("─", inner))
	b.WriteByte('\n')
	b.WriteString(m.renderStatus())

	return borderStyle.Width(inner).Render(b.String())
}

// innerWidth is the content width inside the border.
func (m Model) innerWidth() int {
	w := m.width
	if w <= 0 {
		w = defaultWidth
	}
	w -= 4 // border + breathing room
	if w < 40 {
		w = 40
	}
	return w
}

// commandWidth is the flexible COMMAND column width for a given inner width.
func commandWidth(inner int) int {
	used := colCursor + colPort + colProcess + colLabel + 3*colGap
	w := inner - used
	if w < 10 {
		w = 10
	}
	return w
}

func (m Model) renderHeader(inner int) string {
	cmdW := commandWidth(inner)
	row := fmt.Sprintf("%-*s%-*s%-*s%-*s%-*s",
		colCursor, "",
		colPort+colGap, "PORT",
		colProcess+colGap, "PROCESS",
		cmdW+colGap, "COMMAND",
		colLabel, "LABEL",
	)
	return headerStyle.Render(row)
}

func (m Model) renderList(inner int) string {
	vis := m.visibleServers()
	if len(vis) == 0 {
		msg := "no servers found"
		switch {
		case m.scanErr != nil:
			msg = "scan error"
		case m.filter != "":
			msg = "no matches for " + strconv.Quote(m.filter)
		}
		return statusStyle.Render("  " + msg)
	}

	cmdW := commandWidth(inner)
	rows := make([]string, 0, len(vis))
	for i, s := range vis {
		rows = append(rows, m.renderRow(s, i == m.cursor, cmdW))
	}
	return strings.Join(rows, "\n")
}

func (m Model) renderRow(s scanner.Server, selected bool, cmdW int) string {
	cursor := "  "
	if selected {
		cursor = cursorStyle.Render("► ")
	}

	port := fmt.Sprintf("%-*d", colPort+colGap, s.Port)
	if s.Healthy {
		port = healthyStyle.Render(port)
	} else {
		port = deadStyle.Render(port)
	}

	process := fmt.Sprintf("%-*s", colProcess+colGap, truncate(s.Process, colProcess))
	command := fmt.Sprintf("%-*s", cmdW+colGap, truncate(s.Command, cmdW))

	var label string
	if selected && m.mode == modeLabel && s.Port == m.editPort {
		// Inline editor replaces the label cell.
		label = m.labelInput.View()
	} else {
		label = labelStyle.Render(truncate(s.Label, colLabel))
	}

	return cursor + port + process + command + label
}

func (m Model) renderStatus() string {
	hints := "↑↓/jk:nav  o:open  x:kill  l:label  r:refresh  /:filter  ?:help  q:quit"

	var line1 string
	switch {
	case m.mode == modeConfirmKill:
		line1 = promptStyle.Render(fmt.Sprintf("Kill PID %d (%s)? (y/n)", m.killTarget.PID, m.killTarget.Process))
	case m.scanErr != nil:
		line1 = errorStyle.Render("scan failed: " + m.scanErr.Error())
	case m.status != "":
		line1 = statusStyle.Render(m.status)
	default:
		count := fmt.Sprintf("%d servers", len(m.visibleServers()))
		if m.filter != "" {
			count += fmt.Sprintf(" (filtered from %d)", len(m.servers))
		}
		line1 = statusStyle.Render(count + " · " + m.refreshedAgo())
	}

	return line1 + "\n" + statusStyle.Render(hints)
}

// refreshedAgo describes how long since the last successful scan.
func (m Model) refreshedAgo() string {
	if m.lastScan.IsZero() {
		return "scanning…"
	}
	d := time.Since(m.lastScan).Round(time.Second)
	if d < time.Second {
		return "refreshed just now"
	}
	return fmt.Sprintf("refreshed %s ago", d)
}

// truncate shortens s to at most n display columns, adding an ellipsis when cut.
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > n {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}
