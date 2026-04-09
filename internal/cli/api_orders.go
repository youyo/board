package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/output"
)

// NewAPIOrdersCmd returns the board api orders subcommand group.
func NewAPIOrdersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orders",
		Short: "Manage orders",
	}
	cmd.AddCommand(
		newAPIOrdersGetCmd(),
	)
	return cmd
}

func newAPIOrdersGetCmd() *cobra.Command {
	var documentID int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get an order by document ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if documentID == 0 {
				return fmt.Errorf("--document-id is required")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetOrder(cmd.Context(), documentID, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&documentID, "document-id", 0, "Order document ID (required)")
	return cmd
}
