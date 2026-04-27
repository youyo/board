package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/output"
	"github.com/youyo/board/internal/repository"
	"github.com/youyo/board/internal/service/find"
)

// NewFindPurchaseOrderCmd returns the board find purchase-order command.
func NewFindPurchaseOrderCmd() *cobra.Command {
	var (
		id          int
		vendorName  string
		projectName string
		status      string
	)

	cmd := &cobra.Command{
		Use:   "purchase-order",
		Short: "Search purchase orders with vendor/project resolution",
		Long:  "Search for purchase orders by ID, vendor name, project name or status. Returns purchase orders with their associated vendor and project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 && vendorName == "" && projectName == "" && status == "" {
				return fmt.Errorf("at least one of --id, --vendor-name, --project-name, or --status must be specified")
			}
			if projectName != "" {
				return fmt.Errorf("--project-name is not yet supported for purchase orders (tracked for future enhancement)")
			}

			svc, err := findServiceFromCmd(cmd)
			if err != nil {
				return err
			}

			opts := readOptionsFromCmd(cmd)
			readOpts := repository.ReadOptions{Refresh: opts.Refresh, ForceRefresh: opts.ForceRefresh}

			var vendorID int
			if vendorName != "" {
				vendorID, err = svc.ResolveVendorByName(cmd.Context(), vendorName, readOpts)
				if err != nil {
					return err
				}
			}

			q := find.FindPurchaseOrderQuery{
				FindCommonOpts: find.FindCommonOpts{
					Limit: opts.Limit,
					Opts:  readOpts,
				},
				ID:       id,
				VendorID: vendorID,
				Status:   status,
			}

			results, err := svc.FindPurchaseOrder(cmd.Context(), q)
			if err != nil {
				return err
			}

			return output.Write(os.Stdout, results, prettyFromCmd(cmd))
		},
	}

	cmd.Flags().IntVar(&id, "id", 0, "Purchase order ID (direct lookup, highest priority)")
	cmd.Flags().StringVar(&vendorName, "vendor-name", "", "Vendor name to resolve purchase orders for")
	cmd.Flags().StringVar(&projectName, "project-name", "", "Project name to resolve purchase orders for")
	cmd.Flags().StringVar(&status, "status", "", "Filter by purchase order status")

	return cmd
}
