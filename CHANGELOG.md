# Changelog

All notable user-facing changes to portview are documented here.

## Unreleased

## v0.3.0 - 2026-07-29

### Added

- Hide or unhide the selected listener with `h`.
- Reveal configured hidden listeners with `a`; revealed rows carry an explicit
  `[hidden]` marker, compacted to `[h]` when they also have a label.
- Manage visibility without the TUI using `portview hide <port>`,
  `portview unhide <port>`, and `portview hidden [--json]`.
- Mark hidden listeners in `portview list --all` table and JSON output.

### Changed

- Keep hidden listeners in the TUI model, allowing instant reveal and restore
  without another platform scan.
- Apply labels and hidden state when scan results arrive so an older in-flight
  scan cannot revert a newer config edit.
- Persist config with locked, revision-ordered read-modify-write transactions
  so rapid edits and concurrent CLI/TUI processes cannot discard one another's
  label or visibility changes.

## v0.2.1 - 2026-07-29

### Fixed

- Detect IPv6-only localhost listeners as healthy.
- Probe IPv6-only HTTP servers from the insight pane.

## v0.2.0 - 2026-07-02

### Added

- On-demand insight pane with project cwd, process uptime, CPU/memory usage,
  RSS, and HTTP response details.
- Docker Desktop, OrbStack, and `docker-proxy` container discovery.
- Safe `docker stop` behavior for container-published ports.
- Scriptable `list`, `kill`, and `open` CLI commands, including JSON output.

## v0.1.0 - 2026-06-09

### Added

- Initial macOS/Linux localhost listener discovery TUI.
- Health status, browser opening, graceful termination, labels, filtering,
  configurable port range, and XDG configuration.
