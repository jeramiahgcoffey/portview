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

	"gopkg.in/yaml.v3"
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
	if c.PortRange.Min == 0 {
		c.PortRange.Min = DefaultMinPort
	}
	if c.PortRange.Max == 0 {
		c.PortRange.Max = DefaultMaxPort
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

// Save writes the config to DefaultPath, creating the directory if needed.
func (c Config) Save() error {
	path, err := DefaultPath()
	if err != nil {
		return err
	}
	return c.SaveTo(path)
}

// SaveTo writes the config to an explicit path, creating parent directories as
// needed. This is the lazy-creation point: the file exists only after a save.
func (c Config) SaveTo(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("config: create dir for %s: %w", path, err)
	}
	data, err := yaml.Marshal(c.withDefaults())
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("config: write %s: %w", path, err)
	}
	return nil
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
