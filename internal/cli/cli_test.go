package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeramiahgcoffey/portview/internal/config"
	"github.com/jeramiahgcoffey/portview/internal/scanner"
)

func run(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = Run(args, "test-version", &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestVersionSubcommand(t *testing.T) {
	code, out, _ := run("version")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "portview test-version") {
		t.Errorf("output = %q, want version string", out)
	}
}

func TestHelpSubcommand(t *testing.T) {
	code, out, _ := run("help")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{"list", "kill", "open", "hide", "unhide", "hidden"} {
		if !strings.Contains(out, want) {
			t.Errorf("help output missing %q", want)
		}
	}
}

func TestUnknownSubcommand(t *testing.T) {
	code, _, errOut := run("frobnicate")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "unknown command") {
		t.Errorf("stderr = %q, want unknown-command message", errOut)
	}
}

func TestNoArgsPrintsUsage(t *testing.T) {
	code, _, errOut := run()
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "Usage") {
		t.Errorf("stderr = %q, want usage", errOut)
	}
}

func TestPortArg(t *testing.T) {
	tests := []struct {
		args   []string
		wantOK bool
		want   int
	}{
		{[]string{"3000"}, true, 3000},
		{[]string{"1"}, true, 1},
		{[]string{"65535"}, true, 65535},
		{[]string{}, false, 0},
		{[]string{"3000", "extra"}, false, 0},
		{[]string{"abc"}, false, 0},
		{[]string{"0"}, false, 0},
		{[]string{"-1"}, false, 0},
		{[]string{"65536"}, false, 0},
	}
	for _, tt := range tests {
		var errOut bytes.Buffer
		got, ok := portArg("kill", tt.args, &errOut)
		if ok != tt.wantOK || got != tt.want {
			t.Errorf("portArg(%v) = (%d, %v), want (%d, %v)", tt.args, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestKillInvalidPortExitsBeforeScanning(t *testing.T) {
	code, _, errOut := run("kill", "not-a-port")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "invalid port") {
		t.Errorf("stderr = %q, want invalid-port message", errOut)
	}
}

func TestServerTableMarksHiddenRows(t *testing.T) {
	var out bytes.Buffer
	servers := []scanner.Server{
		{Port: 3000, PID: 1, Process: "node", Command: "next dev", Healthy: true},
		{Port: 5432, PID: 2, Process: "postgres", Command: "postgres", Hidden: true, Healthy: true},
	}
	writeServerTable(&out, servers, true)
	got := out.String()
	if !strings.Contains(got, "HIDDEN") {
		t.Fatalf("table missing HIDDEN header:\n%s", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "5432") && !strings.Contains(line, "yes") {
			t.Fatalf("hidden row not marked yes:\n%s", line)
		}
		if strings.Contains(line, "3000") && strings.Contains(line, "yes") {
			t.Fatalf("visible row marked hidden:\n%s", line)
		}
	}

	out.Reset()
	writeServerTable(&out, servers[:1], false)
	if strings.Contains(out.String(), "HIDDEN") {
		t.Fatalf("default table changed shape with an empty hidden column:\n%s", out.String())
	}
}

func TestHideAndUnhidePersistConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	code, out, errOut := run("hide", "5432")
	if code != 0 || errOut != "" || !strings.Contains(out, "hid port 5432") {
		t.Fatalf("hide = code %d, stdout %q, stderr %q", code, out, errOut)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load after hide: %v", err)
	}
	if !cfg.IsHidden(5432) {
		t.Fatalf("hidden = %v, want 5432", cfg.Hidden)
	}

	code, out, errOut = run("hide", "5432")
	if code != 0 || errOut != "" || !strings.Contains(out, "already hidden") {
		t.Fatalf("idempotent hide = code %d, stdout %q, stderr %q", code, out, errOut)
	}

	code, out, errOut = run("unhide", "5432")
	if code != 0 || errOut != "" || !strings.Contains(out, "unhid port 5432") {
		t.Fatalf("unhide = code %d, stdout %q, stderr %q", code, out, errOut)
	}
	cfg, err = config.Load()
	if err != nil {
		t.Fatalf("load after unhide: %v", err)
	}
	if cfg.IsHidden(5432) {
		t.Fatalf("hidden = %v, want 5432 removed", cfg.Hidden)
	}
}

func TestUnhideAbsentPortIsIdempotentWithoutCreatingConfig(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)

	code, out, errOut := run("unhide", "3000")
	if code != 0 || errOut != "" || !strings.Contains(out, "is not hidden") {
		t.Fatalf("unhide absent = code %d, stdout %q, stderr %q", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(root, "portview", "config.yaml")); !os.IsNotExist(err) {
		t.Fatalf("idempotent unhide created config, stat err = %v", err)
	}
}

func TestHiddenListsConfiguredPortsSortedWithLabels(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := config.Default()
	cfg.SetLabel(8080, "backend")
	cfg.Hide(8080)
	cfg.Hide(3000)
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	code, out, errOut := run("hidden")
	if code != 0 || errOut != "" {
		t.Fatalf("hidden = code %d, stderr %q", code, errOut)
	}
	if !strings.Contains(out, "PORT") || !strings.Contains(out, "backend") {
		t.Fatalf("hidden output missing header/label:\n%s", out)
	}
	if strings.Index(out, "3000") > strings.Index(out, "8080") {
		t.Fatalf("hidden ports not sorted:\n%s", out)
	}

	code, out, errOut = run("hidden", "--json")
	if code != 0 || errOut != "" {
		t.Fatalf("hidden --json = code %d, stderr %q", code, errOut)
	}
	var got []struct {
		Port  int    `json:"port"`
		Label string `json:"label"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode hidden JSON: %v\n%s", err, out)
	}
	if len(got) != 2 || got[0].Port != 3000 || got[1].Port != 8080 || got[1].Label != "backend" {
		t.Fatalf("hidden JSON = %+v, want sorted 3000/8080 with backend label", got)
	}
}

func TestHiddenRejectsArguments(t *testing.T) {
	code, _, errOut := run("hidden", "extra")
	if code != 2 || !strings.Contains(errOut, "usage: portview hidden") {
		t.Fatalf("hidden extra = code %d, stderr %q", code, errOut)
	}
}
