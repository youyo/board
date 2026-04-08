package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIClientsCmd  returns the board api clients subcommand group.
func NewAPIClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients",
		Short: "Manage clients",
	}
	cmd.AddCommand(
		newAPIClientsListCmd(),
		newAPIClientsGetCmd(),
		newAPIClientsSearchCmd(),
	)
	return cmd
}

func newAPIClientsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all clients",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.ListClients(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
}

func newAPIClientsGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a client by ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id is required")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetClient(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "Client ID (required)")
	return cmd
}

func newAPIClientsSearchCmd() *cobra.Command {
	var name, updatedAtFrom string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search clients by criteria",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			params := boardapi.ClientSearchParams{
				Name:          name,
				UpdatedAtFrom: updatedAtFrom,
			}
			result, err := svc.SearchClients(cmd.Context(), params, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Filter by client name")
	cmd.Flags().StringVar(&updatedAtFrom, "updated-at-from", "", "Filter by updated_at (ISO 8601, lower bound)")
	return cmd
}
