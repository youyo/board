package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIVendorsCmd  returns the board api vendors subcommand group.
func NewAPIVendorsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vendors",
		Short: "Manage vendors",
	}
	cmd.AddCommand(
		newAPIVendorsListCmd(),
		newAPIVendorsGetCmd(),
		newAPIVendorsSearchCmd(),
	)
	return cmd
}

func newAPIVendorsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all vendors",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.ListVendors(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
}

func newAPIVendorsGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a vendor by ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id is required")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetVendor(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "Vendor ID (required)")
	return cmd
}

func newAPIVendorsSearchCmd() *cobra.Command {
	var name, updatedAtFrom string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search vendors by criteria",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			params := boardapi.VendorSearchParams{
				Name:          name,
				UpdatedAtFrom: updatedAtFrom,
			}
			result, err := svc.SearchVendors(cmd.Context(), params, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Filter by vendor name")
	cmd.Flags().StringVar(&updatedAtFrom, "updated-at-from", "", "Filter by updated_at (ISO 8601, lower bound)")
	return cmd
}
