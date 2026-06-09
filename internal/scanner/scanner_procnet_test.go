package scanner

import "testing"

func TestParseProcNet(t *testing.T) {
	// Real /proc/net/tcp shape: header + LISTEN (0A) and non-LISTEN rows.
	// 0100007F:1538 -> 127.0.0.1:5432, 00000000:1F90 -> 0.0.0.0:8080.
	data := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:1538 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 11111 1 0000000000000000 100 0 0 10 0
   1: 00000000:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 22222 1 0000000000000000 100 0 0 10 0
   2: 0100007F:8AE2 0100007F:1538 01 00000000:00000000 00:00000000 00000000  1000        0 33333 1 0000000000000000 20 0 0 10 0
`
	got := parseProcNet([]byte(data))
	want := []procEntry{
		{Port: 5432, Inode: "11111"},
		{Port: 8080, Inode: "22222"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseProcNetIPv6(t *testing.T) {
	// IPv6 local_address is 32 hex chars + :PORT. 1538 -> 5432.
	data := `  sl  local_address                         remote_address                        st ... inode
   0: 00000000000000000000000001000000:1538 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000 1000 0 44444 1 x 100 0 0 10 0
`
	got := parseProcNet([]byte(data))
	if len(got) != 1 || got[0].Port != 5432 || got[0].Inode != "44444" {
		t.Fatalf("got %+v, want one entry port 5432 inode 44444", got)
	}
}

func TestPortFromHexAddr(t *testing.T) {
	tests := []struct {
		addr     string
		wantPort int
		wantOK   bool
	}{
		{"0100007F:1538", 5432, true},
		{"00000000:1F90", 8080, true},
		{"0100007F:0050", 80, true},
		{"0100007F:", 0, false},
		{"noport", 0, false},
		{"0100007F:ZZZZ", 0, false},
	}
	for _, tt := range tests {
		port, ok := portFromHexAddr(tt.addr)
		if port != tt.wantPort || ok != tt.wantOK {
			t.Errorf("portFromHexAddr(%q) = (%d, %v), want (%d, %v)", tt.addr, port, ok, tt.wantPort, tt.wantOK)
		}
	}
}

func TestParseInodeFromLink(t *testing.T) {
	tests := []struct {
		link      string
		wantInode string
		wantOK    bool
	}{
		{"socket:[12345]", "12345", true},
		{"socket:[1]", "1", true},
		{"/dev/null", "", false},
		{"pipe:[999]", "", false},
		{"socket:[", "", false},
	}
	for _, tt := range tests {
		inode, ok := parseInodeFromLink(tt.link)
		if inode != tt.wantInode || ok != tt.wantOK {
			t.Errorf("parseInodeFromLink(%q) = (%q, %v), want (%q, %v)", tt.link, inode, ok, tt.wantInode, tt.wantOK)
		}
	}
}

func TestParseCmdline(t *testing.T) {
	tests := map[string]string{
		"node\x00server.js\x00":      "node server.js",
		"postgres\x00-D\x00/data\x00": "postgres -D /data",
		"go":                          "go",
		"":                            "",
	}
	for in, want := range tests {
		if got := parseCmdline([]byte(in)); got != want {
			t.Errorf("parseCmdline(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDedupeEntriesByPort(t *testing.T) {
	in := []procEntry{
		{Port: 5432, Inode: "1"}, // tcp
		{Port: 5432, Inode: "2"}, // tcp6 duplicate
		{Port: 8080, Inode: "3"},
	}
	got := dedupeEntriesByPort(in)
	if len(got) != 2 {
		t.Fatalf("got %d, want 2: %+v", len(got), got)
	}
	if got[0].Port != 5432 || got[0].Inode != "1" {
		t.Errorf("first = %+v, want port 5432 inode 1 (first wins)", got[0])
	}
}
