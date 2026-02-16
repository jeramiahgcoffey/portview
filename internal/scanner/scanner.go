package scanner

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// Server represents a TCP server listening on localhost.
type Server struct {
	Port    int
	PID     int
	Process string
	Command string
	State   string
	Label   string
	Healthy bool
}

// Scanner discovers TCP servers listening on localhost.
type Scanner interface {
	Scan(ctx context.Context) ([]Server, error)
}

// MockScanner is a test double that returns pre-configured data.
type MockScanner struct {
	Servers []Server
	Err     error
}

// Scan returns the pre-configured servers and error.
func (m *MockScanner) Scan(_ context.Context) ([]Server, error) {
	return m.Servers, m.Err
}

// CheckHealth probes each server's port via TCP and sets the Healthy flag.
// It returns a new slice, leaving the original unmodified.
func CheckHealth(servers []Server, timeout time.Duration) []Server {
	// Create a copy of the slice to avoid mutating the original
	result := make([]Server, len(servers))
	copy(result, servers)

	var wg sync.WaitGroup
	for i := range result {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", result[idx].Port), timeout)
			if err == nil {
				result[idx].Healthy = true
				conn.Close()
			}
		}(i)
	}
	wg.Wait()
	return result
}
