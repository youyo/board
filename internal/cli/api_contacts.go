package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIContactsCmd  returns the board api contacts subcommand group.
func NewAPIContactsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contacts",
		Short: "Manage contacts",
	}
	cmd.AddCommand(
		newAPIContactsListCmd(),
		newAPIContactsGetCmd(),
		newAPIContactsSearchCmd(),
	)
	return cmd
}

func newAPIContactsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all contacts",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			page, _ := cmd.Flags().GetInt("page")
			perPage, _ := cmd.Flags().GetInt("per-page")
			if page > 0 {
				result, err := svc.ListContactsPage(cmd.Context(), page, perPage)
				if err != nil {
					return err
				}
				totalPages := (result.TotalCount + result.PerPage - 1) / result.PerPage
				fmt.Fprintf(os.Stderr, "# Total: %d, Page: %d/%d, PerPage: %d\n",
					result.TotalCount, result.Page, totalPages, result.PerPage)
				return output.Write(os.Stdout, result.Items, prettyFromCmd(cmd))
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.ListContacts(cmd.Context(), opts)
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

func newAPIContactsGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a contact by ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id is required")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetContact(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "Contact ID (required)")
	return cmd
}

func newAPIContactsSearchCmd() *cobra.Command {
	var clientID int
	var name, email string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search contacts by criteria",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			params := boardapi.ContactSearchParams{
				ClientID: clientID,
				Name:     name,
				Email:    email,
			}
			result, err := svc.SearchContacts(cmd.Context(), params, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&clientID, "client-id", 0, "Filter by client ID")
	cmd.Flags().StringVar(&name, "name", "", "Filter by contact name")
	cmd.Flags().StringVar(&email, "email", "", "Filter by email address")
	return cmd
}
