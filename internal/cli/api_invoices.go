package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIInvoicesCmd  returns the board api invoices subcommand group.
func NewAPIInvoicesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "invoices",
		Short: "Manage invoices",
	}
	cmd.AddCommand(
		newAPIInvoicesListCmd(),
		newAPIInvoicesGetCmd(),
		newAPIInvoicesSearchCmd(),
	)
	return cmd
}

func newAPIInvoicesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all invoices",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.ListInvoices(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
}

func newAPIInvoicesGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a invoice by ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id is required")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetInvoice(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "Invoice ID (required)")
	return cmd
}

func newAPIInvoicesSearchCmd() *cobra.Command {
	var clientID int
	var projectID int
	var status, updatedAtFrom string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search invoices by criteria",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			params := boardapi.InvoiceSearchParams{
				ClientID:      clientID,
				ProjectID:     projectID,
				Status:        status,
				UpdatedAtFrom: updatedAtFrom,
			}
			result, err := svc.SearchInvoices(cmd.Context(), params, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&clientID, "client-id", 0, "Filter by client ID")
	cmd.Flags().IntVar(&projectID, "project-id", 0, "Filter by project ID")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status")
	cmd.Flags().StringVar(&updatedAtFrom, "updated-at-from", "", "Filter by updated_at (ISO 8601, lower bound)")
	return cmd
}
