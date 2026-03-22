package ui

import (
	"fmt"
	"net/http"
	"syscall"

	"github.com/lanesket/llm.log/internal/daemon"
)

func (s *Server) handleProxyStart(w http.ResponseWriter, r *http.Request) {
	pid, running := daemon.IsRunning(s.dataDir)
	if running {
		writeError(w, http.StatusConflict, fmt.Sprintf("Proxy is already running (PID %d)", pid))
		return
	}

	newPID, err := daemon.StartDaemon(ProxyAddr, &syscall.SysProcAttr{Setsid: true})
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to start proxy: %v", err))
		return
	}

	port := proxyPort
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"pid":  newPID,
		"port": port,
	})
}

func (s *Server) handleProxyStop(w http.ResponseWriter, r *http.Request) {
	_, running := daemon.IsRunning(s.dataDir)
	if !running {
		writeError(w, http.StatusConflict, "Proxy is not running")
		return
	}

	if err := daemon.Stop(s.dataDir); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to stop proxy: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
