# Contributing to portview

Thanks for your interest in improving portview! This is a small, focused tool —
see the [design doc](docs/plans/2026-02-16-portview-design.md) and its
"Non-Goals" section before proposing large features.

## Getting started

```sh
git clone https://github.com/jeramiahgcoffey/portview
cd portview
make build      # build into bin/
make run        # run from source
```

## Before opening a PR

Run the full loop locally — CI runs the same checks on Linux and macOS:

```sh
make test               # unit tests
make test-integration   # tests that touch the real lsof / /proc backend
make lint               # golangci-lint (v2)
```

- Keep the layer separation intact: the TUI must not depend on a platform — all
  OS-specific discovery lives behind the `scanner.Scanner` interface.
- Add tests alongside changes (pure-function tests for parsers, `teatest` for
  TUI flows).
- Use [Conventional Commits](https://www.conventionalcommits.org/) for commit
  messages.

## Reporting issues

Open an issue with your OS, `portview --version`, and steps to reproduce.
