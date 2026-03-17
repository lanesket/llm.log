package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/lanesket/llm.log/internal/format"
	"github.com/lanesket/llm.log/internal/storage"
	"github.com/spf13/cobra"
)

func init() {
	logsCmd.Flags().IntP("last", "n", 20, "Number of recent requests")
	logsCmd.Flags().Bool("full", false, "Show full prompts/responses")
	logsCmd.Flags().Int64("id", 0, "Show details for a specific request ID")
	logsCmd.Flags().StringP("source", "s", "", "Filter by source: cc:, cc:sub, cc:key")
	rootCmd.AddCommand(logsCmd)
}

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show recent requests",
	RunE:  runLogs,
}

func runLogs(cmd *cobra.Command, args []string) error {
	dir := DataDir()
	store, err := storage.Open(dir)
	if err != nil {
		return err
	}
	defer store.Close()

	id, _ := cmd.Flags().GetInt64("id")
	if id > 0 {
		return showRequest(store, id)
	}

	n, _ := cmd.Flags().GetInt("last")
	full, _ := cmd.Flags().GetBool("full")
	source, _ := cmd.Flags().GetString("source")

	records, err := store.Recent(n, time.Time{}, time.Time{}, "", source)
	if err != nil {
		return err
	}

	if len(records) == 0 {
		fmt.Println("No requests recorded yet.")
		return nil
	}

	fmt.Println()
	for _, r := range records {
		ts := r.Timestamp.Local().Format("15:04:05")
		costStr := "—"
		if r.TotalCost != nil {
			costStr = format.Cost(*r.TotalCost)
		}
		stream := ""
		if r.Streaming {
			stream = " stream"
		}
		src := ""
		if r.Source != "" {
			src = " [" + r.Source + "]"
		}
		errMark := ""
		if r.StatusCode >= 400 {
			errMark = fmt.Sprintf(" err:%d", r.StatusCode)
		}

		fmt.Printf("  #%-5d %s  %-10s %-25s %6d in / %6d out  %8s  %4dms%s%s%s\n",
			r.ID, ts, r.Provider, format.Truncate(r.Model, 25),
			r.InputTokens, r.OutputTokens, costStr, r.DurationMs, stream, src, errMark)

		if full {
			rec, err := store.Get(r.ID)
			if err == nil {
				printBody("  Request", rec.RequestBody)
				printBody("  Response", rec.ResponseBody)
				fmt.Println()
			}
		}
	}
	fmt.Println()
	return nil
}

func showRequest(store storage.Store, id int64) error {
	rec, err := store.Get(id)
	if err != nil {
		return fmt.Errorf("request #%d: %w", id, err)
	}

	costStr := "—"
	if rec.TotalCost != nil {
		costStr = format.Cost(*rec.TotalCost)
	}

	src := ""
	if rec.Source != "" {
		src = " · source: " + rec.Source
	}

	fmt.Printf("\n  Request #%d\n", rec.ID)
	fmt.Printf("  %s · %s · %s%s\n", rec.Timestamp.Local().Format("2006-01-02 15:04:05"), rec.Provider, rec.Model, src)
	fmt.Printf("  In: %d · Out: %d · Cache read: %d · Cache write: %d · Cost: %s · %dms\n",
		rec.InputTokens, rec.OutputTokens, rec.CacheReadTokens, rec.CacheWriteTokens, costStr, rec.DurationMs)
	if rec.StatusCode >= 400 {
		fmt.Printf("  Status: %d (error)\n", rec.StatusCode)
	}
	fmt.Println()

	printBody("  Prompt", rec.RequestBody)
	printBody("  Response", rec.ResponseBody)
	fmt.Println()

	return nil
}

func printBody(label string, body []byte) {
	if len(body) == 0 {
		return
	}
	var v any
	if json.Unmarshal(body, &v) == nil {
		pretty, _ := json.MarshalIndent(v, "    ", "  ")
		fmt.Printf("%s:\n    %s\n", label, string(pretty))
	} else {
		fmt.Printf("%s:\n    %s\n", label, format.Truncate(string(body), 500))
	}
}
