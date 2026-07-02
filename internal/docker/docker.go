// Package docker resolves which Docker container publishes a given host port.
// Docker-published ports otherwise show up as an opaque proxy process
// (com.docker.backend on macOS, docker-proxy on Linux), which tells the user
// nothing — and killing that proxy PID would take down the whole Docker
// daemon, not the one container. This package lets portview show the real
// container and stop it safely instead.
package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/jeramiahgcoffey/portview/internal/scanner"
)

// Container identifies a running container that publishes one or more host ports.
type Container struct {
	ID    string
	Name  string
	Image string
}

// psRow is one line of `docker ps --format '{{json .}}'` output. Docker emits
// one JSON object per line, not a JSON array.
type psRow struct {
	ID     string `json:"ID"`
	Names  string `json:"Names"`
	Image  string `json:"Image"`
	Ports  string `json:"Ports"`
	State  string `json:"State"`
	Status string `json:"Status"`
}

// looksDockerOwned reports whether a scanned server is a container runtime's
// port proxy rather than a directly-owned process: Docker Desktop
// (com.docker.backend), Linux dockerd (docker-proxy), or OrbStack
// (OrbStack Helper). All of them answer `docker ps`.
func looksDockerOwned(s scanner.Server) bool {
	p := strings.ToLower(s.Process)
	return strings.Contains(p, "docker") || strings.Contains(p, "orbstack")
}

// Enrich maps docker-proxy servers to their containers. If none of the
// servers look Docker-owned, or the docker CLI is unavailable or fails, the
// input is returned unchanged — enrichment is strictly best-effort.
func Enrich(ctx context.Context, servers []scanner.Server) []scanner.Server {
	hasDocker := false
	for _, s := range servers {
		if looksDockerOwned(s) {
			hasDocker = true
			break
		}
	}
	if !hasDocker {
		return servers
	}

	byPort, err := PortMap(ctx)
	if err != nil || len(byPort) == 0 {
		return servers
	}

	out := make([]scanner.Server, len(servers))
	copy(out, servers)
	for i, s := range out {
		if !looksDockerOwned(s) {
			continue
		}
		if c, ok := byPort[s.Port]; ok {
			out[i].Container = c.Name
			out[i].Image = c.Image
		}
	}
	return out
}

// PortMap returns host port → container for all running containers.
func PortMap(ctx context.Context) (map[int]Container, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, err
	}
	out, err := exec.CommandContext(ctx, "docker", "ps", "--format", "{{json .}}").Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps: %w", err)
	}
	return parsePortMap(out), nil
}

// parsePortMap builds the port→container map from `docker ps` JSON-lines output.
func parsePortMap(out []byte) map[int]Container {
	m := make(map[int]Container)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var row psRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			continue
		}
		c := Container{ID: row.ID, Name: row.Names, Image: row.Image}
		for _, port := range parseHostPorts(row.Ports) {
			if _, exists := m[port]; !exists {
				m[port] = c
			}
		}
	}
	return m
}

// parseHostPorts extracts published host ports from a docker ps Ports value,
// e.g. "0.0.0.0:5432->5432/tcp, :::5432->5432/tcp, 6379/tcp". Entries without
// "->" are exposed-but-unpublished and are skipped.
func parseHostPorts(s string) []int {
	var ports []int
	seen := make(map[int]struct{})
	for _, entry := range strings.Split(s, ",") {
		entry = strings.TrimSpace(entry)
		host, _, found := strings.Cut(entry, "->")
		if !found {
			continue
		}
		i := strings.LastIndex(host, ":")
		if i < 0 {
			continue
		}
		port, err := strconv.Atoi(host[i+1:])
		if err != nil {
			continue
		}
		if _, dup := seen[port]; dup {
			continue
		}
		seen[port] = struct{}{}
		ports = append(ports, port)
	}
	return ports
}

// Stop stops a container by name or ID via `docker stop`.
func Stop(ctx context.Context, nameOrID string) error {
	if nameOrID == "" {
		return fmt.Errorf("docker: empty container name")
	}
	if out, err := exec.CommandContext(ctx, "docker", "stop", nameOrID).CombinedOutput(); err != nil {
		return fmt.Errorf("docker stop %s: %s", nameOrID, strings.TrimSpace(string(out)))
	}
	return nil
}
