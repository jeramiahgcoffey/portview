package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
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

func TestDefaultPath_WithXDGSet(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	got := DefaultPath()
	want := filepath.Join(tmpDir, "portview", "config.yaml")

	if got != want {
		t.Errorf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestDefaultPath_WithoutXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("could not get home dir: %v", err)
	}

	got := DefaultPath()
	want := filepath.Join(homeDir, ".config", "portview", "config.yaml")

	if got != want {
		t.Errorf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestSetLabel(t *testing.T) {
	cfg := Default()

	// Set a label and verify
	cfg.SetLabel(8080, "web-server")
	if got := cfg.Labels[8080]; got != "web-server" {
		t.Errorf("Labels[8080] = %q, want %q", got, "web-server")
	}

	// Overwrite the label and verify
	cfg.SetLabel(8080, "api-gateway")
	if got := cfg.Labels[8080]; got != "api-gateway" {
		t.Errorf("Labels[8080] after overwrite = %q, want %q", got, "api-gateway")
	}
}

func TestRemoveLabel(t *testing.T) {
	cfg := Default()

	// Set then remove a label
	cfg.SetLabel(3000, "frontend")
	cfg.RemoveLabel(3000)

	if got, ok := cfg.Labels[3000]; ok {
		t.Errorf("Labels[3000] should be absent after RemoveLabel, got %q", got)
	}
}

func TestIsHidden(t *testing.T) {
	cfg := Default()

	// Initially not hidden
	if cfg.IsHidden(9090) {
		t.Error("port 9090 should not be hidden initially")
	}

	// After toggling on, it should be hidden
	cfg.ToggleHidden(9090)
	if !cfg.IsHidden(9090) {
		t.Error("port 9090 should be hidden after ToggleHidden")
	}
}

func TestToggleHidden(t *testing.T) {
	cfg := Default()

	// Toggle on
	cfg.ToggleHidden(4000)
	if !cfg.IsHidden(4000) {
		t.Error("port 4000 should be hidden after first toggle")
	}

	// Toggle off
	cfg.ToggleHidden(4000)
	if cfg.IsHidden(4000) {
		t.Error("port 4000 should not be hidden after second toggle")
	}
}

func TestInPortRange(t *testing.T) {
	cfg := Default() // Min=1024, Max=65535

	tests := []struct {
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
				t.Errorf("InPortRange(%d) = %v, want %v", tt.port, got, tt.want)
			}
		})
	}
}

func TestSave_CreatesDirectoryAndFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "deep", "dir")
	path := filepath.Join(dir, "config.yaml")

	cfg := Default()
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected file to exist at %s, got error: %v", path, err)
	}
	if info.IsDir() {
		t.Fatal("expected a file, got a directory")
	}
	if info.Size() == 0 {
		t.Fatal("expected non-empty file")
	}
}

func TestSave_Roundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	original := Config{
		RefreshInterval: 5 * time.Second,
		PortRange:       PortRange{Min: 2000, Max: 50000},
		Labels:          map[int]string{8080: "web", 3000: "api"},
		Hidden:          []int{22, 443},
	}

	if err := Save(path, original); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var loaded Config
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if loaded.RefreshInterval != original.RefreshInterval {
		t.Errorf("RefreshInterval = %v, want %v", loaded.RefreshInterval, original.RefreshInterval)
	}
	if loaded.PortRange.Min != original.PortRange.Min {
		t.Errorf("PortRange.Min = %d, want %d", loaded.PortRange.Min, original.PortRange.Min)
	}
	if loaded.PortRange.Max != original.PortRange.Max {
		t.Errorf("PortRange.Max = %d, want %d", loaded.PortRange.Max, original.PortRange.Max)
	}
	if len(loaded.Labels) != len(original.Labels) {
		t.Errorf("Labels length = %d, want %d", len(loaded.Labels), len(original.Labels))
	}
	for port, label := range original.Labels {
		if loaded.Labels[port] != label {
			t.Errorf("Labels[%d] = %q, want %q", port, loaded.Labels[port], label)
		}
	}
	if len(loaded.Hidden) != len(original.Hidden) {
		t.Errorf("Hidden length = %d, want %d", len(loaded.Hidden), len(original.Hidden))
	}
	for i, v := range original.Hidden {
		if loaded.Hidden[i] != v {
			t.Errorf("Hidden[%d] = %d, want %d", i, loaded.Hidden[i], v)
		}
	}
}

