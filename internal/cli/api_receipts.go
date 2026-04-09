package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/output"
)

// NewAPIReceiptsCmd returns the board api receipts subcommand group.
func NewAPIReceiptsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "receipts",
		Short: "Manage receipts",
	}
	cmd.AddCommand(
		newAPIReceiptsGetCmd(),
	)
	return cmd
}

func newAPIReceiptsGetCmd() *cobra.Command {
	var documentID int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a receipt by document ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if documentID == 0 {
				return fmt.Errorf("--document-id is required")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetReceipt(cmd.Context(), documentID, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&documentID, "document-id", 0, "Receipt document ID (required)")
	return cmd
}
