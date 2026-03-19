package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lanesket/llm.log/internal/storage"
	"github.com/spf13/cobra"
)

func init() {
	pruneCmd.Flags().String("older-than", "", "Delete bodies older than duration (e.g. 7d, 30d, 6m, 1y)")
	pruneCmd.Flags().Bool("dry-run", false, "Show what would be deleted without deleting")
	pruneCmd.Flags().Bool("force", false, "Skip confirmation prompt")
	pruneCmd.MarkFlagRequired("older-than")
	rootCmd.AddCommand(pruneCmd)
}

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Delete old request/response bodies (keeps metadata)",
	RunE:  runPrune,
}

func runPrune(cmd *cobra.Command, args []string) error {
	dir := DataDir()
	store, err := storage.Open(dir)
	if err != nil {
		return err
	}
	defer store.Close()

	olderThan, _ := cmd.Flags().GetString("older-than")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	force, _ := cmd.Flags().GetBool("force")

	dur, err := parseDuration(olderThan)
	if err != nil {
		return err
	}

	before := time.Now().Add(-dur)

	preview, err := store.PrunePreview(before)
	if err != nil {
		return err
	}

	if preview.Count == 0 {
		fmt.Println("No bodies to prune.")
		return nil
	}

	msg := fmt.Sprintf("Found %d request bodies older than %s", preview.Count, olderThan)
	if preview.Bytes > 0 {
		msg += fmt.Sprintf(" (%s on disk)", formatBytes(preview.Bytes))
	}
	fmt.Println(msg + ".")

	if dryRun {
		return nil
	}

	if !force {
		fmt.Print("Delete? [y/N]: ")
		var answer string
		fmt.Scanln(&answer)
		if strings.ToLower(strings.TrimSpace(answer)) != "y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	pruned, err := store.PruneBodies(before)
	if err != nil {
		return err
	}

	fmt.Printf("Pruned %d bodies. Running VACUUM...\n", pruned)
	if err := store.Vacuum(); err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}
	fmt.Println("Done.")
	return nil
}

// parseDuration parses human durations like "7d", "30d", "6m", "1y".
func parseDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration %q (use e.g. 7d, 30d, 6m, 1y)", s)
	}
	unit := s[len(s)-1]
	num, err := strconv.Atoi(s[:len(s)-1])
	if err != nil || num <= 0 {
		return 0, fmt.Errorf("invalid duration %q (use e.g. 7d, 30d, 6m, 1y)", s)
	}
	switch unit {
	case 'd':
		return time.Duration(num) * 24 * time.Hour, nil
	case 'm':
		return time.Duration(num) * 30 * 24 * time.Hour, nil
	case 'y':
		return time.Duration(num) * 365 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown unit %q in %q (use d, m, or y)", string(unit), s)
	}
}

func formatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
