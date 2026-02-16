package scanner

import (
	"context"
	"errors"
	"testing"
)

func TestMockScanner_ReturnsCannedData(t *testing.T) {
	servers := []Server{
		{Port: 8080, PID: 1234, Process: "node", Command: "node server.js", State: "LISTEN"},
		{Port: 3000, PID: 5678, Process: "python", Command: "python app.py", State: "LISTEN"},
	}

	var s Scanner = &MockScanner{Servers: servers}
	result, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(result))
	}
	if result[0].Port != 8080 {
		t.Errorf("result[0].Port = %d, want 8080", result[0].Port)
	}
	if result[1].Process != "python" {
		t.Errorf("result[1].Process = %q, want %q", result[1].Process, "python")
	}
}

func TestMockScanner_ReturnsError(t *testing.T) {
	expected := errors.New("scan failed")
	var s Scanner = &MockScanner{Err: expected}

	_, err := s.Scan(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != expected {
		t.Errorf("error = %v, want %v", err, expected)
	}
}
