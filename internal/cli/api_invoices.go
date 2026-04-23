package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIInvoicesCmd returns the board api invoices subcommand group.
func NewAPIInvoicesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "invoices",
		Short: "Manage invoices",
	}
	cmd.AddCommand(
		newAPIInvoicesListCmd(),
		newAPIInvoicesGetCmd(),
	)
	return cmd
}

// invoiceListFlagsFromCmd reads the Ransack-style filter flags and returns an
// InvoiceListOptions. All flags are optional; any flag left at its zero value
// is omitted from the outgoing request.
func invoiceListFlagsFromCmd(cmd *cobra.Command) boardapi.InvoiceListOptions {
	clientIDEq, _ := cmd.Flags().GetInt("client-id-eq")
	projectIDEq, _ := cmd.Flags().GetInt("project-id-eq")
	statusEq, _ := cmd.Flags().GetString("status-eq")
	responseGroup, _ := cmd.Flags().GetString("response-group")
	updatedAtGteq, _ := cmd.Flags().GetString("updated-at-gteq")
	updatedAtLteq, _ := cmd.Flags().GetString("updated-at-lteq")

	var includeArchive *bool
	if cmd.Flags().Changed("include-archive-flg") {
		v, _ := cmd.Flags().GetBool("include-archive-flg")
		includeArchive = &v
	}

	return boardapi.InvoiceListOptions{
		ClientIDEq:        clientIDEq,
		ProjectIDEq:       projectIDEq,
		StatusEq:          statusEq,
		ResponseGroup:     responseGroup,
		UpdatedAtGteq:     updatedAtGteq,
		UpdatedAtLteq:     updatedAtLteq,
		IncludeArchiveFlg: includeArchive,
	}
}

func newAPIInvoicesListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List invoices (optionally filtered by Ransack-style query params)",
		Long: `List invoices. Filters are forwarded to the BOARD API as Ransack-style
query parameters (e.g. --client-id-eq sends client_id_eq). A zero-filter
request uses the local cache; any non-zero filter bypasses the cache and
calls the API directly so server-side filter semantics take effect.

JSON output includes an _meta object (total_count, page, per_page, rate
limits, ETag, last_modified) derived from response headers. Use
--no-show-meta to omit it.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			readOpts := readOptionsFromCmd(cmd)
			filter := invoiceListFlagsFromCmd(cmd)
			result, err := svc.ListInvoices(cmd.Context(), readOpts, filter)
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
	cmd.Flags().Int("client-id-eq", 0, "Filter by client ID (exact match)")
	cmd.Flags().Int("project-id-eq", 0, "Filter by project ID (exact match)")
	cmd.Flags().String("status-eq", "", "Filter by status (exact match)")
	cmd.Flags().String("response-group", "", `Response group: "small" (default) or "large"`)
	cmd.Flags().String("updated-at-gteq", "", `updated_at >= (YYYY-MM-DD HH:MM:SS)`)
	cmd.Flags().String("updated-at-lteq", "", `updated_at <= (YYYY-MM-DD HH:MM:SS)`)
	cmd.Flags().Bool("include-archive-flg", false, "Include archived invoices (send include_archive_flg=1)")
	cmd.Flags().Bool("show-meta", true, "Include _meta (pagination / rate limit / ETag) in JSON output")
	return cmd
}

func newAPIInvoicesGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get an invoice by ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id is required")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetInvoice(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "Invoice ID (required)")
	return cmd
}