func TestSave_OverwritesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	first := Default()
	first.Labels[9090] = "first-service"

	if err := Save(path, first); err != nil {
		t.Fatalf("first Save() error = %v", err)
	}

	second := Config{
		RefreshInterval: 10 * time.Second,
		PortRange:       PortRange{Min: 3000, Max: 40000},
		Labels:          map[int]string{4000: "second-service"},
		Hidden:          []int{80},
	}

	if err := Save(path, second); err != nil {
		t.Fatalf("second Save() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var loaded Config
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if loaded.RefreshInterval != 10*time.Second {
		t.Errorf("RefreshInterval = %v, want %v", loaded.RefreshInterval, 10*time.Second)
	}
	if loaded.PortRange.Min != 3000 {
		t.Errorf("PortRange.Min = %d, want 3000", loaded.PortRange.Min)
	}
	if loaded.PortRange.Max != 40000 {
		t.Errorf("PortRange.Max = %d, want 40000", loaded.PortRange.Max)
	}
	if len(loaded.Labels) != 1 {
		t.Errorf("Labels length = %d, want 1", len(loaded.Labels))
	}
	if loaded.Labels[4000] != "second-service" {
		t.Errorf("Labels[4000] = %q, want %q", loaded.Labels[4000], "second-service")
	}
	// Ensure old label is gone
	if _, ok := loaded.Labels[9090]; ok {
		t.Error("expected Labels[9090] to be absent after overwrite")
	}
	if len(loaded.Hidden) != 1 || loaded.Hidden[0] != 80 {
		t.Errorf("Hidden = %v, want [80]", loaded.Hidden)
	}
}

func TestLoad_MissingFile_ReturnsDefaults(t *testing.T) {
	cfg, err := Load("/tmp/portview_nonexistent_config_file.yaml")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}

	def := Default()
	if cfg.RefreshInterval != def.RefreshInterval {
		t.Errorf("RefreshInterval = %v, want %v", cfg.RefreshInterval, def.RefreshInterval)
	}
	if cfg.PortRange.Min != def.PortRange.Min {
		t.Errorf("PortRange.Min = %d, want %d", cfg.PortRange.Min, def.PortRange.Min)
	}
	if cfg.PortRange.Max != def.PortRange.Max {
		t.Errorf("PortRange.Max = %d, want %d", cfg.PortRange.Max, def.PortRange.Max)
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

func TestLoad_ValidYAML_ParsesAllFields(t *testing.T) {
	content := `refresh_interval: 5s
port_range:
  min: 2000
  max: 9000
labels:
  8080: "web"
  3000: "api"
hidden:
  - 22
  - 443
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.RefreshInterval != 5*time.Second {
		t.Errorf("RefreshInterval = %v, want %v", cfg.RefreshInterval, 5*time.Second)
	}
	if cfg.PortRange.Min != 2000 {
		t.Errorf("PortRange.Min = %d, want 2000", cfg.PortRange.Min)
	}
	if cfg.PortRange.Max != 9000 {
		t.Errorf("PortRange.Max = %d, want 9000", cfg.PortRange.Max)
	}
	if len(cfg.Labels) != 2 {
		t.Fatalf("Labels length = %d, want 2", len(cfg.Labels))
	}
	if cfg.Labels[8080] != "web" {
		t.Errorf("Labels[8080] = %q, want %q", cfg.Labels[8080], "web")
	}
	if cfg.Labels[3000] != "api" {
		t.Errorf("Labels[3000] = %q, want %q", cfg.Labels[3000], "api")
	}
	if len(cfg.Hidden) != 2 {
		t.Fatalf("Hidden length = %d, want 2", len(cfg.Hidden))
	}
	if cfg.Hidden[0] != 22 {
		t.Errorf("Hidden[0] = %d, want 22", cfg.Hidden[0])
	}
	if cfg.Hidden[1] != 443 {
		t.Errorf("Hidden[1] = %d, want 443", cfg.Hidden[1])
	}
}

func TestLoad_PartialYAML_MergesWithDefaults(t *testing.T) {
	content := `labels:
  5432: "postgres"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	def := Default()
	// Fields not in YAML should retain defaults
	if cfg.RefreshInterval != def.RefreshInterval {
		t.Errorf("RefreshInterval = %v, want default %v", cfg.RefreshInterval, def.RefreshInterval)
	}
	if cfg.PortRange.Min != def.PortRange.Min {
		t.Errorf("PortRange.Min = %d, want default %d", cfg.PortRange.Min, def.PortRange.Min)
	}
	if cfg.PortRange.Max != def.PortRange.Max {
		t.Errorf("PortRange.Max = %d, want default %d", cfg.PortRange.Max, def.PortRange.Max)
	}
	// Labels should have the value from YAML
	if len(cfg.Labels) != 1 {
		t.Fatalf("Labels length = %d, want 1", len(cfg.Labels))
	}
	if cfg.Labels[5432] != "postgres" {
		t.Errorf("Labels[5432] = %q, want %q", cfg.Labels[5432], "postgres")
	}
	// Hidden should remain nil (not set in YAML)
	if cfg.Hidden != nil {
		t.Errorf("Hidden should be nil, got %v", cfg.Hidden)
	}
}

func TestLoad_InvalidYAML_ReturnsError(t *testing.T) {
	content := `{{{not valid yaml: [}`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoad_EmptyFile_ReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	def := Default()
	if cfg.RefreshInterval != def.RefreshInterval {
		t.Errorf("RefreshInterval = %v, want %v", cfg.RefreshInterval, def.RefreshInterval)
	}
	if cfg.PortRange.Min != def.PortRange.Min {
		t.Errorf("PortRange.Min = %d, want %d", cfg.PortRange.Min, def.PortRange.Min)
	}
	if cfg.PortRange.Max != def.PortRange.Max {
		t.Errorf("PortRange.Max = %d, want %d", cfg.PortRange.Max, def.PortRange.Max)
	}
	if cfg.Labels == nil {
		t.Error("Labels should not be nil")
	}
	if len(cfg.Labels) != 0 {
		t.Errorf("Labels should be empty, got %d entries", len(cfg.Labels))
	}
}
