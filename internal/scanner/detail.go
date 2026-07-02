package scanner

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Detail holds on-demand process insight for a single server. It is fetched
// only when the user asks (the insight pane, not the poll loop) because it
// costs extra process lookups per call.
type Detail struct {
	CWD        string        `json:"cwd"`         // process working directory (usually the project root)
	Uptime     time.Duration `json:"uptime"`      // elapsed time since the process started
	CPUPercent float64       `json:"cpu_percent"` // instantaneous CPU usage as reported by ps
	MemPercent float64       `json:"mem_percent"` // physical memory usage as reported by ps
	RSSKB      int64         `json:"rss_kb"`      // resident set size in kilobytes
}

// HTTPProbe is the result of an on-demand HTTP GET against a port. A server
// that answers with any HTTP status — even a 500 — is an HTTP server; only a
// protocol-level failure marks the probe as not OK.
type HTTPProbe struct {
	OK      bool          `json:"ok"`               // true if the port spoke HTTP at all
	Status  int           `json:"status,omitempty"` // HTTP status code when OK
	Latency time.Duration `json:"latency"`          // time to first response
	Server  string        `json:"server,omitempty"` // Server response header, if sent
	Err     string        `json:"error,omitempty"`  // failure description when not OK
}

// DefaultProbeTimeout bounds the on-demand HTTP probe.
const DefaultProbeTimeout = 2 * time.Second

// DefaultInspectTimeout bounds the on-demand process inspection when the caller
// supplies no deadline, so a stuck ps/lsof can't hang the insight pane forever.
const DefaultInspectTimeout = 2 * time.Second

// Inspect returns process insight for pid. The cwd lookup is platform-specific
// (see detail_darwin.go / detail_linux.go); the rest comes from ps, which
// speaks the same dialect on macOS and Linux for these columns.
func Inspect(ctx context.Context, pid int) (Detail, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultInspectTimeout)
		defer cancel()
	}
	d, err := psDetail(ctx, pid)
	if err != nil {
		return Detail{}, err
	}
	d.CWD = processCWD(ctx, pid)
	// On Linux a /proc-based uptime replaces the ps etime value, which can be
	// garbage on hosts with boot-time clock skew (e.g. some CI runners).
	if u, ok := processUptime(ctx, pid); ok {
		d.Uptime = u
	}
	return d, nil
}

// psDetail fetches uptime, CPU, memory, and RSS for pid in a single ps call.
func psDetail(ctx context.Context, pid int) (Detail, error) {
	out, err := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid),
		"-o", "etime=,%cpu=,%mem=,rss=").Output()
	if err != nil {
		return Detail{}, fmt.Errorf("ps -p %d: %w", pid, err)
	}
	return parsePSDetail(string(out))
}

// parsePSDetail parses one `ps -o etime=,%cpu=,%mem=,rss=` output line, e.g.
//
//	"  1-02:03:04  0.3  1.2  54321"
//
// An unparseable etime degrades to zero (uptime unknown) rather than failing
// the whole inspection: hosts with boot-time clock skew report nonsense
// etimes, and the other fields are still good.
func parsePSDetail(out string) (Detail, error) {
	fields := strings.Fields(out)
	if len(fields) != 4 {
		return Detail{}, fmt.Errorf("unexpected ps output %q", strings.TrimSpace(out))
	}
	uptime, err := parseEtime(fields[0])
	if err != nil {
		uptime = 0
	}
	cpu, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return Detail{}, fmt.Errorf("parse %%cpu %q: %w", fields[1], err)
	}
	mem, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return Detail{}, fmt.Errorf("parse %%mem %q: %w", fields[2], err)
	}
	rss, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil {
		return Detail{}, fmt.Errorf("parse rss %q: %w", fields[3], err)
	}
	return Detail{Uptime: uptime, CPUPercent: cpu, MemPercent: mem, RSSKB: rss}, nil
}

// maxEtimeDays rejects nonsense process ages (100 years). Some virtualized
// hosts report etime values centuries long when the boot-time clock is
// skewed; naively converting those overflows time.Duration into negatives.
const maxEtimeDays = 36500

// parseEtime converts a ps etime value to a Duration. The format is
// [[dd-]hh:]mm:ss — e.g. "05:03", "1:02:03", or "12-01:02:03".
func parseEtime(s string) (time.Duration, error) {
	var days int64
	rest := s
	if i := strings.IndexByte(s, '-'); i >= 0 {
		d, err := strconv.ParseInt(s[:i], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse etime %q: %w", s, err)
		}
		days = d
		rest = s[i+1:]
	}
	if days < 0 || days > maxEtimeDays {
		return 0, fmt.Errorf("parse etime %q: implausible day count", s)
	}

	parts := strings.Split(rest, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, fmt.Errorf("parse etime %q: unexpected format", s)
	}
	// Each part is a base-60 digit group: mm:ss, or hh:mm:ss when 3 parts.
	// Minutes and seconds must be < 60; hours are bounded by the day guard.
	var total int64
	for i, p := range parts {
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse etime %q: %w", s, err)
		}
		if n < 0 || (i > 0 && n > 59) || (i == 0 && len(parts) == 2 && n > 59) || n > 9999 {
			return 0, fmt.Errorf("parse etime %q: field out of range", s)
		}
		total = total*60 + n
	}
	return time.Duration(days)*24*time.Hour + time.Duration(total)*time.Second, nil
}

// ProbeHTTP issues a GET to http://127.0.0.1:port/ and reports whether the
// port speaks HTTP, how fast it answered, and what it said. Redirects are not
// followed: a 301 from the server is itself a valid answer.
func ProbeHTTP(ctx context.Context, port int, timeout time.Duration) HTTPProbe {
	if timeout <= 0 {
		timeout = DefaultProbeTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	url := fmt.Sprintf("http://127.0.0.1:%d/", port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return HTTPProbe{Err: err.Error()}
	}

	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return HTTPProbe{Latency: latency, Err: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()

	return HTTPProbe{
		OK:      true,
		Status:  resp.StatusCode,
		Latency: latency,
		Server:  resp.Header.Get("Server"),
	}
}
