package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIPaymentsCmd returns the board api payments subcommand group.
func NewAPIPaymentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "payments",
		Short: "Manage payments",
	}
	cmd.AddCommand(
		newAPIPaymentsListCmd(),
		newAPIPaymentsGetCmd(),
	)
	return cmd
}

// paymentListFlagsFromCmd reads the Ransack-style filter flags and returns a
// PaymentListOptions. All flags are optional; any flag left at its zero value
// is omitted from the outgoing request.
func paymentListFlagsFromCmd(cmd *cobra.Command) boardapi.PaymentListOptions {
	vendorIDEq, _ := cmd.Flags().GetInt("vendor-id-eq")
	purchaseOrderIDEq, _ := cmd.Flags().GetInt("purchase-order-id-eq")
	statusEq, _ := cmd.Flags().GetString("status-eq")
	responseGroup, _ := cmd.Flags().GetString("response-group")
	updatedAtGteq, _ := cmd.Flags().GetString("updated-at-gteq")
	updatedAtLteq, _ := cmd.Flags().GetString("updated-at-lteq")

	var includeArchive *bool
	if cmd.Flags().Changed("include-archive-flg") {
		v, _ := cmd.Flags().GetBool("include-archive-flg")
		includeArchive = &v
	}

	return boardapi.PaymentListOptions{
		VendorIDEq:        vendorIDEq,
		PurchaseOrderIDEq: purchaseOrderIDEq,
		StatusEq:          statusEq,
		ResponseGroup:     responseGroup,
		UpdatedAtGteq:     updatedAtGteq,
		UpdatedAtLteq:     updatedAtLteq,
		IncludeArchiveFlg: includeArchive,
	}
}

func newAPIPaymentsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List payments (optionally filtered by Ransack-style query params)",
		Long: `List payments. Filters are forwarded to the BOARD API as Ransack-style
query parameters (e.g. --vendor-id-eq sends vendor_id_eq). A zero-filter
request uses the local cache; any non-zero filter bypasses the cache and
calls the API directly so server-side filter semantics take effect.

Note: the real BOARD API path is /v1/expenditure_payments (Go name: payments).

JSON output includes an _meta object (total_count, page, per_page, rate
limits, ETag, last_modified) derived from response headers. Use
--no-show-meta to omit it.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			readOpts := readOptionsFromCmd(cmd)
			filter := paymentListFlagsFromCmd(cmd)
			result, err := svc.ListPayments(cmd.Context(), readOpts, filter)
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
	cmd.Flags().Int("vendor-id-eq", 0, "Filter by vendor ID (exact match)")
	cmd.Flags().Int("purchase-order-id-eq", 0, "Filter by purchase order ID (exact match)")
	cmd.Flags().String("status-eq", "", "Filter by status (exact match)")
	cmd.Flags().String("response-group", "", `Response group: "small" (default) or "large"`)
	cmd.Flags().String("updated-at-gteq", "", `updated_at >= (YYYY-MM-DD HH:MM:SS)`)
	cmd.Flags().String("updated-at-lteq", "", `updated_at <= (YYYY-MM-DD HH:MM:SS)`)
	cmd.Flags().Bool("include-archive-flg", false, "Include archived payments (send include_archive_flg=1)")
	cmd.Flags().Bool("show-meta", true, "Include _meta (pagination / rate limit / ETag) in JSON output")
	return cmd
}

func newAPIPaymentsGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a payment by ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id is required")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetPayment(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "Payment ID (required)")
	return cmd
}
