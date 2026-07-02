package scanner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func TestParseEtime(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"00:42", 42 * time.Second},
		{"05:03", 5*time.Minute + 3*time.Second},
		{"1:02:03", time.Hour + 2*time.Minute + 3*time.Second},
		{"23:59:59", 23*time.Hour + 59*time.Minute + 59*time.Second},
		{"12-01:02:03", 12*24*time.Hour + time.Hour + 2*time.Minute + 3*time.Second},
		{"3-00:00:01", 3*24*time.Hour + time.Second},
	}
	for _, tt := range tests {
		got, err := parseEtime(tt.in)
		if err != nil {
			t.Errorf("parseEtime(%q): unexpected error %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseEtime(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseEtimeRejectsGarbage(t *testing.T) {
	for _, in := range []string{"", "x", "42", "1:2:3:4", "a-01:02:03", "1:xx"} {
		if _, err := parseEtime(in); err == nil {
			t.Errorf("parseEtime(%q): expected error, got none", in)
		}
	}
}

func TestParsePSDetail(t *testing.T) {
	d, err := parsePSDetail("  1-02:03:04  0.3  1.2  54321\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantUptime := 26*time.Hour + 3*time.Minute + 4*time.Second
	if d.Uptime != wantUptime {
		t.Errorf("Uptime = %v, want %v", d.Uptime, wantUptime)
	}
	if d.CPUPercent != 0.3 || d.MemPercent != 1.2 || d.RSSKB != 54321 {
		t.Errorf("cpu/mem/rss = %v/%v/%v, want 0.3/1.2/54321", d.CPUPercent, d.MemPercent, d.RSSKB)
	}
}

func TestParsePSDetailRejectsGarbage(t *testing.T) {
	for _, in := range []string{"", "1 2 3", "01:00 a 1.0 100", "01:00 1.0 1.0 x"} {
		if _, err := parsePSDetail(in); err == nil {
			t.Errorf("parsePSDetail(%q): expected error, got none", in)
		}
	}
}

// serverPort extracts the port an httptest server is listening on.
func serverPort(t *testing.T, ts *httptest.Server) int {
	t.Helper()
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parse test server port: %v", err)
	}
	return port
}

func TestProbeHTTPSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Server", "testsrv")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	p := ProbeHTTP(context.Background(), serverPort(t, ts), time.Second)
	if !p.OK {
		t.Fatalf("probe not OK: %+v", p)
	}
	if p.Status != http.StatusOK {
		t.Errorf("Status = %d, want 200", p.Status)
	}
	if p.Server != "testsrv" {
		t.Errorf("Server = %q, want testsrv", p.Server)
	}
	if p.Latency <= 0 {
		t.Errorf("Latency = %v, want > 0", p.Latency)
	}
}

func TestProbeHTTPDoesNotFollowRedirects(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/elsewhere", http.StatusMovedPermanently)
	}))
	defer ts.Close()

	p := ProbeHTTP(context.Background(), serverPort(t, ts), time.Second)
	if !p.OK || p.Status != http.StatusMovedPermanently {
		t.Fatalf("probe = %+v, want OK with status 301", p)
	}
}

func TestProbeHTTPClosedPort(t *testing.T) {
	// Grab a port that is definitely closed: listen, note the port, close.
	ts := httptest.NewServer(http.NotFoundHandler())
	port := serverPort(t, ts)
	ts.Close()

	p := ProbeHTTP(context.Background(), port, 500*time.Millisecond)
	if p.OK {
		t.Fatalf("probe OK against closed port: %+v", p)
	}
	if p.Err == "" {
		t.Error("expected non-empty Err for closed port")
	}
}
