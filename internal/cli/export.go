package cli

import (
	"fmt"
	"os"

	"github.com/lanesket/llm.log/internal/export"
	"github.com/lanesket/llm.log/internal/storage"
	"github.com/spf13/cobra"
)

func init() {
	exportCmd.Flags().StringP("format", "f", "csv", "Output format: csv, json, jsonl")
	exportCmd.Flags().StringP("period", "p", "month", "Period: today, week, month, all")
	exportCmd.Flags().String("from", "", "Start date (YYYY-MM-DD)")
	exportCmd.Flags().String("to", "", "End date (YYYY-MM-DD)")
	exportCmd.Flags().StringP("source", "s", "", "Filter by source")
	exportCmd.Flags().String("provider", "", "Filter by provider")
	exportCmd.Flags().StringP("output", "o", "", "Output file (default: stdout)")
	exportCmd.Flags().Bool("with-bodies", false, "Include request/response bodies")
	rootCmd.AddCommand(exportCmd)
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export logged data to CSV, JSON, or JSONL",
	RunE:  runExport,
}

func runExport(cmd *cobra.Command, args []string) error {
	dir := DataDir()
	store, err := storage.Open(dir)
	if err != nil {
		return err
	}
	defer store.Close()

	formatStr, _ := cmd.Flags().GetString("format")
	period, _ := cmd.Flags().GetString("period")
	fromStr, _ := cmd.Flags().GetString("from")
	toStr, _ := cmd.Flags().GetString("to")
	source, _ := cmd.Flags().GetString("source")
	provider, _ := cmd.Flags().GetString("provider")
	output, _ := cmd.Flags().GetString("output")
	withBodies, _ := cmd.Flags().GetBool("with-bodies")

	f := export.Format(formatStr)
	switch f {
	case export.CSV, export.JSON, export.JSONL:
	default:
		return fmt.Errorf("unsupported format %q (use csv, json, or jsonl)", formatStr)
	}

	from, to, err := parseDateRange(fromStr, toStr, period)
	if err != nil {
		return err
	}

	records, err := store.Recent(0, from, to, provider, source)
	if err != nil {
		return err
	}

	var w *os.File
	if output != "" {
		w, err = os.Create(output)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer w.Close()
	} else {
		w = os.Stdout
	}

	opts := export.Options{Format: f, WithBodies: withBodies}

	var bodyFetcher func(int64) (*storage.Record, error)
	if withBodies {
		bodyFetcher = store.Get
	}

	return export.Write(w, records, opts, bodyFetcher)
}
