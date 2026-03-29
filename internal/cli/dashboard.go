package cli

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lanesket/llm.log/internal/dashboard"
	"github.com/lanesket/llm.log/internal/storage"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(dashboardCmd)
}

var dashboardCmd = &cobra.Command{
	Use:     "dashboard",
	Aliases: []string{"dash"},
	Short:   "Interactive TUI dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := DataDir()
		store, err := storage.Open(dir)
		if err != nil {
			return err
		}
		defer store.Close()

		p := tea.NewProgram(
			dashboard.New(store, dir),
			tea.WithAltScreen(),
			tea.WithMouseCellMotion(),
		)
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("dashboard: %w", err)
		}
		return nil
	},
}
