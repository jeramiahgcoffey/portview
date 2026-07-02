package action

import (
	"os/exec"
	"testing"
	"time"
)

func TestKillRejectsNonPositivePID(t *testing.T) {
	for _, pid := range []int{0, -1, -1000} {
		if err := Kill(pid); err == nil {
			t.Errorf("Kill(%d) = nil, want error (guards against process-group signal)", pid)
		}
	}
}

func TestKillSignalsRealProcess(t *testing.T) {
	// Spawn a child we own, SIGTERM it, and confirm it dies from the signal.
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}

	if err := Kill(cmd.Process.Pid); err != nil {
		t.Fatalf("Kill(child) = %v, want nil", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("child exited cleanly, expected termination by signal")
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("child not terminated within 5s of SIGTERM")
	}
}
