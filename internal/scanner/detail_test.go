package scanner

import (
	"context"
	"net"
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
	for _, in := range []string{
		"", "x", "42", "1:2:3:4", "a-01:02:03", "1:xx",
		"99:99",           // minutes/seconds must be < 60
		"01:99:00",        // out-of-range minutes
		"191510-00:00:42", // implausible day count (clock-skewed hosts report centuries)
		"-01:02:03",       // negative
	} {
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

// A nonsense etime (clock-skewed host) must not fail the inspection — uptime
// degrades to zero and the remaining fields survive.
func TestParsePSDetailToleratesGarbageEtime(t *testing.T) {
	d, err := parsePSDetail("191510-00:00:42 0.3 1.2 54321")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Uptime != 0 {
		t.Errorf("Uptime = %v, want 0 (unknown)", d.Uptime)
	}
	if d.RSSKB != 54321 {
		t.Errorf("RSSKB = %d, want 54321", d.RSSKB)
	}
}

func TestParseProcUptime(t *testing.T) {
	secs, err := parseProcUptime([]byte("35045.12 137941.61\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secs != 35045.12 {
		t.Errorf("secs = %v, want 35045.12", secs)
	}
	if _, err := parseProcUptime([]byte("")); err == nil {
		t.Error("empty input: expected error")
	}
	if _, err := parseProcUptime([]byte("abc")); err == nil {
		t.Error("non-numeric input: expected error")
	}
}

func TestParseStatStartTicks(t *testing.T) {
	// Real-shaped /proc/pid/stat line; comm "(a) b" contains a space and a
	// paren to exercise the last-')' scan. starttime (field 22) is 3389.
	stat := []byte("1234 ((a) b) S 1 1234 1234 0 -1 4194560 1000 0 0 0 10 5 0 0 20 0 8 0 3389 1000000 500 18446744073709551615")
	ticks, err := parseStatStartTicks(stat)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ticks != 3389 {
		t.Errorf("ticks = %d, want 3389", ticks)
	}
	if _, err := parseStatStartTicks([]byte("no paren here")); err == nil {
		t.Error("missing ')': expected error")
	}
	if _, err := parseStatStartTicks([]byte("1 (x) S 2 3")); err == nil {
		t.Error("short line: expected error")
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

func TestProbeHTTPIPv6(t *testing.T) {
	ln := listenLoopback(t, "tcp6", "[::1]:0")
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Server", "ipv6-testsrv")
		w.WriteHeader(http.StatusNoContent)
	}))
	_ = ts.Listener.Close() // replace httptest's auto-created listener below
	ts.Listener = ln
	ts.Start()
	defer ts.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	p := ProbeHTTP(context.Background(), port, time.Second)
	if !p.OK {
		t.Fatalf("IPv6 probe not OK: %+v", p)
	}
	if p.Status != http.StatusNoContent || p.Server != "ipv6-testsrv" {
		t.Errorf("probe = %+v, want status 204 from ipv6-testsrv", p)
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
