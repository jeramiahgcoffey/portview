// Package action holds the side-effecting operations shared by the TUI and
// the CLI subcommands: opening a port in the browser and terminating the
// process that owns it.
package action

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
)

// OpenBrowser launches the platform's default opener for localhost:port.
func OpenBrowser(port int) error {
	url := fmt.Sprintf("http://localhost:%d", port)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	// The opener exits immediately after handing the URL to the desktop.
	// Reap it in the background so it doesn't linger as a zombie for the
	// lifetime of a long-running TUI session; its exit status is irrelevant.
	go func() { _ = cmd.Wait() }()
	return nil
}

// Kill sends a graceful termination signal (SIGTERM) to pid. A non-positive
// pid is rejected: on Unix, signaling pid 0 hits the caller's whole process
// group and a negative pid targets a process-group id — far wider than the
// single process a caller means to stop.
func Kill(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("refusing to signal non-positive pid %d", pid)
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(syscall.SIGTERM)
}
