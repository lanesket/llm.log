package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/lanesket/llm.log/internal/proxy"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(envCmd)
}

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Print proxy env vars (use with: eval $(llm-log env))",
	Run: func(cmd *cobra.Command, args []string) {
		for _, kv := range proxyEnvVars() {
			fmt.Printf("export %s=%s\n", kv.k, kv.v)
		}
	},
}

type envVar struct{ k, v string }

func envFile() string {
	return filepath.Join(DataDir(), "env")
}

func envFileContent() string {
	vars := proxyEnvVars()
	var b strings.Builder
	for _, kv := range vars {
		fmt.Fprintf(&b, "export %s=%s\n", kv.k, kv.v)
	}
	return b.String()
}

func writeEnvFile() error {
	return os.WriteFile(envFile(), []byte(envFileContent()), 0644)
}

func clearEnvFile() {
	os.Remove(envFile())
}

// proxyEnvVars returns all env vars needed for the proxy to work.
// Single source of truth — used by env command, env file, and system proxy activation.
func proxyEnvVars() []envVar {
	dir := DataDir()
	proxyURL := "http://" + proxyAddr
	certPath := proxy.CertPath(dir)
	bundlePath := caBundlePath(dir)

	vars := []envVar{
		{"HTTPS_PROXY", proxyURL},
		{"https_proxy", proxyURL},        // POSIX lowercase (curl, wget, some Ruby/Python)
		{"NODE_EXTRA_CA_CERTS", certPath}, // Node.js (additive — does not replace system CAs)
	}

	if _, err := os.Stat(bundlePath); err == nil {
		vars = append(vars,
			envVar{"SSL_CERT_FILE", bundlePath},      // OpenSSL-based: Go, Ruby, httpx
			envVar{"REQUESTS_CA_BUNDLE", bundlePath},  // Python requests, OpenAI/Anthropic SDK
			envVar{"CURL_CA_BUNDLE", bundlePath},      // curl (checked before SSL_CERT_FILE)
		)
	}
	return vars
}

// activateSystemProxy sets env vars for GUI apps (Cursor, VS Code launched from Dock/Activities).
//   - macOS: launchctl setenv (picked up by apps launched via launchd)
//   - Linux: systemctl --user set-environment (picked up by apps in systemd user session)
func activateSystemProxy() {
	vars := proxyEnvVars()
	switch runtime.GOOS {
	case "darwin":
		for _, kv := range vars {
			exec.Command("launchctl", "setenv", kv.k, kv.v).Run()
		}
	case "linux":
		if _, err := exec.LookPath("systemctl"); err == nil {
			args := make([]string, 0, len(vars)+2)
			args = append(args, "--user", "set-environment")
			for _, kv := range vars {
				args = append(args, kv.k+"="+kv.v)
			}
			exec.Command("systemctl", args...).Run()
		}
	}
}

func deactivateSystemProxy() {
	vars := proxyEnvVars()
	keys := make([]string, len(vars))
	for i, kv := range vars {
		keys[i] = kv.k
	}

	switch runtime.GOOS {
	case "darwin":
		for _, k := range keys {
			exec.Command("launchctl", "unsetenv", k).Run()
		}
	case "linux":
		if _, err := exec.LookPath("systemctl"); err == nil {
			args := append([]string{"--user", "unset-environment"}, keys...)
			exec.Command("systemctl", args...).Run()
		}
	}
}
