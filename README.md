# portview

A single-binary TUI that auto-discovers TCP servers listening on localhost, shows process ownership, and lets you open, kill, or label them.

## Install

```bash
go install github.com/jeramiahgcoffey/portview/cmd/portview@latest
```

## Usage

```bash
portview
```

## Keybindings

| Key | Action |
|---|---|
| `j`/`k` or `↑`/`↓` | Navigate |
| `o` or `Enter` | Open in browser |
| `x` | Kill process |
| `l` | Edit label |
| `r` | Refresh |
| `/` | Filter |
| `?` | Help |
| `q` | Quit |

## Configuration

Config file location: `$XDG_CONFIG_HOME/portview/config.yaml` (default: `~/.config/portview/config.yaml`)

```yaml
refresh_interval: 3s
port_range:
  min: 1024
  max: 65535
labels:
  8080: "web-api"
  3000: "frontend"
hidden:
  - 22
  - 443
```
