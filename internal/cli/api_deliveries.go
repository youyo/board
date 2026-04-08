package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIDeliveriesCmd  returns the board api deliveries subcommand group.
func NewAPIDeliveriesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deliveries",
		Short: "Manage deliveries",
	}
	cmd.AddCommand(
		newAPIDeliveriesListCmd(),
		newAPIDeliveriesGetCmd(),
		newAPIDeliveriesSearchCmd(),
	)
	return cmd
}

func newAPIDeliveriesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all deliveries",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.ListDeliveries(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
}

func newAPIDeliveriesGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a deliverie by ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id is required")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetDelivery(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "Delivery ID (required)")
	return cmd
}

func newAPIDeliveriesSearchCmd() *cobra.Command {
	var clientID int
	var projectID int
	var status, updatedAtFrom string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search deliveries by criteria",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			params := boardapi.DeliverySearchParams{
				ClientID:      clientID,
				ProjectID:     projectID,
				Status:        status,
				UpdatedAtFrom: updatedAtFrom,
			}
			result, err := svc.SearchDeliveries(cmd.Context(), params, opts)
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
