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
	dockerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("5")) // magenta
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

	if m.mode == modeDetail {
		b.WriteString(m.renderDetail(inner))
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

	// Docker-published ports show the container instead of the opaque proxy
	// process (com.docker.backend / docker-proxy), which is what the user
	// actually cares about.
	procText, cmdText := s.Process, s.Command
	isDocker := s.Container != ""
	if isDocker {
		procText = s.Container
		cmdText = "docker: " + s.Image
	}

	process := fmt.Sprintf("%-*s", colProcess+colGap, truncate(procText, colProcess))
	if isDocker {
		process = dockerStyle.Render(process)
	}
	command := fmt.Sprintf("%-*s", cmdW+colGap, truncate(cmdText, cmdW))

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
	hints := "↑↓/jk:nav  o:open  i:inspect  x:kill  l:label  r:refresh  /:filter  ?:help  q:quit"

	var line1 string
	switch {
	case m.mode == modeConfirmKill && m.killTarget.Container != "":
		line1 = promptStyle.Render(fmt.Sprintf("Stop container %s (:%d)? (y/n)", m.killTarget.Container, m.killTarget.Port))
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

// renderDetail renders the insight pane for the selected server. It replaces
// the list area (like the help overlay) and carries its own hint line.
func (m Model) renderDetail(inner int) string {
	s := m.detailTarget

	title := fmt.Sprintf(":%d  %s (pid %d)", s.Port, s.Process, s.PID)
	if s.Label != "" {
		title += "  · " + labelStyle.Render(s.Label)
	}

	var b strings.Builder
	b.WriteString(cursorStyle.Render(title))
	b.WriteByte('\n')

	// kv truncates, so values must be plain text (truncate slices runes and
	// would cut ANSI escape codes in half); kvStyled skips truncation for
	// short pre-styled values.
	kv := func(k, v string) {
		if v == "" {
			return
		}
		b.WriteString(headerStyle.Render(fmt.Sprintf("  %-9s", k)))
		b.WriteString(truncate(v, inner-12))
		b.WriteByte('\n')
	}
	kvStyled := func(k, v string) {
		if v == "" {
			return
		}
		b.WriteString(headerStyle.Render(fmt.Sprintf("  %-9s", k)))
		b.WriteString(v)
		b.WriteByte('\n')
	}

	if s.Container != "" {
		kv("container", s.Container+"  ("+s.Image+")")
	}
	kv("command", s.Command)

	switch {
	case m.detailLoading:
		b.WriteString(statusStyle.Render("  inspecting…"))
		b.WriteByte('\n')
	case m.detailErr != nil:
		b.WriteString(errorStyle.Render("  inspect failed: " + m.detailErr.Error()))
		b.WriteByte('\n')
	default:
		kv("cwd", m.detail.CWD)
		kv("uptime", formatUptime(m.detail.Uptime))
		kv("cpu/mem", fmt.Sprintf("%.1f%% cpu · %.1f%% mem · %s rss",
			m.detail.CPUPercent, m.detail.MemPercent, formatRSS(m.detail.RSSKB)))
	}

	if !m.detailLoading {
		kvStyled("http", formatProbe(m.detailProbe))
	}

	b.WriteByte('\n')
	b.WriteString(strings.Repeat("─", inner))
	b.WriteByte('\n')
	b.WriteString(statusStyle.Render("esc/i:back  o:open  r:re-inspect  q:quit"))
	return b.String()
}

// formatUptime renders a process age compactly: "42s", "3m10s", "2h03m", "5d2h".
func formatUptime(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%dd%dh", int(d.Hours())/24, int(d.Hours())%24)
	}
}

// formatRSS renders a kilobyte count as a human-readable size.
func formatRSS(kb int64) string {
	switch {
	case kb >= 1<<20:
		return fmt.Sprintf("%.1f GB", float64(kb)/(1<<20))
	case kb >= 1<<10:
		return fmt.Sprintf("%.0f MB", float64(kb)/(1<<10))
	default:
		return fmt.Sprintf("%d KB", kb)
	}
}

// formatProbe summarizes the HTTP probe for the insight pane.
func formatProbe(p scanner.HTTPProbe) string {
	if !p.OK {
		if p.Err == "" {
			return ""
		}
		return deadStyle.Render("no HTTP response") + statusStyle.Render(" ("+truncate(p.Err, 48)+")")
	}
	out := fmt.Sprintf("%d · %s", p.Status, p.Latency.Round(time.Millisecond))
	if p.Server != "" {
		out += " · " + p.Server
	}
	if p.Status >= 200 && p.Status < 400 {
		return healthyStyle.Render(out)
	}
	return deadStyle.Render(out)
}

// truncate shortens s to at most n display columns, adding an ellipsis when cut.
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= n {
		return s
	}
	const ellipsis = "…"
	ew := lipgloss.Width(ellipsis)
	if n < ew {
		return "" // not even room for the ellipsis
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+ew > n {
		r = r[:len(r)-1]
	}
	return string(r) + ellipsis
}
