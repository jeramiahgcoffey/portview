package scanner

import (
	"strconv"
	"strings"
)

// parseLsofOutput parses the output of `lsof -iTCP -sTCP:LISTEN -nP`
// and returns a slice of Server for each LISTEN entry found.
func parseLsofOutput(output string) []Server {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return nil
	}

	lines := strings.Split(trimmed, "\n")
	if len(lines) <= 1 {
		return nil
	}

	var servers []Server

	// Skip the header line (index 0), process data lines.
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		// A valid lsof LISTEN line has at least 10 fields:
		// COMMAND PID USER FD TYPE DEVICE SIZE/OFF NODE NAME (LISTEN)
		if len(fields) < 10 {
			continue
		}

		// Field 0: COMMAND (process name)
		command := fields[0]

		// Field 1: PID
		pid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}

		// The last field should be "(LISTEN)".
		if fields[len(fields)-1] != "(LISTEN)" {
			continue
		}

		// The NAME field is the one just before "(LISTEN)" and contains host:port.
		nameField := fields[len(fields)-2]

		// Extract port from host:port format (e.g., "*:8080" or "127.0.0.1:3000").
		lastColon := strings.LastIndex(nameField, ":")
		if lastColon < 0 {
			continue
		}

		portStr := nameField[lastColon+1:]
		port, err := strconv.Atoi(portStr)
		if err != nil {
			continue
		}

		servers = append(servers, Server{
			Port:    port,
			PID:     pid,
			Process: command,
			State:   "LISTEN",
		})
	}

	if len(servers) == 0 {
		return nil
	}

	return servers
}
