package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// PIDFile returns the PID file path.
func PIDFile(dataDir string) string {
	return filepath.Join(dataDir, "pid")
}

// WritePID writes the current process PID.
func WritePID(dataDir string) error {
	return os.WriteFile(PIDFile(dataDir), []byte(strconv.Itoa(os.Getpid())), 0644)
}

// RemovePID removes the PID file.
func RemovePID(dataDir string) {
	os.Remove(PIDFile(dataDir))
}

// ReadPID reads the PID from file. Returns 0 if not found.
func ReadPID(dataDir string) int {
	data, err := os.ReadFile(PIDFile(dataDir))
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(string(data))
	return pid
}

// IsRunning checks if the daemon process is alive.
func IsRunning(dataDir string) (int, bool) {
	pid := ReadPID(dataDir)
	if pid == 0 {
		return 0, false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return 0, false
	}
	// Signal 0 checks if process exists without actually sending a signal
	err = process.Signal(syscall.Signal(0))
	return pid, err == nil
}

// Stop sends SIGTERM to the daemon and waits for it to exit.
func Stop(dataDir string) error {
	pid, running := IsRunning(dataDir)
	if !running {
		RemovePID(dataDir)
		return fmt.Errorf("daemon is not running")
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("signal: %w", err)
	}

	// Wait for process to actually exit (up to 5s)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := process.Signal(syscall.Signal(0)); err != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	RemovePID(dataDir)
	return nil
}
