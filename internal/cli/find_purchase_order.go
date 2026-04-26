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
		text        string
		status      string
	)

	cmd := &cobra.Command{
		Use:   "purchase-order",
		Short: "Search purchase orders with vendor/project resolution",
		Long:  "Search for purchase orders by ID, vendor name, project name, free text, or status. Returns purchase orders with their associated vendor and project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 && vendorName == "" && projectName == "" && text == "" && status == "" {
				return fmt.Errorf("at least one of --id, --vendor-name, --project-name, --text, or --status must be specified")
			}
			if vendorName != "" || projectName != "" {
				return fmt.Errorf("--vendor-name / --project-name are not supported in v0.7.0 service rename (N07b); will be wired in N07c")
			}

			svc, err := findServiceFromCmd(cmd)
			if err != nil {
				return err
			}

			opts := readOptionsFromCmd(cmd)
			q := find.FindPurchaseOrderQuery{
				FindCommonOpts: find.FindCommonOpts{
					Limit: opts.Limit,
					Opts: repository.ReadOptions{
						Refresh:      opts.Refresh,
						ForceRefresh: opts.ForceRefresh,
					},
				},
				ID:     id,
				Text:   text,
				Status: status,
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
	cmd.Flags().StringVar(&text, "text", "", "Free-text search across title, memo")
	cmd.Flags().StringVar(&status, "status", "", "Filter by purchase order status")

	return cmd
}
