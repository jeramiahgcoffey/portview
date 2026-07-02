package config

import (
	"testing"

	"github.com/jeramiahgcoffey/portview/internal/scanner"
)

func decorateTestServers() []scanner.Server {
	return []scanner.Server{
		{Port: 3000, Process: "node"},
		{Port: 5432, Process: "postgres"},
		{Port: 8080, Process: "go"},
	}
}

func TestDecorateHidesAndLabels(t *testing.T) {
	c := Default()
	c.SetLabel(3000, "frontend")
	c.Hide(5432)

	got := c.Decorate(decorateTestServers(), false)
	if len(got) != 2 {
		t.Fatalf("got %d servers, want 2 (5432 hidden)", len(got))
	}
	if got[0].Port != 3000 || got[0].Label != "frontend" {
		t.Errorf("server[0] = %+v, want port 3000 labeled frontend", got[0])
	}
	for _, s := range got {
		if s.Port == 5432 {
			t.Errorf("port 5432 should be hidden, got %+v", s)
		}
	}
}

func TestDecorateIncludeHiddenKeepsAll(t *testing.T) {
	c := Default()
	c.SetLabel(3000, "frontend")
	c.Hide(5432)

	got := c.Decorate(decorateTestServers(), true)
	if len(got) != 3 {
		t.Fatalf("got %d servers, want all 3 with includeHidden", len(got))
	}
	// Labels are still applied even when hidden ports are included.
	if got[0].Label != "frontend" {
		t.Errorf("server[0].Label = %q, want frontend", got[0].Label)
	}
}
