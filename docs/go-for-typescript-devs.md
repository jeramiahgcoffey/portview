# Portview Codebase Guide: Go for TypeScript Developers

This document walks through the portview codebase, explaining Go concepts by
mapping them to TypeScript equivalents. It assumes strong TypeScript experience
and zero Go experience.

---

## Table of Contents

1. [Quick Reference: Go vs TypeScript](#1-quick-reference-go-vs-typescript)
2. [Project Structure](#2-project-structure)
3. [The Entry Point: main.go](#3-the-entry-point-maingo)
4. [Packages and Imports](#4-packages-and-imports)
5. [Types, Structs, and Methods](#5-types-structs-and-methods)
6. [Interfaces](#6-interfaces)
7. [Error Handling](#7-error-handling)
8. [Pointers and Value Receivers](#8-pointers-and-value-receivers)
9. [Slices and Maps](#9-slices-and-maps)
10. [Goroutines and Concurrency](#10-goroutines-and-concurrency)
11. [Build Constraints (Platform-Specific Code)](#11-build-constraints-platform-specific-code)
12. [The Bubble Tea Architecture](#12-the-bubble-tea-architecture)
13. [Walking Through Each Package](#13-walking-through-each-package)
14. [Testing in Go](#14-testing-in-go)
15. [Tooling and Build System](#15-tooling-and-build-system)

---

## 1. Quick Reference: Go vs TypeScript

| Concept                | TypeScript                         | Go                                       |
|------------------------|------------------------------------|------------------------------------------|
| Variable declaration   | `const x: number = 5`             | `x := 5` or `var x int = 5`             |
| Function               | `function add(a: number): number`  | `func add(a int) int`                    |
| Multiple returns       | return tuple/object                | `func f() (int, error)` (native)         |
| Null/undefined         | `null`, `undefined`                | `nil` (only for pointers, slices, maps, interfaces) |
| String interpolation   | `` `hello ${name}` ``              | `fmt.Sprintf("hello %s", name)`          |
| Class                  | `class Foo { ... }`                | `type Foo struct { ... }` + methods      |
| Interface              | `interface Foo { ... }`            | `type Foo interface { ... }` (implicit)  |
| Inheritance            | `extends`, `implements`            | Composition + interfaces (no inheritance)|
| Generics               | `<T>`                              | `[T any]` (since Go 1.18)               |
| Enums                  | `enum Mode { ... }`               | `const` + `iota`                         |
| Array                  | `number[]`                         | `[]int` (slices are dynamic)             |
| Object/map             | `Record<string, number>`           | `map[string]int`                         |
| Async/await            | `async/await`, `Promise`           | goroutines + channels                    |
| Module system          | `import/export`                    | packages (folder = package)              |
| Package manager        | npm/yarn/pnpm                      | `go mod` (built into toolchain)          |
| Visibility             | `export`/no export                 | Uppercase = public, lowercase = private  |

---

## 2. Project Structure

```
portview/
├── cmd/portview/main.go          # Entry point (like index.ts)
├── internal/                      # Private packages (can't be imported externally)
│   ├── config/
│   │   ├── config.go              # Config types + load/save logic
│   │   └── config_test.go         # Tests (co-located, always)
│   ├── scanner/
│   │   ├── scanner.go             # Interface + health checking
│   │   ├── scanner_darwin.go      # macOS implementation
│   │   ├── scanner_linux.go       # Linux implementation
│   │   ├── parse_lsof.go          # macOS output parsing
│   │   ├── parse_proc.go          # Linux /proc parsing
│   │   └── scanner_test.go        # Tests
│   └── tui/
│       ├── model.go               # State + update logic (like a React reducer)
│       ├── view.go                # Rendering (like a React component's render)
│       ├── commands.go            # Side effects (like Redux thunks)
│       ├── keys.go                # Keybinding definitions
│       └── tui_test.go            # Tests
├── go.mod                         # Like package.json
├── go.sum                         # Like package-lock.json
├── Makefile                       # Build scripts (like npm scripts)
└── .golangci.yaml                 # Linter config (like .eslintrc)
```

### Key Conventions

- **`cmd/`**: Executables go here. Each subfolder produces one binary.
- **`internal/`**: Private packages. Go enforces that code outside this module
  cannot import these. Think of it as truly private — not just a convention.
- **One package per directory**: Unlike TypeScript where a folder can have
  multiple modules, in Go a folder IS a package. All `.go` files in a folder
  share the same `package` declaration.
- **Tests live next to code**: `config_test.go` sits beside `config.go`.
  No `__tests__` directory needed.

---

## 3. The Entry Point: main.go

```go
// cmd/portview/main.go
package main                          // "main" package = executable

import (
    "fmt"
    "os"
    tea "github.com/charmbracelet/bubbletea"   // aliased import
    "github.com/jeramiahcoffey/portview/internal/config"
    "github.com/jeramiahcoffey/portview/internal/scanner"
    "github.com/jeramiahcoffey/portview/internal/tui"
)

func main() {                         // func main() is the entry point
    cfgPath := config.DefaultPath()   // := is "short variable declaration"
    cfg, err := config.Load(cfgPath)  // multiple return values!
    if err != nil {                   // Go's error handling pattern
        fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
        os.Exit(1)
    }

    s := scanner.New(cfg.PortRange)   // factory function returns interface
    m := tui.New(s, cfg, cfgPath)     // create the TUI model

    p := tea.NewProgram(m, tea.WithAltScreen())
    if _, err := p.Run(); err != nil {    // _ discards unused return value
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

### TypeScript Equivalent (Conceptual)

```typescript
// If this were TypeScript:
async function main() {
    const cfgPath = config.defaultPath();
    const cfg = await config.load(cfgPath);  // Go does this synchronously
    const s = new Scanner(cfg.portRange);
    const m = new TuiModel(s, cfg, cfgPath);
    const p = new BubbleTea.Program(m);
    await p.run();
}
main();
```

### Key Differences to Notice

- **No `async/await`**: Go is synchronous by default. Concurrency is explicit
  with goroutines.
- **`:=` vs `var`**: `:=` infers the type. `var x int` is explicit. You'll see
  `:=` used almost everywhere.
- **Multiple return values**: `cfg, err := config.Load(cfgPath)` returns two
  values. This is how Go handles errors — no exceptions.
- **`_` (blank identifier)**: Discards a value you don't need. Like `const [, err]`
  if destructuring in TS.

---

## 4. Packages and Imports

### Go's Module System

```go
// go.mod (like package.json)
module github.com/jeramiahcoffey/portview  // module path

go 1.25.6                                   // Go version

require (
    github.com/charmbracelet/bubbletea v1.3.10   // dependencies
    gopkg.in/yaml.v3 v3.0.1
)
```

```go
// Any .go file
package config                               // declares this file's package

import (
    "os"                                     // stdlib package
    "path/filepath"                          // stdlib, nested
    "gopkg.in/yaml.v3"                       // third party
    "github.com/jeramiahcoffey/portview/internal/scanner"  // local package
)
```

### Visibility: Uppercase = Public

This is the single most important Go convention to understand:

```go
// config.go
type Config struct {           // Config is exported (public) — uppercase C
    RefreshInterval time.Duration  // exported field
    Labels          map[int]string // exported field
}

type mode int                  // mode is unexported (private) — lowercase m

func Default() Config { ... }  // exported function
func (c *Config) save() { ... } // unexported method — only usable within this package
```

**TypeScript equivalent:**
```typescript
// Imagine if visibility was determined by casing:
export class Config { ... }      // uppercase = public
class mode { ... }               // lowercase = private to file
export function Default() { ... }
```

---

## 5. Types, Structs, and Methods

### Structs (Go's "Classes")

Go has no classes. Structs hold data, and methods are defined separately.

```go
// scanner/scanner.go
type Server struct {
    Port    int        // like { port: number }
    PID     int
    Process string
    Command string
    State   string
    Label   string
    Healthy bool
}
```

**TypeScript equivalent:**
```typescript
interface Server {
    port: number;
    pid: number;
    process: string;
    command: string;
    state: string;
    label: string;
    healthy: boolean;
}
```

### Methods

Methods are functions attached to a type:

```go
// config.go
func (c *Config) SetLabel(port int, label string) {
    if c.Labels == nil {
        c.Labels = make(map[int]string)
    }
    c.Labels[port] = label
}
```

The `(c *Config)` part is the **receiver** — it's like `this` in TypeScript:

```typescript
// TypeScript equivalent:
class Config {
    setLabel(port: number, label: string): void {
        this.labels[port] = label;
    }
}
```

### Struct Tags

Struct fields can have metadata tags:

```go
type Config struct {
    RefreshInterval time.Duration  `yaml:"refresh_interval"`
    PortRange       PortRange      `yaml:"port_range"`
}
```

This tells the YAML library what field name to use when serializing. Similar
to decorators or class-transformer in TypeScript:

```typescript
// Conceptual TS equivalent:
class Config {
    @JsonProperty("refresh_interval")
    refreshInterval: Duration;
}
```

### Enums with iota

Go doesn't have enums. Instead, it uses `const` blocks with `iota`:

```go
type mode int
const (
    modeNormal mode = iota   // 0
    modeFilter               // 1 (auto-increments)
    modeLabel                // 2
    modeConfirmKill          // 3
    modeHelp                 // 4
)
```

**TypeScript equivalent:**
```typescript
enum Mode {
    Normal = 0,
    Filter = 1,
    Label = 2,
    ConfirmKill = 3,
    Help = 4,
}
```

---

## 6. Interfaces

Go interfaces are **implicit** — the most powerful difference from TypeScript.

```go
// scanner.go
type Scanner interface {
    Scan(ctx context.Context) ([]Server, error)
}
```

Any type with a `Scan` method matching this signature **automatically**
implements the interface. No `implements` keyword needed:

```go
// scanner_darwin.go
type darwinScanner struct {
    portRange config.PortRange
}

func (d *darwinScanner) Scan(ctx context.Context) ([]Server, error) {
    // ... macOS implementation
}
// darwinScanner now satisfies Scanner automatically!
```

```go
// scanner_linux.go
type linuxScanner struct {
    portRange config.PortRange
}

func (l *linuxScanner) Scan(ctx context.Context) ([]Server, error) {
    // ... Linux implementation
}
// linuxScanner also satisfies Scanner automatically!
```

**TypeScript equivalent:**
```typescript
interface Scanner {
    scan(ctx: Context): Promise<[Server[], Error]>;
}

// In TS you'd write: class DarwinScanner implements Scanner { ... }
// In Go, it's implicit — just match the method signatures.
```

### Factory Functions (Instead of Constructors)

Go has no constructors. Instead, you write factory functions:

```go
func New(portRange config.PortRange) Scanner {
    return &darwinScanner{portRange: portRange}
}
```

This returns a `Scanner` interface. The caller never knows or cares whether
it got a `darwinScanner` or `linuxScanner`.

```typescript
// TypeScript equivalent:
function createScanner(portRange: PortRange): Scanner {
    return new DarwinScanner(portRange);
}
```

### Mock for Testing

```go
type MockScanner struct {
    Servers []Server
    Err     error
}

func (m *MockScanner) Scan(_ context.Context) ([]Server, error) {
    return m.Servers, m.Err
}
```

Because interfaces are implicit, `MockScanner` satisfies `Scanner` without
any declaration. In TypeScript you'd need `class MockScanner implements Scanner`.

---

## 7. Error Handling

Go doesn't have exceptions or try/catch. Errors are values returned alongside
results.

```go
cfg, err := config.Load(cfgPath)
if err != nil {
    // handle error
    return
}
// use cfg safely here
```

**TypeScript equivalent:**
```typescript
// Go's pattern is similar to this (but enforced by convention):
const [cfg, err] = config.load(cfgPath);
if (err !== null) {
    // handle error
    return;
}
```

### The `errors.Is` Pattern

```go
// config.go
func Load(path string) (Config, error) {
    cfg := Default()
    data, err := os.ReadFile(path)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {   // like instanceof check
            return cfg, nil                     // file missing = use defaults
        }
        return Config{}, err                    // real error = propagate
    }
    // ...
}
```

### Why No Exceptions?

Go's philosophy: errors are expected, normal control flow. The `if err != nil`
pattern is verbose but explicit — you always know what can fail and what
happens when it does. Compare to TypeScript where any function call might
throw and you won't know unless you read the docs.

---

## 8. Pointers and Value Receivers

This is often the trickiest concept for developers coming from garbage-collected
languages where references are implicit.

### Value vs Pointer

```go
func (c Config) InPortRange(port int) bool {    // value receiver: gets a COPY
    return port >= c.PortRange.Min && port <= c.PortRange.Max
}

func (c *Config) SetLabel(port int, label string) {  // pointer receiver: modifies original
    c.Labels[port] = label
}
```

**TypeScript equivalent:**
```typescript
// In TypeScript, objects are always passed by reference, so this distinction
// doesn't exist. In Go:

// Value receiver = like passing a deep clone
inPortRange(port: number): boolean { ... }

// Pointer receiver = like normal reference behavior in TS
setLabel(port: number, label: string): void { ... }
```

### When to Use Each

- **Pointer receiver `(c *Config)`**: When the method needs to modify the
  struct, or the struct is large and you want to avoid copying.
- **Value receiver `(c Config)`**: When the method only reads data, and the
  struct is small.

In the TUI code, `Model` methods use value receivers because Bubble Tea's
`Update` returns a new model:

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // m is a copy — modifications to m create a new state
    m.cursor++
    return m, nil   // return the modified copy
}
```

But mutation helpers use pointer receivers:

```go
func (m *Model) applyFilter() {
    // directly modifies m's fields
    m.filtered = ...
}
```

### The `&` and `*` Operators

```go
s := scanner.New(cfg.PortRange)   // s holds a Scanner interface value
return &darwinScanner{...}         // & creates a pointer to the struct
```

- `&x` = "get the address of x" (create a pointer)
- `*p` = "get the value that p points to" (dereference)
- In most code, Go handles this automatically. You rarely need to think about it.

---

## 9. Slices and Maps

### Slices (Dynamic Arrays)

```go
var servers []Server                  // nil slice (like: let servers: Server[] | null)
servers = append(servers, newServer)  // append creates a new slice if needed

// Slice literal
hidden := []int{22, 443}

// Make with capacity
result := make([]Server, 0, 10)      // len=0, cap=10

// Copy
copy(dest, src)

// Reslicing (Go's powerful slice trick)
filtered := m.servers[:0:0]           // empty slice, no shared backing array
```

**TypeScript equivalents:**
```typescript
let servers: Server[] = [];
servers.push(newServer);             // Go's append is immutable-ish (returns new)
const hidden = [22, 443];
const result = new Array(10);
```

### Slice Reslicing Idiom

This pattern appears in `filterHidden`:

```go
func (m *Model) filterHidden() {
    filtered := m.servers[:0:0]     // empty slice with zero capacity
    for _, s := range m.servers {
        if !m.config.IsHidden(s.Port) {
            filtered = append(filtered, s)
        }
    }
    m.servers = filtered
}
```

This is Go's idiomatic way to filter a slice. `[:0:0]` creates a new empty
slice that won't share memory with the original.

### Maps

```go
labels := make(map[int]string)      // like: new Map<number, string>()
labels[8080] = "web"                // set
name := labels[8080]                // get (returns zero value "" if missing)
name, ok := labels[8080]            // get with existence check
delete(labels, 8080)                // delete
```

**TypeScript equivalent:**
```typescript
const labels = new Map<number, string>();
labels.set(8080, "web");
const name = labels.get(8080);
labels.delete(8080);
```

### The `range` Keyword

```go
for i, server := range servers {    // like: servers.forEach((server, i) => ...)
    fmt.Println(i, server.Port)
}

for _, server := range servers {    // _ discards index (like omitting first param)
    fmt.Println(server.Port)
}

for port, label := range labels {   // iterate map
    fmt.Println(port, label)
}
```

---

## 10. Goroutines and Concurrency

### Goroutines (Lightweight Threads)

```go
// scanner.go — CheckHealth
func CheckHealth(servers []Server, timeout time.Duration) []Server {
    result := make([]Server, len(servers))
    copy(result, servers)

    var wg sync.WaitGroup           // like a Promise.all() coordinator
    for i := range result {
        wg.Add(1)                   // "one more task to wait for"
        go func(idx int) {          // launch a goroutine (lightweight thread)
            defer wg.Done()         // "this task is done" (runs when function exits)
            conn, err := net.DialTimeout("tcp",
                fmt.Sprintf("127.0.0.1:%d", result[idx].Port), timeout)
            if err == nil {
                result[idx].Healthy = true
                conn.Close()
            }
        }(i)                        // pass i by value to avoid closure capture bug
    }
    wg.Wait()                       // block until all goroutines complete
    return result
}
```

**TypeScript equivalent:**
```typescript
async function checkHealth(servers: Server[], timeout: number): Promise<Server[]> {
    const result = [...servers];
    await Promise.all(result.map(async (server, idx) => {
        try {
            const conn = await net.connect(server.port, { timeout });
            result[idx].healthy = true;
            conn.close();
        } catch {}
    }));
    return result;
}
```

### Key Differences from async/await

1. **`go func()` starts a goroutine** — it runs concurrently, not sequentially.
2. **`sync.WaitGroup`** is the coordination primitive (like `Promise.all`).
3. **`defer`** schedules cleanup to run when the function returns (like a
   `finally` block but more ergonomic).
4. **No colored functions**: In Go, any function can be run as a goroutine.
   There's no `async` marking. You don't need to "make a function async" to
   run it concurrently.

### The Closure Capture Gotcha

```go
// WRONG - all goroutines would share the same i:
for i := range result {
    go func() {
        result[i].Healthy = true    // i changes during loop!
    }()
}

// CORRECT - pass i as a parameter:
for i := range result {
    go func(idx int) {
        result[idx].Healthy = true  // idx is a copy, safe
    }(i)
}
```

This is similar to the classic JavaScript `var` in loop problem, but with
goroutines instead of closures.

---

## 11. Build Constraints (Platform-Specific Code)

Go uses build tags to compile different files for different platforms:

```go
// scanner_darwin.go
//go:build darwin    // only compiled on macOS

package scanner
// ... macOS-specific code using lsof
```

```go
// scanner_linux.go
//go:build linux     // only compiled on Linux

package scanner
// ... Linux-specific code using /proc
```

Both files define a `New()` function that returns `Scanner`. The Go compiler
picks the right file based on the target OS. This is like having:

```typescript
// scanner.ts (conceptual equivalent)
export function createScanner(): Scanner {
    if (process.platform === "darwin") {
        return new DarwinScanner();
    } else {
        return new LinuxScanner();
    }
}
```

But Go does it at **compile time**, not runtime. The macOS binary doesn't
contain any Linux code, and vice versa.

---

## 12. The Bubble Tea Architecture

[Bubble Tea](https://github.com/charmbracelet/bubbletea) is a Go TUI framework
inspired by The Elm Architecture (TEA). If you've used Redux, you'll feel at
home.

### The Pattern

```
          ┌──────────────────────────────────┐
          │         Bubble Tea Runtime        │
          │                                   │
          │   ┌─────┐  Msg   ┌────────┐      │
  User ──>│   │Input│ ──────>│Update()│      │
  Input   │   └─────┘        └───┬────┘      │
          │                      │            │
          │              Model + Cmd          │
          │                      │            │
          │                 ┌────▼───┐        │
  Screen <│                 │ View() │        │
  Output  │                 └────────┘        │
          │                                   │
          │   Cmd executes ──> returns Msg ───┘
          └──────────────────────────────────┘
```

### Mapping to Redux/React

| Bubble Tea        | React/Redux               | Portview File    |
|-------------------|---------------------------|------------------|
| `Model` struct    | State                     | `model.go`       |
| `Update(msg)`     | Reducer                   | `model.go`       |
| `View()`          | Render / JSX              | `view.go`        |
| `tea.Cmd`         | Thunk / Side effect       | `commands.go`    |
| `tea.Msg`         | Action                    | `commands.go`    |
| `Init()`          | Initial state + effects   | `model.go`       |

### The Model (State)

```go
// model.go
type Model struct {
    servers     []scanner.Server    // data from scanning
    filtered    []scanner.Server    // servers matching current filter
    cursor      int                 // which row is selected
    mode        mode                // current UI mode (normal, filter, label, etc.)
    scanner     scanner.Scanner     // the scanner to use for port discovery
    config      config.Config       // user configuration
    configPath  string              // where to save config
    width       int                 // terminal width
    height      int                 // terminal height
    lastRefresh time.Time           // when we last scanned
    filterText  string              // current filter query
    labelInput  textinput.Model     // text input component for labels
    err         error               // last error
}
```

Think of this as your Redux store / React state, containing everything the
UI needs.

### Update (The Reducer)

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {     // type switch — like switch + type assertion
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        return m, nil              // no side effects

    case scanResultMsg:            // scan completed
        if msg.err != nil {
            m.err = msg.err
            return m, nil
        }
        m.servers = msg.servers
        m.mergeLabels()
        m.applyFilter()
        return m, nil

    case tickMsg:                  // timer fired
        return m, tea.Batch(       // batch multiple commands
            m.doScan(),            // start a new scan
            doTick(m.config.RefreshInterval),  // schedule next tick
        )

    case tea.KeyMsg:               // keyboard input
        return m.handleKey(msg)
    }
    return m, nil
}
```

**Redux equivalent:**
```typescript
function reducer(state: State, action: Action): [State, SideEffect?] {
    switch (action.type) {
        case "WINDOW_RESIZE":
            return [{ ...state, width: action.width }, null];
        case "SCAN_RESULT":
            return [{ ...state, servers: action.servers }, null];
        case "TICK":
            return [state, batch(doScan(), doTick(interval))];
        case "KEY_PRESS":
            return handleKey(state, action);
    }
}
```

### Commands (Side Effects / Thunks)

Commands are functions that return a message after performing a side effect:

```go
// commands.go
func (m Model) doScan() tea.Cmd {
    s := m.scanner
    return func() tea.Msg {                    // returns a closure
        servers, err := s.Scan(context.Background())
        return scanResultMsg{servers: servers, err: err}
    }
}
```

**Redux thunk equivalent:**
```typescript
function doScan(): ThunkAction {
    return async (dispatch) => {
        const servers = await scanner.scan();
        dispatch({ type: "SCAN_RESULT", servers });
    };
}
```

### The Type Switch

This is Go's pattern matching. It works like a discriminated union in TypeScript:

```go
switch msg := msg.(type) {    // msg.(type) extracts the concrete type
case tea.WindowSizeMsg:       // msg is now typed as WindowSizeMsg
    m.width = msg.Width
case scanResultMsg:           // msg is now typed as scanResultMsg
    m.servers = msg.servers
case tea.KeyMsg:              // msg is now typed as KeyMsg
    return m.handleKey(msg)
}
```

**TypeScript equivalent:**
```typescript
// Like a discriminated union:
type Msg =
    | { type: "window_size"; width: number; height: number }
    | { type: "scan_result"; servers: Server[]; err?: Error }
    | { type: "key"; key: string };

switch (msg.type) {
    case "window_size": ...
    case "scan_result": ...
    case "key": ...
}
```

### View (Rendering)

```go
// view.go
func (m Model) View() string {     // returns a string — the terminal output
    var b strings.Builder           // efficient string concatenation

    b.WriteString(titleStyle.Render("portview"))
    b.WriteString("\n\n")

    for i, s := range m.filtered {
        cursor := "  "
        if i == m.cursor {
            cursor = cursorStyle.Render("> ")
        }
        // ... render each row
    }

    return b.String()
}
```

This is called after every `Update`. It returns a plain string that becomes
the terminal output. Think of it as React's `render()` method but returning
a string instead of JSX.

### Lipgloss (Styling)

```go
var titleStyle = lipgloss.NewStyle().
    Bold(true).
    Foreground(lipgloss.Color("205"))    // ANSI color 205 = pink

styled := titleStyle.Render("portview")  // returns styled string
```

Like CSS-in-JS but for terminals:
```typescript
// Conceptual TS equivalent:
const titleStyle = styled.span`
    font-weight: bold;
    color: #ff87ff;  /* ANSI 205 */
`;
```

---

## 13. Walking Through Each Package

### config/ — Configuration Management

**What it does:** Reads/writes a YAML config file, provides defaults.

**Key patterns to study:**

1. **Defaults + overlay**: `Load()` starts with defaults and unmarshals YAML
   on top:

    ```go
    func Load(path string) (Config, error) {
        cfg := Default()                        // start with defaults
        data, err := os.ReadFile(path)
        if err != nil {
            if errors.Is(err, os.ErrNotExist) {
                return cfg, nil                 // no file = use defaults
            }
            return Config{}, err
        }
        if err := yaml.Unmarshal(data, &cfg); err != nil {  // overlay on defaults
            return Config{}, err
        }
        if cfg.Labels == nil {
            cfg.Labels = make(map[int]string)   // ensure non-nil map
        }
        return cfg, nil
    }
    ```

2. **XDG Base Directory**: Config lives at `~/.config/portview/config.yaml`
   (or `$XDG_CONFIG_HOME/portview/config.yaml`). This is a Linux/macOS
   convention for user config files.

3. **Lazy directory creation**: `Save` creates parent dirs automatically:

    ```go
    func Save(path string, cfg Config) error {
        if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
            return err
        }
        data, err := yaml.Marshal(cfg)
        // ...
        return os.WriteFile(path, data, 0o644)    // 0o644 = file permissions
    }
    ```

### scanner/ — Port Discovery

**What it does:** Finds TCP servers listening on localhost.

**Key patterns to study:**

1. **Interface + build constraints**: `Scanner` interface has two
   implementations selected at compile time by OS.

2. **macOS approach** (scanner_darwin.go):
   - Runs `lsof -iTCP -sTCP:LISTEN -nP` to find listening ports
   - Runs `ps -p <PID> -o comm=,args=` per port for process details
   - Parses structured text output

3. **Linux approach** (scanner_linux.go):
   - Reads `/proc/net/tcp` (file, no exec) for listening ports
   - Runs single `ss -tlnp` for port-to-PID mapping
   - Reads `/proc/<PID>/comm` and `/proc/<PID>/cmdline` for process details
   - More efficient: fewer exec calls

4. **Text parsing**: Both platforms parse structured command output. This is
   common in Go systems programming:

    ```go
    // parse_lsof.go — parsing lsof output
    lines := strings.Split(trimmed, "\n")
    for _, line := range lines[1:] {         // skip header
        fields := strings.Fields(line)       // split by whitespace
        if len(fields) < 10 { continue }     // skip malformed lines
        pid, err := strconv.Atoi(fields[1])  // string to int
        if err != nil { continue }           // skip bad data
        // ...
    }
    ```

5. **Concurrent health checks** (`CheckHealth`):

    ```go
    var wg sync.WaitGroup
    for i := range result {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            // try TCP connection...
        }(i)
    }
    wg.Wait()
    ```

### tui/ — Terminal User Interface

**What it does:** The interactive terminal application using Bubble Tea.

**Four files, four responsibilities:**

| File           | Responsibility | React Analogy          |
|----------------|---------------|------------------------|
| `model.go`     | State + state transitions | State + reducer |
| `view.go`      | Rendering     | Component render       |
| `commands.go`  | Side effects  | useEffect / thunks     |
| `keys.go`      | Key bindings  | Event handler mapping  |

**State machine** in `model.go`:

```
modeNormal ──/──> modeFilter ──esc/enter──> modeNormal
     │                                           │
     x──> modeConfirmKill ──y──> kill ──> modeNormal
     │          │                                │
     │          n/esc────────────────────> modeNormal
     │                                           │
     l──> modeLabel ──enter──> save ──> modeNormal
     │        │                               │
     │        esc──────────────────────> modeNormal
     │                                        │
     ?──> modeHelp ──any key──> modeNormal
```

Each mode has a dedicated handler:
- `handleNormalKey`: Navigation, mode entry
- `handleFilterKey`: Text input for filtering
- `handleConfirmKillKey`: y/n confirmation
- `handleLabelKey`: Text input for labels (delegates to textinput component)
- `handleHelpKey`: Any key returns to normal

---

## 14. Testing in Go

### File Convention

Tests live in `*_test.go` files in the same package:

```go
// config_test.go
package config              // same package = access to unexported identifiers

import "testing"

func TestDefault_ReturnsExpectedValues(t *testing.T) {
    cfg := Default()

    if cfg.RefreshInterval != 3*time.Second {
        t.Errorf("RefreshInterval = %v, want %v",
            cfg.RefreshInterval, 3*time.Second)
    }
}
```

### Key Differences from TypeScript Testing

| Go                          | TypeScript (Jest/Vitest)         |
|-----------------------------|----------------------------------|
| `func TestXxx(t *testing.T)` | `test("xxx", () => { ... })`   |
| `t.Errorf("got %v", x)`    | `expect(x).toBe(y)`            |
| `t.Fatalf("fatal: %v", x)` | `throw new Error(...)` (stops)  |
| `t.Run("subtest", func...)`| `describe("subtest", () => ...)` |
| `go test ./...`            | `npm test`                       |
| No assertion library needed | Need expect/assert library       |

### Table-Driven Tests

A very common Go pattern:

```go
func TestInPortRange(t *testing.T) {
    cfg := Default()

    tests := []struct {        // anonymous struct slice
        name string
        port int
        want bool
    }{
        {"below min", 1023, false},
        {"at min", 1024, true},
        {"at max", 65535, true},
        {"above max", 65536, false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := cfg.InPortRange(tt.port); got != tt.want {
                t.Errorf("InPortRange(%d) = %v, want %v",
                    tt.port, got, tt.want)
            }
        })
    }
}
```

**TypeScript equivalent:**
```typescript
test.each([
    ["below min", 1023, false],
    ["at min", 1024, true],
    ["at max", 65535, true],
    ["above max", 65536, false],
])("%s", (name, port, expected) => {
    expect(cfg.inPortRange(port)).toBe(expected);
});
```

### t.TempDir() — Automatic Cleanup

```go
dir := t.TempDir()  // creates a temp directory, auto-deleted after test
path := filepath.Join(dir, "config.yaml")
```

Like `jest`'s `afterAll(() => cleanup())` but automatic.

### Running Tests

```bash
go test ./...              # run all tests in all packages
go test ./internal/config  # run tests in one package
go test -v ./...           # verbose output
go test -run TestFilter    # run tests matching pattern
```

---

## 15. Tooling and Build System

### Essential Commands

```bash
go build ./cmd/portview     # compile the binary
go run ./cmd/portview       # compile and run in one step
go test ./...               # run all tests
go mod tidy                 # clean up go.mod/go.sum (like npm prune)
go vet ./...                # static analysis (catches common bugs)
```

### The Makefile

```makefile
.PHONY: build test lint run clean

build:
    go build -o bin/portview ./cmd/portview

test:
    go test ./...

lint:
    golangci-lint run

run:
    go run ./cmd/portview

clean:
    rm -rf bin/
```

This is like `scripts` in package.json:
```json
{
    "scripts": {
        "build": "tsc && ...",
        "test": "jest",
        "lint": "eslint .",
        "start": "ts-node src/index.ts"
    }
}
```

### Linting

`.golangci.yaml` configures the Go linter (like `.eslintrc`):

```yaml
version: "2"
linters:
  enable:
    - errcheck      # checks that errors are handled
    - govet         # official Go static analysis
    - staticcheck   # advanced static analysis
    - unused        # finds unused code
    - gosimple      # suggests code simplifications
```

---

## Appendix: Reading Order

If you want to read the code in the most logical order:

1. **`go.mod`** — understand dependencies
2. **`cmd/portview/main.go`** — see how it all connects
3. **`internal/config/config.go`** — simple types, good intro to Go structs
4. **`internal/scanner/scanner.go`** — interfaces and concurrency
5. **`internal/scanner/scanner_darwin.go`** — platform-specific implementation
6. **`internal/scanner/parse_lsof.go`** — text parsing patterns
7. **`internal/tui/keys.go`** — simple definitions
8. **`internal/tui/commands.go`** — the Cmd pattern (async side effects)
9. **`internal/tui/model.go`** — the core state machine
10. **`internal/tui/view.go`** — rendering logic
11. **`internal/config/config_test.go`** — see how Go tests work
12. **`internal/scanner/scanner_test.go`** — testing with interfaces
13. **`internal/tui/tui_test.go`** — testing TUI components
