package config

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// PortRange defines the min/max ports to scan.
type PortRange struct {
	Min int `yaml:"min"`
	Max int `yaml:"max"`
}

// Config holds all portview configuration.
type Config struct {
	RefreshInterval time.Duration  `yaml:"refresh_interval"`
	PortRange       PortRange      `yaml:"port_range"`
	Labels          map[int]string `yaml:"labels"`
	Hidden          []int          `yaml:"hidden"`
}

// DefaultPath returns the default config file path using XDG Base Directory
// conventions. It checks $XDG_CONFIG_HOME first, falling back to ~/.config.
func DefaultPath() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "portview", "config.yaml")
}

// Default returns a Config with sensible defaults.
func Default() Config {
	return Config{
		RefreshInterval: 3 * time.Second,
		PortRange: PortRange{
			Min: 1024,
			Max: 65535,
		},
		Labels: make(map[int]string),
	}
}

// SetLabel sets a user-friendly label for the given port.
func (c *Config) SetLabel(port int, label string) {
	if c.Labels == nil {
		c.Labels = make(map[int]string)
	}
	c.Labels[port] = label
}

// RemoveLabel deletes the label for the given port.
func (c *Config) RemoveLabel(port int) {
	delete(c.Labels, port)
}

// IsHidden reports whether the given port is in the Hidden list.
func (c *Config) IsHidden(port int) bool {
	for _, p := range c.Hidden {
		if p == port {
			return true
		}
	}
	return false
}

// ToggleHidden adds the port to the Hidden list if it is not already present,
// or removes it if it is.
func (c *Config) ToggleHidden(port int) {
	for i, p := range c.Hidden {
		if p == port {
			c.Hidden = append(c.Hidden[:i], c.Hidden[i+1:]...)
			return
		}
	}
	c.Hidden = append(c.Hidden, port)
}

// InPortRange reports whether the given port falls within the configured
// port range (inclusive on both ends).
func (c *Config) InPortRange(port int) bool {
	return port >= c.PortRange.Min && port <= c.PortRange.Max
}

// Load reads a YAML config file from path and returns the parsed Config.
// If the file does not exist, it returns Default() with a nil error.
// The file contents are unmarshaled on top of Default(), so any fields
// not present in the YAML retain their default values.
func Load(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return Config{}, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	// Ensure Labels map is never nil after unmarshal.
	if cfg.Labels == nil {
		cfg.Labels = make(map[int]string)
	}

	return cfg, nil
}

// Save marshals cfg to YAML and writes it to path, creating parent directories
// as needed with lazy directory creation.
func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}
