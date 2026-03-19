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
	rootCmd.AddCommand(setupCmd)
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Generate CA cert, add to trust store, configure shell",
	RunE:  runSetup,
}

func runSetup(cmd *cobra.Command, args []string) error {
	dir := DataDir()

	// 1. Generate CA cert
	fmt.Println("Generating CA certificate...")
	_, err := proxy.LoadOrGenerateCA(dir)
	if err != nil {
		return fmt.Errorf("CA generation failed: %w", err)
	}
	certPath := proxy.CertPath(dir)
	fmt.Printf("  ✓ %s\n\n", certPath)

	// 2. Add to system trust store
	fmt.Println("Adding to system trust store (requires sudo)...")
	if err := installCert(certPath); err != nil {
		fmt.Printf("  ✗ %v\n", err)
		fmt.Println("  You can add it manually later.")
	} else {
		fmt.Println("  ✓ Certificate trusted")
	}
	fmt.Println()

	// 3. Create merged CA bundle (system CAs + ours) for Python, Homebrew curl, etc.
	fmt.Println("Creating CA bundle...")
	if err := createCABundle(dir); err != nil {
		fmt.Printf("  ✗ %v\n", err)
		fmt.Println("  Python/Homebrew curl may need manual CA configuration.")
	} else {
		fmt.Println("  ✓ " + caBundlePath(dir))
	}
	fmt.Println()

	// 4. Configure shell to source env file
	fmt.Println("Configuring shell...")
	if err := configureShell(); err != nil {
		fmt.Printf("  ✗ %v\n", err)
	} else {
		fmt.Println("  ✓ Shell configured to source ~/.llm.log/env")
	}
	fmt.Println()

	// 5. Generate shell completions
	fmt.Println("Setting up shell completions...")
	if err := setupCompletions(dir); err != nil {
		fmt.Printf("  ✗ %v\n", err)
	} else {
		fmt.Println("  ✓ Shell completions installed")
	}
	fmt.Println()

	fmt.Println("Restart your shell or run: source ~/." + shellName() + "rc")
	fmt.Println("Then run: llm-log start")

	return nil
}

func setupCompletions(dataDir string) error {
	compDir := filepath.Join(dataDir, "completions")
	if err := os.MkdirAll(compDir, 0755); err != nil {
		return err
	}

	shell := shellName()
	switch shell {
	case "zsh":
		f, err := os.Create(filepath.Join(compDir, "_llm-log"))
		if err != nil {
			return err
		}
		defer f.Close()
		if err := rootCmd.GenZshCompletion(f); err != nil {
			return err
		}
		return addFpathToRC(compDir)
	case "bash":
		f, err := os.Create(filepath.Join(compDir, "llm-log.bash"))
		if err != nil {
			return err
		}
		defer f.Close()
		return rootCmd.GenBashCompletion(f)
	default:
		return fmt.Errorf("unsupported shell: %s", shell)
	}
}

func addFpathToRC(compDir string) error {
	rcFile := filepath.Join(os.Getenv("HOME"), ".zshrc")
	content, err := os.ReadFile(rcFile)
	if err != nil {
		return err
	}
	if strings.Contains(string(content), "llm.log/completions") {
		return nil
	}
	f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	fmt.Fprintf(f, "\n# llm.log completions\nfpath=(%s $fpath)\nautoload -Uz compinit && compinit\n", compDir)
	return nil
}

func installCert(certPath string) error {
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("sudo", "security", "add-trusted-cert",
			"-d", "-r", "trustRoot",
			"-k", "/Library/Keychains/System.keychain",
			certPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case "linux":
		// Copy cert and update CA store
		dest := "/usr/local/share/ca-certificates/llm-log.crt"
		cmd := exec.Command("sudo", "cp", certPath, dest)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
		cmd = exec.Command("sudo", "update-ca-certificates")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func configureShell() error {
	rcFile := filepath.Join(os.Getenv("HOME"), "."+shellName()+"rc")
	sourceLine := `source "$HOME/.llm.log/env" 2>/dev/null`

	content, err := os.ReadFile(rcFile)
	if err == nil && strings.Contains(string(content), "llm.log/env") {
		fmt.Println("  Already configured")
		return nil
	}

	f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "\n# llm.log proxy\n%s\n", sourceLine)
	return nil
}

func caBundlePath(dataDir string) string {
	return filepath.Join(dataDir, "ca-bundle.pem")
}

// createCABundle merges system CA certificates with our CA into a single PEM bundle.
// This allows Python, Homebrew curl, Ruby, and other OpenSSL-based tools to trust
// both regular HTTPS sites and our MITM proxy.
func createCABundle(dataDir string) error {
	var systemCAs []byte
	var err error

	switch runtime.GOOS {
	case "darwin":
		// Export all system root CAs + any user-added CAs
		roots, _ := exec.Command("security", "find-certificate", "-a", "-p",
			"/System/Library/Keychains/SystemRootCertificates.keychain").Output()
		user, _ := exec.Command("security", "find-certificate", "-a", "-p",
			"/Library/Keychains/System.keychain").Output()
		systemCAs = append(roots, user...)
	case "linux":
		// Try common CA bundle locations
		for _, path := range []string{
			"/etc/ssl/certs/ca-certificates.crt", // Debian/Ubuntu
			"/etc/pki/tls/certs/ca-bundle.crt",   // RHEL/Fedora
			"/etc/ssl/ca-bundle.pem",             // OpenSUSE
		} {
			if data, err := os.ReadFile(path); err == nil {
				systemCAs = data
				break
			}
		}
	}

	if len(systemCAs) == 0 {
		return fmt.Errorf("could not read system CA certificates")
	}

	ourCA, err := os.ReadFile(proxy.CertPath(dataDir))
	if err != nil {
		return fmt.Errorf("read CA cert: %w", err)
	}

	bundle := append(systemCAs, '\n')
	bundle = append(bundle, ourCA...)
	return os.WriteFile(caBundlePath(dataDir), bundle, 0644)
}

func shellName() string {
	shell := os.Getenv("SHELL")
	if strings.Contains(shell, "zsh") {
		return "zsh"
	}
	return "bash"
}
