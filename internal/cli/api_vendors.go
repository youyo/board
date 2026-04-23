package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIVendorsCmd returns the board api vendors subcommand group.
func NewAPIVendorsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vendors",
		Short: "Manage vendors",
	}
	cmd.AddCommand(
		newAPIVendorsListCmd(),
		newAPIVendorsGetCmd(),
	)
	return cmd
}

// vendorListFlagsFromCmd reads the Ransack-style filter flags and returns a
// VendorListOptions. All flags are optional; any flag left at its zero value
// is omitted from the outgoing request.
func vendorListFlagsFromCmd(cmd *cobra.Command) boardapi.VendorListOptions {
	nameCont, _ := cmd.Flags().GetString("name-cont")
	updatedAtGteq, _ := cmd.Flags().GetString("updated-at-gteq")
	updatedAtLteq, _ := cmd.Flags().GetString("updated-at-lteq")

	var includeArchive *bool
	if cmd.Flags().Changed("include-archive-flg") {
		v, _ := cmd.Flags().GetBool("include-archive-flg")
		includeArchive = &v
	}

	return boardapi.VendorListOptions{
		NameCont:          nameCont,
		UpdatedAtGteq:     updatedAtGteq,
		UpdatedAtLteq:     updatedAtLteq,
		IncludeArchiveFlg: includeArchive,
	}
}

func newAPIVendorsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List vendors (optionally filtered by Ransack-style query params)",
		Long: `List vendors. Filters are forwarded to the BOARD API as Ransack-style
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
			filter := vendorListFlagsFromCmd(cmd)
			result, err := svc.ListVendors(cmd.Context(), readOpts, filter)
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
	cmd.Flags().String("name-cont", "", "Filter by vendor name (Ransack name_cont, partial match)")
	cmd.Flags().String("updated-at-gteq", "", `updated_at >= (YYYY-MM-DD HH:MM:SS)`)
	cmd.Flags().String("updated-at-lteq", "", `updated_at <= (YYYY-MM-DD HH:MM:SS)`)
	cmd.Flags().Bool("include-archive-flg", false, "Include archived vendors (send include_archive_flg=1)")
	cmd.Flags().Bool("show-meta", true, "Include _meta (pagination / rate limit / ETag) in JSON output")
	return cmd
}

func newAPIVendorsGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a vendor by ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id is required")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetVendor(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "Vendor ID (required)")
	return cmd
}
