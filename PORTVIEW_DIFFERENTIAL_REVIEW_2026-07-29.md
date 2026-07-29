# Portview Differential Review - 2026-07-29

## Executive Summary

| Severity | Open findings |
|----------|---------------|
| 🔴 CRITICAL | 0 |
| 🟠 HIGH | 0 |
| 🟡 MEDIUM | 0 |
| 🟢 LOW | 0 |

**Overall Risk:** LOW
**Recommendation:** APPROVE

**Key Metrics:**

- Files analyzed: 19/19 pre-report changed files (100%)
- Production Go files analyzed: 8/8 (100%)
- Changed-function test gaps: 0 blocking gaps
- High-blast-radius changes: 0
- Security regressions detected: 0
- Resolved during review: 1 asynchronous persistence-ordering defect
- Final statement coverage: 71.6% repository-wide

The change preserves portview's core safety boundaries: discovery and probes
remain localhost-only, port arguments remain range-validated, hiding never
terminates or opens a process, Docker proxy termination protections are
unchanged, and the default CLI table/JSON shapes remain backward-compatible.

## What Changed

**Baseline:** `79d78e2` (`v0.2.0`, `origin/main`)
**Reviewed state:** working tree based on `79d78e2`
**Timeline:** 2026-07-28 to 2026-07-29

| Area | Files | Risk | Blast radius |
|------|-------|------|--------------|
| IPv4/IPv6 health and HTTP probing | `internal/scanner/scanner.go`, `internal/scanner/detail.go` | MEDIUM | LOW (two platform scanners and one insight command) |
| Hidden-state decoration | `internal/config/config.go`, `internal/scanner/scanner.go` | MEDIUM | LOW (CLI and TUI decoration paths) |
| CLI visibility commands/output | `internal/cli/cli.go` | MEDIUM | LOW (single command dispatcher) |
| TUI visibility and persistence | `internal/tui/model.go`, `commands.go`, `keys.go`, `view.go` | MEDIUM | LOW (single Bubble Tea model) |
| Tests | six `*_test.go` files | LOW | Test-only |
| Documentation/demo | README, changelog, design doc, VHS source/GIF | LOW | User-facing only |

Pre-report totals were 795 additions and 91 deletions across 19 files,
including the new changelog and regenerated binary GIF.

### Behavioral changes

- Health and HTTP insight resolve IPv4- and IPv6-only localhost listeners.
- Decorated servers carry explicit hidden state while preserving omission from
  default JSON when false.
- The TUI keeps hidden listeners in memory, reveals them with `a`, and toggles
  visibility with `h`.
- The CLI adds `hide`, `unhide`, and `hidden [--json]`; only `list --all`
  receives the additional `HIDDEN` table column.
- Scan results are decorated with current config on receipt, preventing an
  older in-flight scan from reverting a newer UI edit.
- Config writes are revision-ordered and serialized so asynchronous Tea
  commands cannot persist stale state.

## Findings

No open critical, high, medium, or low findings remain.

### Resolved during review: stale asynchronous config write

**Files:** `internal/tui/commands.go`, `internal/tui/model.go`
**Original severity:** MEDIUM
**Blast radius:** LOW (label and hide/unhide persistence)
**Test coverage:** YES

Tea commands execute asynchronously. The original implementation captured a
config value for each edit and wrote it independently. A rapid hide followed
by unhide could execute the newer write first and then allow the older write
to overwrite disk, leaving the UI and persisted config inconsistent.

The fix introduces a shared `configSaver` that:

1. deep-copies maps and slices at schedule time;
2. assigns each save a monotonically increasing revision;
3. serializes writes under a mutex; and
4. discards commands superseded before execution.

Deterministic tests execute newest-before-oldest and mutate the source config
after scheduling to prove both ordering and snapshot isolation.

## Test Coverage Analysis

