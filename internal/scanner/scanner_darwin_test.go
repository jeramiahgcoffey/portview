//go:build darwin

package scanner

import "testing"

func TestParseLsof(t *testing.T) {
	out := `COMMAND     PID           USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
node       1234 user   23u  IPv4 0xaaaa      0t0  TCP 127.0.0.1:3000 (LISTEN)
node       1234 user   24u  IPv6 0xbbbb      0t0  TCP [::1]:3000 (LISTEN)
go         5678 user   12u  IPv4 0xcccc      0t0  TCP *:8080 (LISTEN)
ssh         999 user    3u  IPv4 0xdddd      0t0  TCP 127.0.0.1:22 (LISTEN)
chrome     4321 user   88u  IPv4 0xeeee      0t0  TCP 127.0.0.1:5000->127.0.0.1:6000 (ESTABLISHED)
`
	got := parseLsof([]byte(out))
	want := []listing{
		{Port: 3000, PID: 1234, Comm: "node"},
		{Port: 3000, PID: 1234, Comm: "node"},
		{Port: 8080, PID: 5678, Comm: "go"},
		{Port: 22, PID: 999, Comm: "ssh"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d listings, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("listing[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseLsofIgnoresGarbage(t *testing.T) {
	out := "\n   \nnot a real line\nnode 12 user 1u IPv4 0x 0t0 TCP notaport (LISTEN)\n"
	if got := parseLsof([]byte(out)); len(got) != 0 {
		t.Errorf("got %+v, want no listings from garbage input", got)
	}
}

func TestPortFromAddr(t *testing.T) {
	tests := []struct {
		addr     string
		wantPort int
		wantOK   bool
	}{
		{"127.0.0.1:3000", 3000, true},
		{"[::1]:8080", 8080, true},
		{"*:443", 443, true},
		{"127.0.0.1:", 0, false},
		{"noport", 0, false},
		{"*:abc", 0, false},
	}
	for _, tt := range tests {
		port, ok := portFromAddr(tt.addr)
		if port != tt.wantPort || ok != tt.wantOK {
			t.Errorf("portFromAddr(%q) = (%d, %v), want (%d, %v)", tt.addr, port, ok, tt.wantPort, tt.wantOK)
		}
	}
}

func TestProcName(t *testing.T) {
	tests := map[string]string{
		"/usr/local/bin/node\n": "node",
		"  /usr/sbin/sshd  ":    "sshd",
		"go":                    "go",
	}
	for in, want := range tests {
		if got := procName(in); got != want {
			t.Errorf("procName(%q) = %q, want %q", in, got, want)
		}
	}
}
