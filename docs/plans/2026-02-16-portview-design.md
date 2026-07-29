# portview - Design Document

> A lightweight TUI for discovering and managing localhost dev servers.

**Date:** 2026-02-16
**Status:** Implemented through v0.3
**Last updated:** 2026-07-28
**Language:** Go + Bubble Tea
**Platforms:** macOS, Linux

---

## Problem

Developers routinely run multiple local servers (frontend, API, database, etc.) and have no simple way to see what's listening on localhost. The current workflow is `lsof -i -P | grep LISTEN` or remembering port numbers. There's no dedicated, polished tool for this.

## Solution

A single-binary TUI and scriptable CLI that auto-discovers TCP servers listening
on localhost, shows the process or container that owns each port, and lets you
open, inspect, kill, label, hide, and filter them.

## Non-Goals

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
│         App Logic Layer         │  State, label/visibility CRUD,
│  (model updates, commands)      │  action dispatch, CLI commands
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
│   │   ├── detail.go            # On-demand process + HTTP insight
│   │   ├── scanner_darwin.go    # macOS: lsof-based implementation
│   │   ├── scanner_linux.go     # Linux: /proc/net/tcp{,6} implementation
│   │   └── scanner_test.go      # Unit tests with mock data
│   ├── action/                  # Shared browser/termination actions
│   ├── cli/                     # list/kill/open/hide/unhide/hidden
│   ├── docker/                  # Container discovery and safe stop
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
    Hidden  bool   // Config hides the port from the default view
    Healthy bool   // True if port responds to TCP connect
    Container string // Docker/OrbStack container name, when applicable
    Image     string // Container image
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
   - **Linux:** Read `/proc/net/tcp` and `/proc/net/tcp6`, filter for `LISTEN` state, extract local port + inode, map inode to PID via `/proc/{pid}/fd`.

2. **Resolve process info**
   - **macOS:** `ps -p {pid} -o comm=,args=` for process name and full command.
   - **Linux:** Read `/proc/{pid}/comm` and `/proc/{pid}/cmdline`.

3. **Health check**
   - TCP dial to `localhost:{port}` with 200ms timeout. Go's dual-stack dialer
     supports IPv4-only and IPv6-only dev servers.

4. **Enrich and decorate**
   - Resolve Docker/OrbStack proxies to their container name and image.
   - Match discovered ports against saved labels and hidden-port preferences.

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
│  o:open  x:kill  h:hide  a:hidden  /:filter  ?:help  │
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
| `i` | Open the on-demand insight pane |
| `x` | Kill process/container (with y/n confirmation) |
| `l` | Set/edit label for selected port |
| `h` | Hide selected port, or unhide a revealed port |
| `a` | Toggle configured hidden ports |
| `r` | Force immediate refresh |
| `/` | Filter/search by port, process, or label |
| `q` or `Ctrl+C` | Quit |
| `?` | Toggle help overlay |

### Interaction Details

- **Kill confirmation:** Inline in the status bar: "Kill PID 1234? (y/n)". Not a modal.
- **Label editing:** Inline text input replacing the label cell. Enter to save, Esc to cancel.
- **Visibility:** Hiding persists immediately. Hidden listeners remain in the
  model so `a` can reveal them without another platform scan; revealed rows
  carry an explicit `[hidden]` marker (`[h]` when labeled), and `h` restores
  them.
- **Filter:** Live-filter input at the top that narrows the list as you type.
  Matches port, process, label, container name, and image.
- **Insight:** On demand only: cwd, uptime, CPU/memory/RSS, and an IPv4/IPv6
  localhost HTTP probe. It never adds work to the poll loop.

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
- **Hidden ports:** Filtered out of the default display. `h`/`a` manage them in
  the TUI; `hide`, `unhide`, and `hidden [--json]` provide the same workflow
  without the TUI.
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
- Cover: navigation, kill confirmation, label editing, hide/unhide,
  show-hidden, stale-scan config races, filtering, and the insight pane.

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
- Updates the Homebrew tap cask

### Build Targets

| OS | Architecture |
|----|-------------|
| darwin | arm64, amd64 |
| linux | amd64, arm64 |

---

## Distribution

- **`go install`:** `go install github.com/jeramiahgcoffey/portview/cmd/portview@latest`
- **Homebrew:** `brew install jeramiahgcoffey/tap/portview` (auto-generated by GoReleaser)
- **GitHub Releases:** Pre-built binaries attached to tagged releases
- **AUR / deb / rpm:** Not in v0.1, easy to add later via GoReleaser config

---

## Project Files

| File | Purpose |
|------|---------|
| `LICENSE` | MIT |
| `README.md` | Demo GIF, install instructions, usage, keybindings |
| `CHANGELOG.md` | Release-slice notes and shipped user-facing changes |
| `CONTRIBUTING.md` | Short guide, link to issues |
| `.goreleaser.yaml` | Release automation config |
| `.golangci.yaml` | Linter configuration |
| `Makefile` | `build`, `test`, `lint`, `run` targets |

---

## Future Considerations

- Windows support via `netstat` backend
- Log tailing for selected processes
- ~~Docker container discovery alongside native processes~~ — shipped in v0.2
- Configurable color themes
- Mouse support

> **v0.2 (2026-07-02):** shipped an insight pane (per-process cwd, uptime,
> cpu/mem, on-demand HTTP probe), Docker/OrbStack container discovery
> (container name in the list, `docker stop` in place of a proxy SIGTERM), and
> non-TUI CLI subcommands (`list`/`kill`/`open`, with `list --json`). The
> original layered architecture held: discovery stayed behind the scanner
> boundary, and a new `internal/docker` package sits beside it for enrichment.

> **v0.2.1 / v0.3 (2026-07-28 implementation):** v0.2.1 corrects health and
> HTTP insight for IPv6-only localhost servers. v0.3 adds persistent
> hide/unhide and show-hidden workflows across the TUI and CLI, explicit hidden
> state in table/JSON output, receipt-time config decoration so in-flight scans
> cannot revert newer edits, and revision-ordered config writes so asynchronous
> saves cannot persist stale label or visibility state.
