package config

import (
	"testing"
	"time"
)

func TestDefault_ReturnsExpectedValues(t *testing.T) {
	cfg := Default()

	if cfg.RefreshInterval != 3*time.Second {
		t.Errorf("RefreshInterval = %v, want %v", cfg.RefreshInterval, 3*time.Second)
	}
	if cfg.PortRange.Min != 1024 {
		t.Errorf("PortRange.Min = %d, want 1024", cfg.PortRange.Min)
	}
	if cfg.PortRange.Max != 65535 {
		t.Errorf("PortRange.Max = %d, want 65535", cfg.PortRange.Max)
	}
	if cfg.Labels == nil {
		t.Error("Labels should not be nil")
	}
	if len(cfg.Labels) != 0 {
		t.Errorf("Labels should be empty, got %d entries", len(cfg.Labels))
	}
	if cfg.Hidden != nil {
		t.Errorf("Hidden should be nil, got %v", cfg.Hidden)
	}
}

func TestDefault_LabelsMapIsNotNil(t *testing.T) {
	cfg := Default()

	// Should be safe to write to without panic
	cfg.Labels[8080] = "test"
	if cfg.Labels[8080] != "test" {
		t.Error("expected to be able to write to Labels map")
	}
}
