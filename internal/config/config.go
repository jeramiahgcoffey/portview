// Package config loads and persists portview's user configuration: labels,
// hidden ports, and scan preferences. Paths are XDG-compliant. The tool works
// with sensible defaults when no config file exists, and the file is created
// lazily — only when a user action needs to persist something.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v3"

	"github.com/jeramiahgcoffey/portview/internal/scanner"
)

// Defaults mirror the scanner defaults from the design doc.
const (
	DefaultRefreshInterval = 3 * time.Second
	DefaultMinPort         = 1024
	DefaultMaxPort         = 65535
)

// Config is the on-disk configuration.
type Config struct {
	RefreshInterval Duration       `yaml:"refresh_interval"`
	PortRange       PortRange      `yaml:"port_range"`
	Labels          map[int]string `yaml:"labels"`
	Hidden          []int          `yaml:"hidden"`
}

// PortRange is the inclusive range of ports the scanner considers.
type PortRange struct {
	Min int `yaml:"min"`
	Max int `yaml:"max"`
}

// Default returns a Config populated with sensible defaults.
func Default() Config {
	return Config{
		RefreshInterval: Duration(DefaultRefreshInterval),
		PortRange:       PortRange{Min: DefaultMinPort, Max: DefaultMaxPort},
		Labels:          map[int]string{},
		Hidden:          []int{},
	}
}

// withDefaults fills any zero-valued fields with defaults. It is applied after
// loading so a partial config file still yields a complete, usable Config.
func (c Config) withDefaults() Config {
	if c.RefreshInterval <= 0 {
		c.RefreshInterval = Duration(DefaultRefreshInterval)
	}
	// Fill zero values, then guard against nonsensical ranges (e.g. min > max or
	// out-of-bounds ports) that would otherwise silently break scanning.
	if c.PortRange.Min <= 0 {
		c.PortRange.Min = DefaultMinPort
	}
	if c.PortRange.Max <= 0 || c.PortRange.Max > 65535 {
		c.PortRange.Max = DefaultMaxPort
	}
	if c.PortRange.Min > c.PortRange.Max {
		c.PortRange.Min, c.PortRange.Max = DefaultMinPort, DefaultMaxPort
	}
	if c.Labels == nil {
		c.Labels = map[int]string{}
	}
	if c.Hidden == nil {
		c.Hidden = []int{}
	}
	return c
}

// DefaultPath returns the config file path, respecting $XDG_CONFIG_HOME and
// falling back to ~/.config.
func DefaultPath() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("config: resolve home dir: %w", err)
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "portview", "config.yaml"), nil
}

// Load reads the config from DefaultPath. If the file does not exist it
// returns defaults with no error (the tool requires no config).
func Load() (Config, error) {
	path, err := DefaultPath()
	if err != nil {
		return Config{}, err
	}
	return LoadFrom(path)
}

// LoadFrom reads the config from an explicit path. A missing file yields
// defaults; a present-but-partial file is completed with defaults.
func LoadFrom(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Default(), nil
		}
		return Config{}, fmt.Errorf("config: read %s: %w", path, err)
	}

	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return c.withDefaults(), nil
}

// Save writes the complete config to DefaultPath under the same file lock used
// by Update. Callers modifying one preference should use Update so unrelated
// concurrent edits are merged instead of replaced by a stale snapshot.
func (c Config) Save() error {
	path, err := DefaultPath()
	if err != nil {
		return err
	}
	return c.SaveTo(path)
}

// SaveTo writes the complete config to an explicit path under an advisory
// sibling lock. The final rename is atomic, so readers never observe partial
// YAML. This is the lazy-creation point: the config exists only after a save.
func (c Config) SaveTo(path string) error {
	return withFileLock(path, func() error {
		return saveUnlocked(c, path)
	})
}

// Update reloads the current config while holding its file lock, applies
// mutate, and atomically saves when mutate reports a change. It is the safe
// path for preference-level edits made by concurrent CLI or TUI processes.
func Update(mutate func(*Config) bool) (Config, bool, error) {
	path, err := DefaultPath()
	if err != nil {
		return Config{}, false, err
	}
	return UpdateFrom(path, mutate)
}

// UpdateFrom is Update for an explicit path. The returned config is the
// locked, latest state after the mutation (or after a no-op).
func UpdateFrom(path string, mutate func(*Config) bool) (Config, bool, error) {
	var (
		updated Config
		changed bool
	)
	err := withFileLock(path, func() error {
		current, err := LoadFrom(path)
		if err != nil {
			return err
		}
		changed = mutate(&current)
		updated = current.withDefaults()
		if !changed {
			return nil
		}
		return saveUnlocked(updated, path)
	})
	if err != nil {
		return Config{}, false, err
	}
	return updated, changed, nil
}

func withFileLock(path string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("config: create dir for %s: %w", path, err)
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("config: open lock for %s: %w", path, err)
	}
	defer func() { _ = lock.Close() }()

	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return fmt.Errorf("config: lock %s: %w", path, err)
	}
	defer func() { _ = unix.Flock(int(lock.Fd()), unix.LOCK_UN) }()

	return fn()
}

func saveUnlocked(c Config, path string) error {
	data, err := yaml.Marshal(c.withDefaults())
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".portview-config-*")
	if err != nil {
		return fmt.Errorf("config: create temporary file for %s: %w", path, err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("config: chmod temporary file for %s: %w", path, err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("config: write temporary file for %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("config: sync temporary file for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("config: close temporary file for %s: %w", path, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("config: replace %s: %w", path, err)
	}
	return nil
}

// Decorate applies the config to a freshly scanned server list: it marks
// hidden ports, drops them unless includeHidden, and attaches each server's
// saved label. It is the single merge point shared by the TUI and CLI so their
// filtering behavior cannot drift apart.
func (c Config) Decorate(servers []scanner.Server, includeHidden bool) []scanner.Server {
	out := make([]scanner.Server, 0, len(servers))
	for _, s := range servers {
		s.Hidden = c.IsHidden(s.Port)
		if !includeHidden && s.Hidden {
			continue
		}
		s.Label = c.LabelFor(s.Port)
		out = append(out, s)
	}
	return out
}

// LabelFor returns the user label for a port, or "" if none is set.
func (c Config) LabelFor(port int) string {
	return c.Labels[port]
}

// IsHidden reports whether a port is in the hidden list.
func (c Config) IsHidden(port int) bool {
	for _, p := range c.Hidden {
		if p == port {
			return true
		}
	}
	return false
}

// SetLabel sets (or, with an empty label, clears) the label for a port. It
// mutates the receiver's maps in place; callers persist via Save.
func (c *Config) SetLabel(port int, label string) {
	if c.Labels == nil {
		c.Labels = map[int]string{}
	}
	if label == "" {
		delete(c.Labels, port)
		return
	}
	c.Labels[port] = label
}

// Hide adds a port to the hidden list if not already present.
func (c *Config) Hide(port int) {
	if c.IsHidden(port) {
		return
	}
	c.Hidden = append(c.Hidden, port)
}

// Unhide removes a port from the hidden list.
func (c *Config) Unhide(port int) {
	out := c.Hidden[:0]
	for _, p := range c.Hidden {
		if p != port {
			out = append(out, p)
		}
	}
	c.Hidden = out
}
