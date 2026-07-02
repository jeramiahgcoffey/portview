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
	return cmd.Start()
}

// Kill sends a graceful termination signal (SIGTERM) to pid.
func Kill(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(syscall.SIGTERM)
}
