package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/output"
)

// NewAPIDeliveriesCmd returns the board api deliveries subcommand group.
func NewAPIDeliveriesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deliveries",
		Short: "Manage deliveries",
	}
	cmd.AddCommand(
		newAPIDeliveriesGetCmd(),
	)
	return cmd
}

func newAPIDeliveriesGetCmd() *cobra.Command {
	var documentID int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a delivery by document ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if documentID == 0 {
				return fmt.Errorf("--document-id is required")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetDelivery(cmd.Context(), documentID, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&documentID, "document-id", 0, "Delivery document ID (required)")
	return cmd
}
