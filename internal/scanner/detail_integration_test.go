//go:build integration

package scanner

import (
	"context"
	"os"
	"testing"
)

// TestInspectSelf exercises the real ps/cwd path by inspecting the test
// process itself. Gated behind the `integration` build tag like the scan
// integration test.
func TestInspectSelf(t *testing.T) {
	d, err := Inspect(context.Background(), os.Getpid())
	if err != nil {
		t.Fatalf("Inspect(self): %v", err)
	}
	wd, _ := os.Getwd()
	if d.CWD != wd {
		t.Errorf("CWD = %q, want %q", d.CWD, wd)
	}
	// A just-started test process legitimately reports etime 00:00, so only
	// a negative uptime is wrong.
	if d.Uptime < 0 {
		t.Errorf("Uptime = %v, want >= 0", d.Uptime)
	}
	if d.RSSKB <= 0 {
		t.Errorf("RSSKB = %d, want > 0", d.RSSKB)
	}
}
