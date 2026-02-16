package config

import "time"

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
