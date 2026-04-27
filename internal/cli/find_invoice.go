package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/output"
	"github.com/youyo/board/internal/repository"
	"github.com/youyo/board/internal/service/find"
)

// NewFindInvoiceCmd returns the board find invoice command.
func NewFindInvoiceCmd() *cobra.Command {
	var (
		id          int
		clientName  string
		projectName string
		status      string
	)

	cmd := &cobra.Command{
		Use:   "invoice",
		Short: "Search invoices with client/project resolution",
		Long:  "Search for invoices by ID, client name, project name or status. Returns invoices with their associated client and project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 && clientName == "" && projectName == "" && status == "" {
				return fmt.Errorf("at least one of --id, --client-name, --project-name, or --status must be specified")
			}
			if projectName != "" {
				return fmt.Errorf("--project-name is not yet supported for invoices (tracked for future enhancement)")
			}

			svc, err := findServiceFromCmd(cmd)
			if err != nil {
				return err
			}

			opts := readOptionsFromCmd(cmd)
			readOpts := repository.ReadOptions{Refresh: opts.Refresh, ForceRefresh: opts.ForceRefresh}

			var clientID int
			if clientName != "" {
				clientID, err = svc.ResolveClientByName(cmd.Context(), clientName, readOpts)
				if err != nil {
					return err
				}
			}

			q := find.FindInvoiceQuery{
				FindCommonOpts: find.FindCommonOpts{
					Limit: opts.Limit,
					Opts:  readOpts,
				},
				ID:       id,
				ClientID: clientID,
				Status:   status,
			}

			results, err := svc.FindInvoice(cmd.Context(), q)
			if err != nil {
				return err
			}

			return output.Write(os.Stdout, results, prettyFromCmd(cmd))
		},
	}

	cmd.Flags().IntVar(&id, "id", 0, "Invoice ID (direct lookup, highest priority)")
	cmd.Flags().StringVar(&clientName, "client-name", "", "Client name to resolve invoices for")
	cmd.Flags().StringVar(&projectName, "project-name", "", "Project name to resolve invoices for")
	cmd.Flags().StringVar(&status, "status", "", "Filter by invoice status")

	return cmd
}
