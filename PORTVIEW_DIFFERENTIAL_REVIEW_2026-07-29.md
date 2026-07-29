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

- Files analyzed: 22/22 final changed files (100%)
- Production Go files analyzed: 8/8 (100%)
- Changed-function test gaps: 0 blocking gaps
- High-blast-radius changes: 0
- Security regressions detected: 0
- Resolved during review: 2 persistence defects and 2 documentation findings
- Final statement coverage: 72.4% repository-wide

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
| Hidden-state decoration and persistence | `internal/config/config.go`, `internal/scanner/scanner.go` | MEDIUM | LOW (CLI and TUI decoration paths) |
| CLI visibility commands/output | `internal/cli/cli.go` | MEDIUM | LOW (single command dispatcher) |
| TUI visibility and persistence | `internal/tui/model.go`, `commands.go`, `keys.go`, `view.go` | MEDIUM | LOW (single Bubble Tea model) |
| Tests | seven `*_test.go` files | LOW | Test-only |
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
- Config edits use locked, atomic read-modify-write transactions. TUI edits are
  queued and revision-ordered so asynchronous or cross-process changes cannot
  discard one another.

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

The final fix introduces a shared `configSaver` that queues immutable
preference-level edits, assigns monotonic revisions, and lets whichever Tea
command executes first drain all scheduled edits in order. Model revisions
prevent an older save result from replacing a newer local edit.

Deterministic tests execute newest-before-oldest and prove that the complete
edit queue is persisted once without a stale result changing model state.

### Resolved during remote review: cross-process config lost update

**Files:** `internal/config/config.go`, `internal/cli/cli.go`,
`internal/tui/commands.go`, `internal/tui/model.go`
**Original severity:** HIGH
**Blast radius:** LOW (label and hide/unhide persistence)
**Test coverage:** YES

CodeRabbit correctly identified that serialized Tea commands alone did not
coordinate separate CLI/TUI processes. A stale full-config snapshot could
silently replace an independent label or visibility edit.

The follow-up adds:

1. a persistent sibling advisory lock shared by all portview config writers;
2. locked reload-mutate-save transactions for preference-level updates;
3. atomic temporary-file replacement so readers never see partial YAML;
4. explicit TUI edit operations that merge into the latest on-disk config; and
5. model adoption of external changes returned by the locked transaction.

Concurrent-update, external-change, no-op, revision-order, and stale-result
tests cover the resulting data-integrity boundary.

### Resolved during remote review: documentation accuracy and lint

The design document now separates scanner enrichment from receipt-time config
decoration and removes the MD028 blank line inside its release-note blockquote.

## Test Coverage Analysis

| Changed function | Statement coverage |
|------------------|-------------------:|
| `dialHealthy` | 100.0% |
| `ProbeHTTP` | 88.9% |
| `Config.Decorate` | 100.0% |
| `Config.Save` | 75.0% |
| `Config.Update` | 75.0% |
| `Config.UpdateFrom` | 84.6% |
| `withFileLock` | 75.0% |
| `saveUnlocked` | 56.5% (success path plus error cleanup) |
| `writeServerTable` | 93.3% |
| `runVisibility` | 88.0% |
| `runHidden` | 79.4% |
| `configEdit.apply` | 100.0% |
| `configSaver.command` | 95.2% |
| `handleNormal` | 94.6% |
| `handleLabel` | 94.4% |
| `visibilityServers` | 100.0% |
| `visibleServers` | 100.0% |
| `hiddenCount` | 100.0% |
| `applyConfig` | 100.0% |
| `renderRow` | 92.3% |
| `renderStatus` | 87.5% |

Coverage includes IPv4-only and IPv6-only listeners, closed ports, redirects,
hide/unhide persistence, idempotent CLI behavior, JSON output, default table
compatibility, hidden-row rendering, empty-state behavior, stale scan
decoration, stale save ordering, cross-process update merging, atomic
persistence, and stale save-result rejection.

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
| Old async save overwrites a recent edit | Revisioned edit queue; deterministic regression test passes |
| Concurrent CLI/TUI writes discard independent config changes | Locked read-modify-write mutation; concurrent regression test passes |
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

- The lock is advisory: portview CLI/TUI processes coordinate fully, while a
  third-party editor that deliberately ignores the sibling lock remains
  outside that guarantee.

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
- `markdownlint-cli2` 0.23.1: 0 issues across all changed Markdown
- macOS/Linux amd64/arm64 cross-builds
- isolated CLI hide/list/unhide lifecycle
- ten concurrent CLI processes preserving every independent hidden port
- real IPv6-only Vite listener smoke test
- regenerated GIF contact-sheet inspection

**Remote review:**

- CodeRabbit's GitHub review completed against the initial PR head and its
  three actionable comments were independently verified and fixed.
- GitHub CI passed on Linux, macOS, and lint before the review follow-up; the
  follow-up commit requires the same green gate before merge.
- Tag-triggered release workflow results remain pending release publication.

**Confidence:** HIGH for the reviewed local diff.
