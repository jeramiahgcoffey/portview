package docker

import (
	"context"
	"reflect"
	"testing"

	"github.com/jeramiahgcoffey/portview/internal/scanner"
)

func TestParseHostPorts(t *testing.T) {
	tests := []struct {
		in   string
		want []int
	}{
		{"0.0.0.0:5432->5432/tcp, :::5432->5432/tcp", []int{5432}},
		{"127.0.0.1:8080->80/tcp", []int{8080}},
		{"0.0.0.0:6379->6379/tcp, 0.0.0.0:6380->6380/tcp", []int{6379, 6380}},
		{"6379/tcp", nil},     // exposed but not published
		{"", nil},             // no ports at all
		{"garbage", nil},      // unparseable
		{"host->80/tcp", nil}, // no port in host part
	}
	for _, tt := range tests {
		if got := parseHostPorts(tt.in); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("parseHostPorts(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParsePortMap(t *testing.T) {
	out := []byte(`{"ID":"abc123","Names":"my-postgres","Image":"postgres:16","Ports":"0.0.0.0:5432->5432/tcp, :::5432->5432/tcp"}
{"ID":"def456","Names":"cache","Image":"redis:7","Ports":"0.0.0.0:6379->6379/tcp"}

not-json
{"ID":"ghi789","Names":"unpublished","Image":"busybox","Ports":"8080/tcp"}
`)
	m := parsePortMap(out)
	if len(m) != 2 {
		t.Fatalf("map size = %d, want 2: %+v", len(m), m)
	}
	if c := m[5432]; c.Name != "my-postgres" || c.Image != "postgres:16" || c.ID != "abc123" {
		t.Errorf("m[5432] = %+v, want my-postgres/postgres:16/abc123", c)
	}
	if c := m[6379]; c.Name != "cache" {
		t.Errorf("m[6379] = %+v, want cache", c)
	}
}

func TestLooksDockerOwned(t *testing.T) {
	tests := []struct {
		process string
		want    bool
	}{
		{"com.docker.backend", true},
		{"docker-proxy", true},
		{"Docker Desktop", true},
		{"OrbStack Helper", true},
		{"node", false},
		{"postgres", false},
	}
	for _, tt := range tests {
		got := looksDockerOwned(scanner.Server{Process: tt.process})
		if got != tt.want {
			t.Errorf("looksDockerOwned(%q) = %v, want %v", tt.process, got, tt.want)
		}
	}
}

// TestEnrichNoDockerProcesses verifies Enrich is a no-op (and never shells
// out) when nothing in the scan looks Docker-owned.
func TestEnrichNoDockerProcesses(t *testing.T) {
	in := []scanner.Server{
		{Port: 3000, Process: "node"},
		{Port: 5432, Process: "postgres"},
	}
	got := Enrich(context.Background(), in)
	if !reflect.DeepEqual(got, in) {
		t.Errorf("Enrich changed non-docker servers: %+v", got)
	}
}
