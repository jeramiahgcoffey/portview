package scanner

import (
	"strconv"
	"strings"
)

// parseProcNetTCP parses the contents of /proc/net/tcp and returns
// a slice of Server for each entry in the LISTEN state (state == "0A").
func parseProcNetTCP(content string) []Server {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) <= 1 {
		return nil
	}

	var servers []Server

	// Skip the header line (index 0), process data lines.
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		// Field 3 (index 3) is the connection state.
		state := fields[3]
		if state != "0A" {
			continue
		}

		// Field 1 (index 1) is local_address in the form hex_ip:hex_port.
		localAddr := fields[1]
		parts := strings.Split(localAddr, ":")
		if len(parts) != 2 {
			continue
		}

		port, err := strconv.ParseInt(parts[1], 16, 64)
		if err != nil {
			continue
		}

		servers = append(servers, Server{
			Port:  int(port),
			State: "LISTEN",
		})
	}

	if len(servers) == 0 {
		return nil
	}

	return servers
}
