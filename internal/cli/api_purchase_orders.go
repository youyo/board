package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIPurchaseOrdersCmd  returns the board api purchase_orders subcommand group.
func NewAPIPurchaseOrdersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "purchase_orders",
		Short: "Manage purchase_orders",
	}
	cmd.AddCommand(
		newAPIPurchaseOrdersListCmd(),
		newAPIPurchaseOrdersGetCmd(),
		newAPIPurchaseOrdersSearchCmd(),
	)
	return cmd
}

func newAPIPurchaseOrdersListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all purchase_orders",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.ListPurchaseOrders(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
}

func newAPIPurchaseOrdersGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a purchase_order by ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id is required")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetPurchaseOrder(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "Purchase order ID (required)")
	return cmd
}

func newAPIPurchaseOrdersSearchCmd() *cobra.Command {
	var vendorID, projectID int
	var status, updatedAtFrom string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search purchase_orders by criteria",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			params := boardapi.PurchaseOrderSearchParams{
				VendorID:      vendorID,
				ProjectID:     projectID,
				Status:        status,
				UpdatedAtFrom: updatedAtFrom,
			}
			result, err := svc.SearchPurchaseOrders(cmd.Context(), params, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&vendorID, "vendor-id", 0, "Filter by vendor ID")
	cmd.Flags().IntVar(&projectID, "project-id", 0, "Filter by project ID")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status")
	cmd.Flags().StringVar(&updatedAtFrom, "updated-at-from", "", "Filter by updated_at (ISO 8601, lower bound)")
	return cmd
}
