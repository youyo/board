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
		Long: `Get a receipt by document ID.

JSON output includes an _meta object (rate limit, ETag, last_modified)
derived from response headers. Use --no-show-meta to omit it.`,
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
			showMeta, _ := cmd.Flags().GetBool("show-meta")
			if showMeta {
				return output.Write(os.Stdout, result, prettyFromCmd(cmd))
			}
			return output.Write(os.Stdout, result.Item, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&documentID, "document-id", 0, "Receipt document ID (required)")
	cmd.Flags().Bool("show-meta", true, "Include _meta (rate limit / ETag) in JSON output")
	return cmd
}
