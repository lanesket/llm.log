package daemon

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// StartDaemon forks a new process running "llm-log run" and waits for it to
// start listening on the given address. Returns the PID of the new process.
func StartDaemon(addr string, procAttr *syscall.SysProcAttr) (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("find executable: %w", err)
	}
	proc := exec.Command(exe, "run")
	proc.Stdout = nil
	proc.Stderr = nil
	proc.SysProcAttr = procAttr

	if err := proc.Start(); err != nil {
		return 0, fmt.Errorf("start daemon: %w", err)
	}

	// Wait for the proxy to actually start listening
	if !waitForPort(addr, 5*time.Second) {
		_ = proc.Process.Kill()
		return 0, fmt.Errorf("daemon failed to start — check ~/.llm.log/llm-log.log")
	}

	return proc.Process.Pid, nil
}

func waitForPort(addr string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}
