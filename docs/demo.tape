# portview demo — generate the README GIF with:
#   vhs docs/demo.tape
#
# VHS (https://github.com/charmbracelet/vhs) replays this script in a headless
# terminal and renders docs/demo.gif. Edit + re-run to refresh the demo; this
# file is the source of truth so the GIF stays reproducible.
#
# The demo runs against an isolated config (XDG_CONFIG_HOME) with a tight port
# range, so only the three demo servers show — not the host's real processes.

Output docs/demo.gif

Set Shell "bash"
Set FontSize 16
Set Width 1100
Set Height 320
Set Padding 20
Set Theme "Dracula"
Set TypingSpeed 60ms

# --- setup (hidden): isolated config, build, start demo servers ---
Hide
Type "export XDG_CONFIG_HOME=/tmp/pv-demo-cfg" Enter
Type "rm -rf /tmp/pv-demo-cfg && mkdir -p /tmp/pv-demo-cfg/portview" Enter
Type "printf 'port_range:\n  min: 3000\n  max: 3002\n' > /tmp/pv-demo-cfg/portview/config.yaml" Enter
Type "go build -o /tmp/portview-demo ./cmd/portview" Enter
Sleep 3s
Type "python3 -m http.server 3000 >/dev/null 2>&1 &" Enter
Type "python3 -m http.server 3001 >/dev/null 2>&1 &" Enter
Type "python3 -m http.server 3002 >/dev/null 2>&1 &" Enter
Sleep 1s
Type "clear" Enter
Show

# --- launch ---
Type "/tmp/portview-demo" Enter
Sleep 2s

# label the first port
Type "l"
Sleep 500ms
Type "frontend"
Sleep 500ms
Enter
Sleep 1200ms

# navigate + label another
Down@600ms 1
Sleep 500ms
Type "l"
Sleep 400ms
Type "api"
Sleep 400ms
Enter
Sleep 1200ms

# declutter: hide the selected port, reveal hidden rows, then restore it
Type "h"
Sleep 1200ms
Type "a"
Sleep 1200ms
Type "h"
Sleep 1200ms

# insight pane: cwd, uptime, cpu/mem, live HTTP probe
Type "i"
Sleep 2500ms
Escape
Sleep 800ms

# live filter
Type "/"
Sleep 500ms
Type "3002"
Sleep 1500ms
Escape
Sleep 800ms

# help overlay
Type "?"
Sleep 1800ms
Type "?"
Sleep 700ms

# quit
Type "q"
Sleep 800ms

# CLI mode: same data, scriptable
Type "clear" Enter
Type "/tmp/portview-demo list"
Sleep 600ms
Enter
Sleep 2500ms

# --- cleanup (hidden) ---
Hide
Type "kill %1 %2 %3 2>/dev/null" Enter
Sleep 500ms
