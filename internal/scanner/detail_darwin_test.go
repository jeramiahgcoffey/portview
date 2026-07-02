//go:build darwin

package scanner

import "testing"

func TestParseLsofCWD(t *testing.T) {
	out := []byte("p1234\nfcwd\nn/Users/me/projects/app\n")
	if got := parseLsofCWD(out); got != "/Users/me/projects/app" {
		t.Errorf("parseLsofCWD = %q, want /Users/me/projects/app", got)
	}
}

func TestParseLsofCWDEmpty(t *testing.T) {
	if got := parseLsofCWD(nil); got != "" {
		t.Errorf("parseLsofCWD(nil) = %q, want empty", got)
	}
	if got := parseLsofCWD([]byte("p1234\n")); got != "" {
		t.Errorf("parseLsofCWD(no n-line) = %q, want empty", got)
	}
}
