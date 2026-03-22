package cli

import (
	"fmt"
	"io/fs"
	"net/http"
	"os/exec"
	"runtime"

	"github.com/lanesket/llm.log/internal/storage"
	"github.com/lanesket/llm.log/internal/ui"
	"github.com/lanesket/llm.log/web"
	"github.com/spf13/cobra"
)

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Open web dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")
		devMode, _ := cmd.Flags().GetBool("dev")

		dataDir := DataDir()

		store, err := storage.Open(dataDir)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer store.Close()

		var webFS fs.FS
		if !devMode {
			sub, err := fs.Sub(web.DistFS, "dist")
			if err != nil {
				return fmt.Errorf("embed fs: %w", err)
			}
			webFS = sub
		}

		srv := ui.New(store, dataDir, webFS, devMode)

		addr := fmt.Sprintf("127.0.0.1:%d", port)
		url := fmt.Sprintf("http://%s", addr)
		fmt.Printf("llm.log UI running at %s\n", url)

		if !devMode {
			openBrowser(url)
		}

		return http.ListenAndServe(addr, srv)
	},
}

func init() {
	uiCmd.Flags().Int("port", 9923, "port for the web UI server")
	uiCmd.Flags().Bool("dev", false, "development mode (API only, no static files)")
	rootCmd.AddCommand(uiCmd)
}

func openBrowser(url string) {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", url).Start()
	case "linux":
		exec.Command("xdg-open", url).Start()
	}
}
