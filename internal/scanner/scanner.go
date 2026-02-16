package scanner

import "context"

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
