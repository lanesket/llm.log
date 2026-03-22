package ui

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/lanesket/llm.log/internal/daemon"
)

// proxyPort is the default proxy listen port. Must match cli.proxyAddr.
const proxyPort = 9922

// ProxyAddr is the default proxy listen address, derived from proxyPort.
var ProxyAddr = fmt.Sprintf("127.0.0.1:%d", proxyPort)

type statusResponse struct {
	ProxyRunning  bool   `json:"proxy_running"`
	PID           *int   `json:"pid"`
	UptimeSeconds *int64 `json:"uptime_seconds"`
	Port          *int   `json:"port"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	pid, running := daemon.IsRunning(s.dataDir)
	if !running {
		writeJSON(w, http.StatusOK, statusResponse{ProxyRunning: false})
		return
	}

	resp := statusResponse{
		ProxyRunning: true,
		PID:          &pid,
	}

	port := proxyPort
	resp.Port = &port

	// Determine uptime from PID file modification time
	pidFile := daemon.PIDFile(s.dataDir)
	if info, err := os.Stat(pidFile); err == nil {
		uptime := int64(time.Since(info.ModTime()).Seconds())
		resp.UptimeSeconds = &uptime
	}

	writeJSON(w, http.StatusOK, resp)
}
