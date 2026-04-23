package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIClientsCmd returns the board api clients subcommand group.
func NewAPIClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients",
		Short: "Manage clients",
	}
	cmd.AddCommand(
		newAPIClientsListCmd(),
		newAPIClientsGetCmd(),
	)
	return cmd
}

// clientListFlagsFromCmd reads the Ransack-style filter flags and returns a
// ClientListOptions. All flags are optional; any flag left at its zero value
// is omitted from the outgoing request.
func clientListFlagsFromCmd(cmd *cobra.Command) boardapi.ClientListOptions {
	nameCont, _ := cmd.Flags().GetString("name-cont")
	nameDispCont, _ := cmd.Flags().GetString("name-disp-cont")
	invoiceNumberEq, _ := cmd.Flags().GetString("invoice-system-number-eq")
	customNoEq, _ := cmd.Flags().GetString("custom-no-eq")
	tags, _ := cmd.Flags().GetStringSlice("tags")
	responseGroup, _ := cmd.Flags().GetString("response-group")
	updatedAtGteq, _ := cmd.Flags().GetString("updated-at-gteq")
	updatedAtLteq, _ := cmd.Flags().GetString("updated-at-lteq")

	var includeArchive *bool
	if cmd.Flags().Changed("include-archive-flg") {
		v, _ := cmd.Flags().GetBool("include-archive-flg")
		includeArchive = &v
	}

	return boardapi.ClientListOptions{
		NameCont:              nameCont,
		NameDispCont:          nameDispCont,
		InvoiceSystemNumberEq: invoiceNumberEq,
		CustomNoEq:            customNoEq,
		Tags:                  tags,
		ResponseGroup:         responseGroup,
		UpdatedAtGteq:         updatedAtGteq,
		UpdatedAtLteq:         updatedAtLteq,
		IncludeArchiveFlg:     includeArchive,
	}
}

func newAPIClientsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List clients (optionally filtered by Ransack-style query params)",
		Long: `List clients. Filters are forwarded to the BOARD API as Ransack-style
query parameters (e.g. --name-cont sends name_cont). A zero-filter request
uses the local cache; any non-zero filter bypasses the cache and calls the
API directly so server-side filter semantics take effect.

JSON output includes an _meta object (total_count, page, per_page, rate
limits, ETag, last_modified) derived from response headers. Use
--no-show-meta to omit it.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			readOpts := readOptionsFromCmd(cmd)
			filter := clientListFlagsFromCmd(cmd)
			result, err := svc.ListClients(cmd.Context(), readOpts, filter)
			if err != nil {
				return err
			}
			showMeta, _ := cmd.Flags().GetBool("show-meta")
			if showMeta {
				return output.Write(os.Stdout, result, prettyFromCmd(cmd))
			}
			return output.Write(os.Stdout, result.Items, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().String("name-cont", "", "Filter by customer name (Ransack name_cont, partial match)")
	cmd.Flags().String("name-disp-cont", "", "Filter by display name (Ransack name_disp_cont, partial match)")
	cmd.Flags().String("invoice-system-number-eq", "", "Filter by invoice system number (exact match)")
	cmd.Flags().String("custom-no-eq", "", "Filter by custom_no (exact match)")
	cmd.Flags().StringSlice("tags", nil, "Filter by tags (repeatable, sent as tags[]=A&tags[]=B)")
	cmd.Flags().String("response-group", "", `Response group: "small" (default) or "large" (includes Get-only fields)`)
	cmd.Flags().String("updated-at-gteq", "", `updated_at >= (YYYY-MM-DD HH:MM:SS)`)
	cmd.Flags().String("updated-at-lteq", "", `updated_at <= (YYYY-MM-DD HH:MM:SS)`)
	cmd.Flags().Bool("include-archive-flg", false, "Include archived customers (send include_archive_flg=1)")
	cmd.Flags().Bool("show-meta", true, "Include _meta (pagination / rate limit / ETag) in JSON output")
	return cmd
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
