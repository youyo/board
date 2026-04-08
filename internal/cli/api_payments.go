package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIPaymentsCmd  returns the board api payments subcommand group.
func NewAPIPaymentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "payments",
		Short: "Manage payments",
	}
	cmd.AddCommand(
		newAPIPaymentsListCmd(),
		newAPIPaymentsGetCmd(),
		newAPIPaymentsSearchCmd(),
	)
	return cmd
}

func newAPIPaymentsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all payments",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.ListPayments(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
}

func newAPIPaymentsGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a payment by ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id is required")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetPayment(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "Payment ID (required)")
	return cmd
}

func newAPIPaymentsSearchCmd() *cobra.Command {
	var vendorID, purchaseOrderID int
	var status, updatedAtFrom string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search payments by criteria",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			params := boardapi.PaymentSearchParams{
				VendorID:        vendorID,
				PurchaseOrderID: purchaseOrderID,
				Status:          status,
				UpdatedAtFrom:   updatedAtFrom,
			}
			result, err := svc.SearchPayments(cmd.Context(), params, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&vendorID, "vendor-id", 0, "Filter by vendor ID")
	cmd.Flags().IntVar(&purchaseOrderID, "purchase-order-id", 0, "Filter by purchase order ID")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status")
	cmd.Flags().StringVar(&updatedAtFrom, "updated-at-from", "", "Filter by updated_at (ISO 8601, lower bound)")
	return cmd
}
