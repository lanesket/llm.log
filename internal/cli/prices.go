package cli

import (
	"fmt"

	"github.com/lanesket/llm.log/internal/pricing"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(updatePricesCmd)
}

var updatePricesCmd = &cobra.Command{
	Use:   "update-prices",
	Short: "Update pricing data from upstream",
	RunE: func(cmd *cobra.Command, args []string) error {
		db := pricing.NewDB(DataDir())
		if err := db.Update(); err != nil {
			return err
		}
		fmt.Println("Pricing data updated.")
		return nil
	},
}
