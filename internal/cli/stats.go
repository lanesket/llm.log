package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/lanesket/llm.log/internal/format"
	"github.com/lanesket/llm.log/internal/storage"
	"github.com/spf13/cobra"
)

func init() {
	statsCmd.Flags().StringP("by", "b", "provider", "Group by: provider, model, day")
	statsCmd.Flags().StringP("period", "p", "month", "Period: today, week, month, all")
	statsCmd.Flags().StringP("source", "s", "", "Filter by source: cc:, cc:sub, cc:key")
	statsCmd.Flags().String("from", "", "Start date (YYYY-MM-DD)")
	statsCmd.Flags().String("to", "", "End date (YYYY-MM-DD)")
	statsCmd.Flags().Bool("json", false, "Output as JSON")
	rootCmd.AddCommand(statsCmd)
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show usage statistics",
	RunE:  runStats,
}

func runStats(cmd *cobra.Command, args []string) error {
	dir := DataDir()
	store, err := storage.Open(dir)
	if err != nil {
		return err
	}
	defer store.Close()

	groupBy, _ := cmd.Flags().GetString("by")
	period, _ := cmd.Flags().GetString("period")
	source, _ := cmd.Flags().GetString("source")
	fromStr, _ := cmd.Flags().GetString("from")
	toStr, _ := cmd.Flags().GetString("to")
	asJSON, _ := cmd.Flags().GetBool("json")

	from, to, err := parseDateRange(fromStr, toStr, period)
	if err != nil {
		return err
	}

	stats, err := store.Stats(storage.StatsFilter{
		From:    from,
		To:      to,
		GroupBy: groupBy,
		Source:  source,
	})
	if err != nil {
		return err
	}

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(stats)
	}

	if len(stats) == 0 {
		fmt.Println("No data for this period.")
		return nil
	}

	printStatsTable(stats, groupBy, period)
	return nil
}

func printStatsTable(stats []storage.StatRow, groupBy, period string) {
	title := strings.ToUpper(groupBy[:1]) + groupBy[1:]
	fmt.Printf("\n  llm.log — %s (%s)\n\n", title, period)

	maxKey := len(title)
	for _, s := range stats {
		if len(s.Key) > maxKey {
			maxKey = len(s.Key)
		}
	}
	if maxKey > 30 {
		maxKey = 30
	}

	// Table header
	fmt.Printf("  %-*s  %6s  %10s  %10s  %10s  %10s  %10s  %8s  %6s\n",
		maxKey, title, "Reqs", "In", "Out", "Cache↓", "Cache↑", "Cost", "Avg ms", "Errors")
	sep := func(w int) string { return strings.Repeat("─", w) }
	fmt.Printf("  %s  %s  %s  %s  %s  %s  %s  %s  %s\n",
		sep(maxKey), sep(6), sep(10), sep(10), sep(10), sep(10), sep(10), sep(8), sep(6))

	var totalReqs, totalErrors int
	var totalIn, totalOut, totalCacheRead, totalCacheWrite int64
	var totalCost float64
	var totalDuration int

	for _, s := range stats {
		key := format.Truncate(s.Key, maxKey)

		errStr := ""
		if s.Errors > 0 {
			errStr = fmt.Sprintf("%d", s.Errors)
		}

		fmt.Printf("  %-*s  %6d  %10s  %10s  %10s  %10s  %10s  %8d  %6s\n",
			maxKey, key,
			s.Requests,
			format.Tokens(s.InputTokens),
			format.Tokens(s.OutputTokens),
			format.Tokens(s.CacheReadTokens),
			format.Tokens(s.CacheWriteTokens),
			format.Cost(s.TotalCost),
			s.AvgDurationMs,
			errStr,
		)

		totalReqs += s.Requests
		totalIn += s.InputTokens
		totalOut += s.OutputTokens
		totalCacheRead += s.CacheReadTokens
		totalCacheWrite += s.CacheWriteTokens
		totalCost += s.TotalCost
		totalErrors += s.Errors
		totalDuration += s.AvgDurationMs * s.Requests

		// Bar chart
		if s.TotalCost > 0 {
			maxCost := stats[0].TotalCost
			barLen := int(s.TotalCost / maxCost * 30)
			if barLen < 1 {
				barLen = 1
			}
			fmt.Printf("  %-*s  %s\n", maxKey, "", strings.Repeat("█", barLen))
		}
	}

	// Totals
	fmt.Printf("  %s  %s  %s  %s  %s  %s  %s  %s  %s\n",
		sep(maxKey), sep(6), sep(10), sep(10), sep(10), sep(10), sep(10), sep(8), sep(6))

	avgMs := 0
	if totalReqs > 0 {
		avgMs = totalDuration / totalReqs
	}
	totalErrStr := ""
	if totalErrors > 0 {
		totalErrStr = fmt.Sprintf("%d", totalErrors)
	}

	fmt.Printf("  %-*s  %6d  %10s  %10s  %10s  %10s  %10s  %8d  %6s\n\n",
		maxKey, "Total",
		totalReqs,
		format.Tokens(totalIn),
		format.Tokens(totalOut),
		format.Tokens(totalCacheRead),
		format.Tokens(totalCacheWrite),
		format.Cost(totalCost),
		avgMs,
		totalErrStr,
	)
}
