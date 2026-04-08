package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/output"
	"github.com/youyo/board/internal/repository"
	"github.com/youyo/board/internal/service/find"
)

// NewFindPaymentCmd returns the board find payment command.
func NewFindPaymentCmd() *cobra.Command {
	var (
		id              int
		vendorName      string
		purchaseOrderID int
		text            string
		status          string
	)

	cmd := &cobra.Command{
		Use:   "payment",
		Short: "Search payments with vendor resolution",
		Long:  "Search for payments by ID, vendor name, purchase order ID, free text, or status. Returns payments with their associated vendor.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 && vendorName == "" && purchaseOrderID == 0 && text == "" && status == "" {
				return fmt.Errorf("at least one of --id, --vendor-name, --purchase-order-id, --text, or --status must be specified")
			}

			svc, err := findServiceFromCmd(cmd)
			if err != nil {
				return err
			}

			opts := readOptionsFromCmd(cmd)
			q := find.FindPaymentQuery{
				ID:              id,
				VendorName:      vendorName,
				PurchaseOrderID: purchaseOrderID,
				Text:            text,
				Status:          status,
				Limit:           opts.Limit,
				Opts: repository.ReadOptions{
					Refresh:      opts.Refresh,
					ForceRefresh: opts.ForceRefresh,
				},
			}

			results, err := svc.FindPayment(cmd.Context(), q)
			if err != nil {
				return err
			}

			return output.Write(os.Stdout, results, prettyFromCmd(cmd))
		},
	}

	cmd.Flags().IntVar(&id, "id", 0, "Payment ID (direct lookup, highest priority)")
	cmd.Flags().StringVar(&vendorName, "vendor-name", "", "Vendor name to resolve payments for")
	cmd.Flags().IntVar(&purchaseOrderID, "purchase-order-id", 0, "Purchase order ID to resolve payments for")
	cmd.Flags().StringVar(&text, "text", "", "Free-text search across memo")
	cmd.Flags().StringVar(&status, "status", "", "Filter by payment status")

	return cmd
}
