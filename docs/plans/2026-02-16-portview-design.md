# portview - Design Document

> A lightweight TUI for discovering and managing localhost dev servers.

**Date:** 2026-02-16
**Status:** Draft
**Language:** Go + Bubble Tea
**Platforms:** macOS, Linux

---

## Problem

Developers routinely run multiple local servers (frontend, API, database, etc.) and have no simple way to see what's listening on localhost. The current workflow is `lsof -i -P | grep LISTEN` or remembering port numbers. There's no dedicated, polished tool for this.

## Solution

A single-binary TUI that auto-discovers TCP servers listening on localhost, shows what process owns each port, and lets you open, kill, or label them.

## Non-Goals (v0.1)

- Windows support
- Log viewing or tailing
- Process starting/restarting (this is not a process manager)
- Monitoring, alerting, or history
- Remote server discovery

---

## Architecture

```
┌─────────────────────────────────┐
│           TUI Layer             │  Bubble Tea model + Lip Gloss styling
│  (view, keybindings, layout)    │
├─────────────────────────────────┤
│         App Logic Layer         │  State management, label CRUD,
│  (model updates, commands)      │  action dispatch
├─────────────────────────────────┤
│        Scanner Layer            │  Port discovery, process resolution
│  (platform-specific backends)   │
├─────────────────────────────────┤
│        Config Layer             │  ~/.config/portview/config.yaml
│  (labels, preferences)          │  XDG-compliant paths
└─────────────────────────────────┘
```

### Package Layout

```
portview/
├── cmd/
│   └── portview/
│       └── main.go              # CLI entrypoint, load config, start TUI
├── internal/
│   ├── scanner/
│   │   ├── scanner.go           # Scanner interface + Server type
│   │   ├── scanner_darwin.go    # macOS: lsof-based implementation
│   │   ├── scanner_linux.go     # Linux: /proc/net/tcp implementation
│   │   └── scanner_test.go      # Unit tests with mock data
│   ├── tui/
│   │   ├── model.go             # Bubble Tea model
│   │   ├── view.go              # Rendering logic
│   │   ├── keys.go              # Keybinding definitions
│   │   ├── commands.go          # Bubble Tea commands (scan, open, kill)
│   │   └── tui_test.go          # teatest-based tests
│   └── config/
│       ├── config.go            # Load/save config, XDG paths
│       └── config_test.go
├── docs/
│   └── plans/
│       └── 2026-02-16-portview-design.md
├── .github/
│   └── workflows/
│       ├── ci.yaml
│       └── release.yaml
├── .goreleaser.yaml
├── .golangci.yaml
├── Makefile
├── LICENSE
├── README.md
├── CONTRIBUTING.md
├── go.mod
└── go.sum
```

---

## Data Model

### Server

The core type returned by the scanner for each discovered server:

```go
type Server struct {
    Port    int    // TCP port number
    PID     int    // OS process ID
    Process string // Short process name (e.g., "node", "python3", "go")
    Command string // Full command line (e.g., "node server.js")
    State   string // TCP state, typically "LISTEN"
    Label   string // User-assigned label from config (e.g., "frontend")
    Healthy bool   // True if port responds to TCP connect
}
```

---

## Scanner

### Interface

```go
type Scanner interface {
    Scan(ctx context.Context) ([]Server, error)
}
```

Platform-specific implementations (`darwinScanner`, `linuxScanner`) satisfy this interface. The TUI never knows which platform it's on.

### Discovery Steps

1. **Discover listening ports**
   - **macOS:** Shell out to `lsof -iTCP -sTCP:LISTEN -nP`. Parse output for port + PID.
   - **Linux:** Read `/proc/net/tcp`, filter for `LISTEN` state, extract local port + inode, map inode to PID via `/proc/{pid}/fd`.

2. **Resolve process info**
   - **macOS:** `ps -p {pid} -o comm=,args=` for process name and full command.
   - **Linux:** Read `/proc/{pid}/comm` and `/proc/{pid}/cmdline`.

3. **Health check**
   - TCP dial to `localhost:{port}` with 200ms timeout. Binary healthy/unhealthy.

4. **Merge labels**
   - Match discovered ports against user's saved labels from config.

### Port Range

Default: `1024-65535`. Skips well-known ports below 1024 to avoid noise from system services. Configurable via config file.

### Poll Loop

The scanner runs on a ticker (default 3s). Each tick fires a Bubble Tea `Cmd` that calls `Scan()` and sends the result back to the model as a message. A manual refresh keybind triggers an immediate scan outside the ticker.

