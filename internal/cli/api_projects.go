package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIProjectsCmd  returns the board api projects subcommand group.
func NewAPIProjectsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "Manage projects",
	}
	cmd.AddCommand(
		newAPIProjectsListCmd(),
		newAPIProjectsGetCmd(),
	)
	return cmd
}

// projectListFlagsFromCmd reads the Ransack-style filter flags and returns a
// ProjectListOptions. All flags are optional; any flag left at its zero value
// is omitted from the outgoing request.
func projectListFlagsFromCmd(cmd *cobra.Command) boardapi.ProjectListOptions {
	nameCont, _ := cmd.Flags().GetString("name-cont")
	clientID, _ := cmd.Flags().GetInt("client-id")
	clientNameCont, _ := cmd.Flags().GetString("client-name-cont")
	orderStatusIn, _ := cmd.Flags().GetIntSlice("order-status-in")
	deliveryStatusIn, _ := cmd.Flags().GetIntSlice("delivery-status-in")
	projectNoEq, _ := cmd.Flags().GetString("project-no-eq")
	managementNoEq, _ := cmd.Flags().GetString("management-no-eq")
	deliveryDateGteq, _ := cmd.Flags().GetString("delivery-date-gteq")
	deliveryDateLteq, _ := cmd.Flags().GetString("delivery-date-lteq")
	invoiceDateGteq, _ := cmd.Flags().GetString("invoice-date-gteq")
	invoiceDateLteq, _ := cmd.Flags().GetString("invoice-date-lteq")
	invoiceTimingKbnIn, _ := cmd.Flags().GetIntSlice("invoice-timing-kbn-in")
	tags, _ := cmd.Flags().GetStringSlice("tags")
	responseGroup, _ := cmd.Flags().GetString("response-group")
	updatedAtGteq, _ := cmd.Flags().GetString("updated-at-gteq")
	updatedAtLteq, _ := cmd.Flags().GetString("updated-at-lteq")
	createdAtGteq, _ := cmd.Flags().GetString("created-at-gteq")
	createdAtLteq, _ := cmd.Flags().GetString("created-at-lteq")

	var includeArchive *bool
	if cmd.Flags().Changed("include-archive-flg") {
		v, _ := cmd.Flags().GetBool("include-archive-flg")
		includeArchive = &v
	}

	var includeLost *bool
	if cmd.Flags().Changed("include-lost-flg") {
		v, _ := cmd.Flags().GetBool("include-lost-flg")
		includeLost = &v
	}

	return boardapi.ProjectListOptions{
		NameCont:           nameCont,
		ClientIDEq:         clientID,
		ClientNameCont:     clientNameCont,
		OrderStatusIn:      orderStatusIn,
		DeliveryStatusIn:   deliveryStatusIn,
		ProjectNoEq:        projectNoEq,
		ManagementNoEq:     managementNoEq,
		DeliveryDateGteq:   deliveryDateGteq,
		DeliveryDateLteq:   deliveryDateLteq,
		InvoiceDateGteq:    invoiceDateGteq,
		InvoiceDateLteq:    invoiceDateLteq,
		InvoiceTimingKbnIn: invoiceTimingKbnIn,
		Tags:               tags,
		ResponseGroup:      responseGroup,
		UpdatedAtGteq:      updatedAtGteq,
		UpdatedAtLteq:      updatedAtLteq,
		CreatedAtGteq:      createdAtGteq,
		CreatedAtLteq:      createdAtLteq,
		IncludeArchiveFlg:  includeArchive,
		IncludeLostFlg:     includeLost,
	}
}

func newAPIProjectsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List projects (optionally filtered by Ransack-style query params)",
		Long: `List projects. Filters are forwarded to the BOARD API as Ransack-style
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
			filter := projectListFlagsFromCmd(cmd)
			result, err := svc.ListProjects(cmd.Context(), readOpts, filter)
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
	cmd.Flags().String("name-cont", "", "Filter by project name (Ransack name_cont, partial match)")
	cmd.Flags().Int("client-id", 0, "Filter by client ID (Ransack client_id_eq)")
	cmd.Flags().String("client-name-cont", "", "Filter by client name (Ransack client_name_cont, partial match)")
	cmd.Flags().IntSlice("order-status-in", nil, "Filter by order status (comma-separated, e.g. 1,2,3)")
	cmd.Flags().IntSlice("delivery-status-in", nil, "Filter by delivery status (comma-separated)")
	cmd.Flags().String("project-no-eq", "", "Filter by project number (exact match)")
	cmd.Flags().String("management-no-eq", "", "Filter by management number (exact match)")
	cmd.Flags().String("delivery-date-gteq", "", "Delivery date >= (YYYY-MM-DD)")
	cmd.Flags().String("delivery-date-lteq", "", "Delivery date <= (YYYY-MM-DD)")
	cmd.Flags().String("invoice-date-gteq", "", "Invoice date >= (YYYY-MM-DD)")
	cmd.Flags().String("invoice-date-lteq", "", "Invoice date <= (YYYY-MM-DD)")
	cmd.Flags().IntSlice("invoice-timing-kbn-in", nil, "Filter by invoice timing (comma-separated)")
	cmd.Flags().StringSlice("tags", nil, "Filter by tags (repeatable, sent as tags[]=A&tags[]=B)")
	cmd.Flags().String("response-group", "", `Response group: "small" / "large" / "estimate" / "order" / "delivery" / "invoice" / "receipt" / "all"`)
	cmd.Flags().String("updated-at-gteq", "", `updated_at >= (YYYY-MM-DD HH:MM:SS)`)
	cmd.Flags().String("updated-at-lteq", "", `updated_at <= (YYYY-MM-DD HH:MM:SS)`)
	cmd.Flags().String("created-at-gteq", "", `created_at >= (YYYY-MM-DD HH:MM:SS)`)
	cmd.Flags().String("created-at-lteq", "", `created_at <= (YYYY-MM-DD HH:MM:SS)`)
	cmd.Flags().Bool("include-archive-flg", false, "Include archived projects (send include_archive_flg=1)")
	cmd.Flags().Bool("include-lost-flg", false, "Include lost projects (send include_lost_flg=1)")
	cmd.Flags().Bool("show-meta", true, "Include _meta (pagination / rate limit / ETag) in JSON output")
	return cmd
}

func newAPIProjectsGetCmd() *cobra.Command {
	var id int
	var responseGroup string
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a project by ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id is required")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			if responseGroup != "" {
				result, err := svc.GetProjectWithGroup(cmd.Context(), id, responseGroup)
				if err != nil {
					return err
				}
				return output.Write(os.Stdout, result, prettyFromCmd(cmd))
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetProject(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "Project ID (required)")
	cmd.Flags().StringVar(&responseGroup, "response-group", "", `Response group: "estimate" / "order" / "delivery" / "invoice" / "receipt" / "all"`)
	return cmd
}
