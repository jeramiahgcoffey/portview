package config

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a time.Duration that marshals to and from YAML as a Go duration
// string (e.g. "3s"), matching the design doc's config format. The stdlib
// time.Duration would otherwise serialize as an integer nanosecond count.
type Duration time.Duration

// Std returns the underlying time.Duration.
func (d Duration) Std() time.Duration { return time.Duration(d) }

// UnmarshalYAML parses a duration string such as "3s" or "500ms".
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return fmt.Errorf("config: refresh_interval must be a duration string: %w", err)
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("config: invalid duration %q: %w", s, err)
	}
	*d = Duration(parsed)
	return nil
}

// MarshalYAML renders the duration as a string like "3s".
func (d Duration) MarshalYAML() (any, error) {
	return time.Duration(d).String(), nil
}
