package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIVendorBranchesCmd  returns the board api vendor_branches subcommand group.
func NewAPIVendorBranchesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vendor_branches",
		Short: "Manage vendor_branches",
	}
	cmd.AddCommand(
		newAPIVendorBranchesListCmd(),
		newAPIVendorBranchesGetCmd(),
		newAPIVendorBranchesSearchCmd(),
	)
	return cmd
}

func newAPIVendorBranchesListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all vendor_branches",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			page, _ := cmd.Flags().GetInt("page")
			perPage, _ := cmd.Flags().GetInt("per-page")
			if page > 0 {
				result, err := svc.ListVendorBranchesPage(cmd.Context(), page, perPage)
				if err != nil {
					return err
				}
				totalPages := (result.TotalCount + result.PerPage - 1) / result.PerPage
				fmt.Fprintf(os.Stderr, "# Total: %d, Page: %d/%d, PerPage: %d\n",
					result.TotalCount, result.Page, totalPages, result.PerPage)
				return output.Write(os.Stdout, result.Items, prettyFromCmd(cmd))
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.ListVendorBranches(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().Int("page", 0, "Page number (1-based, bypasses cache)")
	cmd.Flags().Int("per-page", 50, "Items per page (max 100, used with --page)")
	return cmd
}

func newAPIVendorBranchesGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a vendor_branche by ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id is required")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetVendorBranch(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "Vendor branch ID (required)")
	return cmd
}

func newAPIVendorBranchesSearchCmd() *cobra.Command {
	var vendorID int
	var updatedAtFrom string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search vendor_branches by criteria",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			params := boardapi.VendorBranchSearchParams{
				VendorID:      vendorID,
				UpdatedAtFrom: updatedAtFrom,
			}
			result, err := svc.SearchVendorBranches(cmd.Context(), params, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&vendorID, "vendor-id", 0, "Filter by vendor ID")
	cmd.Flags().StringVar(&updatedAtFrom, "updated-at-from", "", "Filter by updated_at (ISO 8601, lower bound)")
	return cmd
}
