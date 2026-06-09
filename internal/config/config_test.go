package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingReturnsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.yaml")
	got, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.RefreshInterval.Std() != DefaultRefreshInterval {
		t.Errorf("RefreshInterval = %v, want %v", got.RefreshInterval.Std(), DefaultRefreshInterval)
	}
	if got.PortRange.Min != DefaultMinPort || got.PortRange.Max != DefaultMaxPort {
		t.Errorf("PortRange = %+v, want {%d %d}", got.PortRange, DefaultMinPort, DefaultMaxPort)
	}
	if os.Getenv("CI") == "" {
		// Loading must never create the file (lazy creation).
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Errorf("Load created %s; want it left absent", path)
		}
	}
}

func TestLoadFullConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := `refresh_interval: 5s
port_range:
  min: 2000
  max: 9000
labels:
  3000: frontend
  8080: backend
hidden:
  - 5432
  - 6379
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.RefreshInterval.Std() != 5*time.Second {
		t.Errorf("RefreshInterval = %v, want 5s", got.RefreshInterval.Std())
	}
	if got.PortRange.Min != 2000 || got.PortRange.Max != 9000 {
		t.Errorf("PortRange = %+v, want {2000 9000}", got.PortRange)
	}
	if got.LabelFor(3000) != "frontend" || got.LabelFor(8080) != "backend" {
		t.Errorf("labels = %v, want frontend/backend", got.Labels)
	}
	if !got.IsHidden(5432) || !got.IsHidden(6379) || got.IsHidden(3000) {
		t.Errorf("hidden = %v, want [5432 6379] only", got.Hidden)
	}
}

func TestPartialConfigGetsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	// Only labels set; everything else should default.
	if err := os.WriteFile(path, []byte("labels:\n  3000: web\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.RefreshInterval.Std() != DefaultRefreshInterval {
		t.Errorf("RefreshInterval = %v, want default", got.RefreshInterval.Std())
	}
	if got.PortRange.Max != DefaultMaxPort {
		t.Errorf("PortRange.Max = %d, want default", got.PortRange.Max)
	}
	if got.LabelFor(3000) != "web" {
		t.Errorf("label 3000 = %q, want web", got.LabelFor(3000))
	}
}

func TestSaveCreatesFileLazilyAndRoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "portview", "config.yaml")

	cfg := Default()
	cfg.SetLabel(3000, "frontend")
	cfg.Hide(5432)
	cfg.RefreshInterval = Duration(2 * time.Second)

	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file created at %s: %v", path, err)
	}

	got, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if got.LabelFor(3000) != "frontend" {
		t.Errorf("label = %q, want frontend", got.LabelFor(3000))
	}
	if !got.IsHidden(5432) {
		t.Errorf("hidden = %v, want 5432 present", got.Hidden)
	}
	if got.RefreshInterval.Std() != 2*time.Second {
		t.Errorf("RefreshInterval = %v, want 2s", got.RefreshInterval.Std())
	}
}

func TestSetLabelEmptyClears(t *testing.T) {
	cfg := Default()
	cfg.SetLabel(3000, "frontend")
	cfg.SetLabel(3000, "")
	if cfg.LabelFor(3000) != "" {
		t.Errorf("label = %q, want cleared", cfg.LabelFor(3000))
	}
}

func TestHideUnhideIdempotent(t *testing.T) {
	cfg := Default()
	cfg.Hide(5432)
	cfg.Hide(5432) // no duplicate
	if len(cfg.Hidden) != 1 {
		t.Errorf("hidden = %v, want single entry", cfg.Hidden)
	}
	cfg.Unhide(5432)
	if cfg.IsHidden(5432) {
		t.Errorf("hidden = %v, want 5432 removed", cfg.Hidden)
	}
}

func TestDefaultPathRespectsXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	got, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	want := "/tmp/xdg/portview/config.yaml"
	if got != want {
		t.Errorf("DefaultPath = %q, want %q", got, want)
	}
}

func TestDefaultPathFallsBackToHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	got, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".config", "portview", "config.yaml")
	if got != want {
		t.Errorf("DefaultPath = %q, want %q", got, want)
	}
}
