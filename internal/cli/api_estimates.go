package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/output"
)

// NewAPIEstimatesCmd returns the board api estimates subcommand group.
func NewAPIEstimatesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "estimates",
		Short: "Manage estimates",
	}
	cmd.AddCommand(
		newAPIEstimatesGetCmd(),
	)
	return cmd
}

func newAPIEstimatesGetCmd() *cobra.Command {
	var documentID int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get an estimate by document ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if documentID == 0 {
				return fmt.Errorf("--document-id is required")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetEstimate(cmd.Context(), documentID, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&documentID, "document-id", 0, "Estimate document ID (required)")
	return cmd
}
