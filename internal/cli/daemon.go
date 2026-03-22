package cli

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/lanesket/llm.log/internal/daemon"
	"github.com/lanesket/llm.log/internal/format"
	"github.com/lanesket/llm.log/internal/pricing"
	"github.com/lanesket/llm.log/internal/proxy"
	"github.com/lanesket/llm.log/internal/storage"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(runCmd)
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the proxy daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := DataDir()

		if pid, running := daemon.IsRunning(dir); running {
			fmt.Printf("Already running (PID %d) on %s\n", pid, proxyAddr)
			return nil
		}

		// Clean up stale proxy state from a previous crashed daemon
		clearEnvFile()
		deactivateSystemProxy()

		pid, err := daemon.StartDaemon(proxyAddr, daemonSysProcAttr())
		if err != nil {
			return err
		}

		// Refresh CA bundle (system CAs may have changed since setup)
		createCABundle(dir)

		if err := writeEnvFile(); err != nil {
			return fmt.Errorf("write env file: %w", err)
		}
		activateSystemProxy()

		fmt.Printf("Started llm-log daemon (PID %d) on %s\n", pid, proxyAddr)
		fmt.Println("HTTPS_PROXY is now active for new terminals and apps")
		return nil
	},
}

func hasStaleProxyState() bool {
	_, err := os.Stat(envFile())
	return err == nil
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the proxy daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := DataDir()

		// Always clean up proxy state, even if daemon is already dead
		defer clearEnvFile()
		defer deactivateSystemProxy()

		if err := daemon.Stop(dir); err != nil {
			fmt.Printf("Note: %v\n", err)
			fmt.Println("Cleaned up proxy environment")
			return nil
		}

		fmt.Println("Daemon stopped, proxy environment cleared")
		fmt.Println("Note: already-running apps (Cursor, terminals) may need a restart")
		return nil
	},
}

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the proxy daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := DataDir()
		if pid, running := daemon.IsRunning(dir); running {
			fmt.Printf("Stopping daemon (PID %d)...\n", pid)
			if err := daemon.Stop(dir); err != nil {
				return err
			}
			clearEnvFile()
			deactivateSystemProxy()
		}
		return startCmd.RunE(cmd, args)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := DataDir()
		pid, running := daemon.IsRunning(dir)
		if !running {
			fmt.Println("● Daemon is not running")
			if hasStaleProxyState() {
				clearEnvFile()
				deactivateSystemProxy()
				fmt.Println("  Cleaned up stale proxy environment")
			}
			return nil
		}
		fmt.Printf("● Daemon running (PID %d) on %s\n", pid, proxyAddr)

		// Show today's summary
		store, err := storage.Open(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warning: cannot read stats: %v\n", err)
			return nil
		}
		defer store.Close()

		from, to := storage.PeriodToTimeRange("today")
		stats, err := store.Stats(storage.StatsFilter{From: from, To: to, GroupBy: "provider"})
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warning: cannot read stats: %v\n", err)
			return nil
		}

		var totalReqs int
		var totalTokens int64
		var totalCost float64
		for _, s := range stats {
			totalReqs += s.Requests
			totalTokens += s.InputTokens + s.OutputTokens
			totalCost += s.TotalCost
		}

		fmt.Printf("  Today: %d requests · %s tokens · $%.2f\n", totalReqs, format.Tokens(totalTokens), totalCost)
		return nil
	},
}

// runCmd is the internal command that runs the proxy in foreground (used by start).
var runCmd = &cobra.Command{
	Use:    "run",
	Short:  "Run proxy in foreground (internal)",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := DataDir()

		// Prevent the daemon from proxying its own outgoing requests
		os.Unsetenv("HTTPS_PROXY")
		os.Unsetenv("HTTP_PROXY")
		os.Unsetenv("https_proxy")
		os.Unsetenv("http_proxy")

		// Setup logging
		logFile, err := os.OpenFile(
			filepath.Join(dir, "llm-log.log"),
			os.O_CREATE|os.O_APPEND|os.O_WRONLY,
			0644,
		)
		if err != nil {
			return err
		}
		defer logFile.Close()
		log.SetOutput(logFile)
		log.SetFlags(log.LstdFlags | log.Lshortfile)

		// Open storage
		store, err := storage.Open(dir)
		if err != nil {
			return fmt.Errorf("open storage: %w", err)
		}
		defer store.Close()

		// Load pricing from cache (instant), fetch update in background
		priceDB := pricing.NewDB(dir)
		go priceDB.UpdateIfStale()
		priceDB.StartAutoUpdate()

		// Create proxy
		p, err := proxy.New(proxyAddr, dir, store, priceDB)
		if err != nil {
			return fmt.Errorf("create proxy: %w", err)
		}

		// Write PID
		if err := daemon.WritePID(dir); err != nil {
			return fmt.Errorf("write pid: %w", err)
		}

		// Clean up all state on ANY exit (crash, SIGTERM, error)
		defer func() {
			daemon.RemovePID(dir)
			clearEnvFile()
			deactivateSystemProxy()
			log.Println("cleaned up proxy state")
		}()

		// Handle shutdown
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sig
			log.Println("shutting down...")
			if err := p.Shutdown(); err != nil {
				log.Printf("shutdown error: %v", err)
			}
		}()

		log.Println("llm-log daemon started")
		return p.ListenAndServe()
	},
}