---

## TUI

### Layout

```
╭─ portview ──────────────────────────────────────────╮
│  PORT   PROCESS       COMMAND            LABEL      │
│ ► 3000  node          next dev           frontend   │
│   3001  node          express server.js  api        │
│   5432  postgres      postgres -D ...               │
│   8080  go            go run main.go     backend    │
│                                                     │
├─────────────────────────────────────────────────────┤
│  4 servers · refreshed 1s ago                       │
│  o:open  k:kill  l:label  r:refresh  /:filter  ?:help│
╰─────────────────────────────────────────────────────╯
```

Three zones:

1. **Header** - App name, column headings.
2. **Server list** - Scrollable with highlighted cursor row. Healthy ports in green, unresponsive in dim/yellow.
3. **Status bar** - Server count, time since last refresh, keybind hints.

### Keybindings

| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Navigate list |
| `o` or `Enter` | Open in default browser |
| `k` | Kill process (with y/n confirmation) |
| `l` | Set/edit label for selected port |
| `r` | Force immediate refresh |
| `/` | Filter/search by port, process, or label |
| `q` or `Ctrl+C` | Quit |
| `?` | Toggle help overlay |

### Interaction Details

- **Kill confirmation:** Inline in the status bar: "Kill PID 1234? (y/n)". Not a modal.
- **Label editing:** Inline text input replacing the label cell. Enter to save, Esc to cancel.
- **Filter:** Live-filter input at the top that narrows the list as you type. Matches against port number, process name, and label.

---

## Config

### Location

`~/.config/portview/config.yaml`

Respects `$XDG_CONFIG_HOME` if set. Falls back to `~/.config/` otherwise.

### Structure

```yaml
refresh_interval: 3s
port_range:
  min: 1024
  max: 65535

labels:
  3000: frontend
  3001: api
  8080: backend

hidden:
  - 5432
  - 6379
```

### Behavior

- **Labels:** Saved immediately on user action. Port-based (not PID-based) since dev servers reuse ports.
- **Hidden ports:** Filtered out of the display. Useful for noisy background services.
- **Preferences:** Refresh interval, port range.
- **No config required:** Tool works with sensible defaults if no config file exists.
- **Lazy creation:** Config file is only created on first user action that needs persistence (setting a label, hiding a port).

---

## Testing

### Scanner Layer

- Unit tests with a `mockScanner` returning canned `[]Server` data.
- Integration tests (build-tag gated) that call real `lsof`/`/proc` on CI runners.
- Pure function tests for parsing lsof output and /proc/net/tcp format.

### Config Layer

- Unit tests: write temp YAML files, load them, assert values.
- Test default behavior when no config file exists.
- Test config creation on first write.

### TUI Layer

- Bubble Tea's `teatest` package for programmatic testing.
- Send key messages, assert on rendered output.
- Cover: navigation, kill confirmation flow, label editing, filter input.

---

## CI/CD

### GitHub Actions

**ci.yaml** (runs on every PR):
- Lint with `golangci-lint`
- Run `go test ./...` on both `ubuntu-latest` and `macos-latest` runners
- Build check for both platforms

**release.yaml** (runs on version tags `v*`):
- GoReleaser builds cross-platform binaries
- Publishes GitHub release with binaries
- Updates Homebrew tap formula

### Build Targets

| OS | Architecture |
|----|-------------|
| darwin | arm64, amd64 |
| linux | amd64, arm64 |

---

## Distribution

- **`go install`:** `go install github.com/<owner>/portview@latest`
- **Homebrew:** `brew install <owner>/tap/portview` (auto-generated by GoReleaser)
- **GitHub Releases:** Pre-built binaries attached to tagged releases
- **AUR / deb / rpm:** Not in v0.1, easy to add later via GoReleaser config

---

## Project Files

| File | Purpose |
|------|---------|
| `LICENSE` | MIT |
| `README.md` | Demo GIF, install instructions, usage, keybindings |
| `CONTRIBUTING.md` | Short guide, link to issues |
| `.goreleaser.yaml` | Release automation config |
| `.golangci.yaml` | Linter configuration |
| `Makefile` | `build`, `test`, `lint`, `run` targets |

---

## Future Considerations (not v0.1)

- Windows support via `netstat` backend
- Log tailing for selected processes
- Docker container discovery alongside native processes
- Configurable color themes
- Mouse support
