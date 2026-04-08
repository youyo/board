package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIDocumentSendChannelsCmd  returns the board api document_send_channels subcommand group.
func NewAPIDocumentSendChannelsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "document_send_channels",
		Short: "Manage document_send_channels",
	}
	cmd.AddCommand(
		newAPIDocumentSendChannelsListCmd(),
		newAPIDocumentSendChannelsGetCmd(),
		newAPIDocumentSendChannelsSearchCmd(),
	)
	return cmd
}

func newAPIDocumentSendChannelsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all document_send_channels",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.ListDocumentSendChannels(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
}

func newAPIDocumentSendChannelsGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a document_send_channel by ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id is required")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetDocumentSendChannel(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "Document send channel ID (required)")
	return cmd
}

func newAPIDocumentSendChannelsSearchCmd() *cobra.Command {
	var name, updatedAtFrom string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search document_send_channels by criteria",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			params := boardapi.DocumentSendChannelSearchParams{
				Name:          name,
				UpdatedAtFrom: updatedAtFrom,
			}
			result, err := svc.SearchDocumentSendChannels(cmd.Context(), params, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Filter by document send channel name")
	cmd.Flags().StringVar(&updatedAtFrom, "updated-at-from", "", "Filter by updated_at (ISO 8601, lower bound)")
	return cmd
}
