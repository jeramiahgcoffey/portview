package cli

import (
	"bytes"
	"strings"
	"testing"
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
	for _, want := range []string{"list", "kill", "open"} {
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
