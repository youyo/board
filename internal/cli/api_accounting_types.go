package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIAccountingTypesCmd  returns the board api accounting_types subcommand group.
func NewAPIAccountingTypesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accounting_types",
		Short: "Manage accounting_types",
	}
	cmd.AddCommand(
		newAPIAccountingTypesListCmd(),
		newAPIAccountingTypesGetCmd(),
		newAPIAccountingTypesSearchCmd(),
	)
	return cmd
}

func newAPIAccountingTypesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all accounting_types",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.ListAccountingTypes(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
}

func newAPIAccountingTypesGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a accounting_type by ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id is required")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetAccountingType(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "Accounting type ID (required)")
	return cmd
}

func newAPIAccountingTypesSearchCmd() *cobra.Command {
	var name, updatedAtFrom string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search accounting_types by criteria",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			params := boardapi.AccountingTypeSearchParams{
				Name:          name,
				UpdatedAtFrom: updatedAtFrom,
			}
			result, err := svc.SearchAccountingTypes(cmd.Context(), params, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Filter by accounting type name")
	cmd.Flags().StringVar(&updatedAtFrom, "updated-at-from", "", "Filter by updated_at (ISO 8601, lower bound)")
	return cmd
}