| Changed function | Statement coverage |
|------------------|-------------------:|
| `dialHealthy` | 100.0% |
| `ProbeHTTP` | 88.9% |
| `Config.Decorate` | 100.0% |
| `writeServerTable` | 93.3% |
| `runVisibility` | 77.3% |
| `runHidden` | 79.4% |
| `configSaver.command` | 100.0% |
| `cloneConfig` | 100.0% |
| `handleNormal` | 94.2% |
| `visibilityServers` | 100.0% |
| `visibleServers` | 100.0% |
| `hiddenCount` | 100.0% |
| `applyConfig` | 100.0% |
| `renderRow` | 92.3% |
| `renderStatus` | 87.5% |

Coverage includes IPv4-only and IPv6-only listeners, closed ports, redirects,
hide/unhide persistence, idempotent CLI behavior, JSON output, default table
compatibility, hidden-row rendering, empty-state behavior, stale scan
decoration, stale save ordering, and immutable save snapshots.

## Blast Radius Analysis

All changed production functions have low blast radius:

- `dialHealthy`: two platform scanner callers
- `ProbeHTTP`: one TUI insight caller
- `Config.Decorate`: CLI plus TUI merge paths
- `runVisibility` / `runHidden`: one CLI dispatcher each
- `configSaver.command`: label and hide/unhide edit paths
- visibility/render helpers: internal to one Bubble Tea model

No authentication, authorization, process-signal validation, Docker stop
logic, release credentials, or remote network discovery code changed.

## Historical Context

- The fixed IPv4 literal originated in the v0.1 scanner implementation and was
  not introduced as a security control.
- Hidden-port primitives (`Hide`, `Unhide`, `IsHidden`) originated in v0.1 but
  previously lacked user-facing TUI/CLI management.
- `Config.Decorate` was added in v0.2 to keep CLI/TUI filtering consistent; the
  new explicit `Hidden` field extends that invariant rather than bypassing it.
- Git history searches found no removed validation from security-, CVE-, or
  audit-motivated commits and no reintroduction of previously removed unsafe
  patterns.

## Adversarial Checks

| Scenario | Result |
|----------|--------|
| Invalid/negative/out-of-range port passed to hide/unhide | Rejected by shared `portArg`; tested |
| Hide action accidentally kills or opens a listener | No action/docker call is reachable from visibility path |
| Hidden listener becomes indistinguishable when revealed | Explicit `[hidden]` or `[h]` marker; visually and automatically tested |
| Default script output breaks after upgrade | Default table unchanged; false `hidden` JSON omitted |
| Old scan result reverts a recent hide/label | Config applied at result receipt; regression test passes |
| Old async save overwrites a recent edit | Revisioned serialized writer; deterministic regression test passes |
| IPv6-only server is misreported down | Unit and real Vite smoke tests pass |

## Recommendations

### Immediate

- None. The reviewed diff is approved for merge and release.

### Release verification

- Require green Linux and macOS CI before merge.
- Build all four GoReleaser targets.
- Verify both tag-triggered release workflows, checksums, binaries, and the
  Homebrew tap update.

### Technical debt

- Concurrent config changes from separate OS processes remain last-writer-wins,
  matching the pre-existing config model. Consider file locking only if real
  concurrent CLI/TUI usage becomes common.

## Analysis Methodology

**Strategy:** FOCUSED (33 Go files; medium codebase)

**Analysis scope:**

- 100% of changed production, test, and documentation files
- Baseline and changed implementations for every production diff
- Git blame/history for removed literals and filtering behavior
- One-hop caller counts and state-flow tracing
- Validation-removal and secret-pattern scans
- Micro-adversarial analysis of CLI inputs, persistence ordering, local
  networking, output compatibility, and TUI state transitions

**Validation evidence:**

- `go test -count=1 ./...`
- `go test -count=1 -tags integration ./...`
- `go test -race ./...`
- `go vet ./...`
- `golangci-lint`: 0 issues
- macOS/Linux amd64/arm64 cross-builds
- isolated CLI hide/list/unhide lifecycle
- real IPv6-only Vite listener smoke test
- regenerated GIF contact-sheet inspection

**Limitations:**

- CodeRabbit CLI review was unavailable at report time because the local CLI
  was signed out.
- GitHub-hosted CI and release workflow results are pending publication.

**Confidence:** HIGH for the reviewed local diff.
