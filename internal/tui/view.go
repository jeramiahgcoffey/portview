package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("240"))

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205"))

	healthyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	unhealthyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	filterBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205"))

	helpOverlayStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				Padding(1, 2).
				BorderForeground(lipgloss.Color("205"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))
)

// View implements tea.Model.
func (m Model) View() string {
	if m.mode == modeHelp {
		return m.viewHelp()
	}

	var b strings.Builder

	// Header
	b.WriteString(titleStyle.Render("portview"))
	b.WriteString("\n\n")

	// Filter bar
	if m.mode == modeFilter {
		b.WriteString(filterBarStyle.Render("Filter: "))
		b.WriteString(m.filterText)
		b.WriteString("_\n\n")
	} else if m.filterText != "" {
		b.WriteString(filterBarStyle.Render(fmt.Sprintf("Filter: %s", m.filterText)))
		b.WriteString("\n\n")
	}

	// Label input bar
	if m.mode == modeLabel {
		b.WriteString("Label: ")
		b.WriteString(m.labelInput.View())
		b.WriteString("\n\n")
	}

	// Column headers
	b.WriteString(headerStyle.Render(fmt.Sprintf("  %-7s %-7s %-15s %-15s %s", "PORT", "PID", "PROCESS", "LABEL", "COMMAND")))
	b.WriteString("\n")

	// Server list
	if len(m.filtered) == 0 {
		b.WriteString("\n  No servers found.\n")
	} else {
		for i, s := range m.filtered {
			cursor := "  "
			if i == m.cursor {
				cursor = cursorStyle.Render("> ")
			}

			portStr := fmt.Sprintf("%-7d", s.Port)
			pidStr := fmt.Sprintf("%-7d", s.PID)

			var style lipgloss.Style
			if s.Healthy {
				style = healthyStyle
			} else {
				style = unhealthyStyle
			}

			process := truncate(s.Process, 15)
			label := truncate(s.Label, 15)
			command := truncate(s.Command, 40)

			row := fmt.Sprintf("%s%s %s %-15s %s %s",
				cursor,
				style.Render(portStr),
				style.Render(pidStr),
				style.Render(process),
				labelStyle.Render(fmt.Sprintf("%-15s", label)),
				style.Render(command),
			)
			b.WriteString(row)
			b.WriteString("\n")
		}
	}

	// Status bar
	b.WriteString("\n")
	b.WriteString(m.statusBar())

	return b.String()
}

// statusBar renders the bottom status bar.
func (m Model) statusBar() string {
	if m.mode == modeConfirmKill && len(m.filtered) > 0 {
		s := m.filtered[m.cursor]
		return statusBarStyle.Render(fmt.Sprintf("Kill PID %d? (y/n)", s.PID))
	}

	var parts []string

	// Server count
	parts = append(parts, fmt.Sprintf("%d servers", len(m.filtered)))

	// Last refresh
	if !m.lastRefresh.IsZero() {
		ago := time.Since(m.lastRefresh).Truncate(time.Second)
		parts = append(parts, fmt.Sprintf("refreshed %s ago", ago))
	}

	// Error
	if m.err != nil {
		parts = append(parts, fmt.Sprintf("error: %v", m.err))
	}

	status := statusBarStyle.Render(strings.Join(parts, " | "))

	hints := statusBarStyle.Render("j/k:nav  o:open  x:kill  l:label  /:filter  ?:help  q:quit")

	return status + "\n" + hints
}

// viewHelp renders the help overlay.
func (m Model) viewHelp() string {
	help := strings.Builder{}
	help.WriteString("Help\n\n")
	help.WriteString("  j/k, ↑/↓   Navigate\n")
	help.WriteString("  o, enter   Open in browser\n")
	help.WriteString("  x          Kill process\n")
	help.WriteString("  l          Edit label\n")
	help.WriteString("  r          Refresh\n")
	help.WriteString("  /          Filter\n")
	help.WriteString("  ?          Toggle help\n")
	help.WriteString("  q          Quit\n")
	help.WriteString("\nPress any key to close")

	return helpOverlayStyle.Render(help.String())
}

// truncate shortens a string to maxLen runes, adding "…" if truncated.
func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	if maxLen == 1 {
		return string(r[:1])
	}
	return string(r[:maxLen-1]) + "…"
}
