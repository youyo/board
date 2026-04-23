package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIProjectCostsCmd returns the board api project_costs subcommand group.
func NewAPIProjectCostsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project_costs",
		Short: "Manage project_costs",
	}
	cmd.AddCommand(
		newAPIProjectCostsListCmd(),
		newAPIProjectCostsGetCmd(),
	)
	return cmd
}

// projectCostListFlagsFromCmd reads the Ransack-style filter flags and returns a
// ProjectCostListOptions. All flags are optional; any flag left at its zero value
// is omitted from the outgoing request.
func projectCostListFlagsFromCmd(cmd *cobra.Command) boardapi.ProjectCostListOptions {
	projectIDEq, _ := cmd.Flags().GetInt("project-id")
	updatedAtGteq, _ := cmd.Flags().GetString("updated-at-gteq")
	updatedAtLteq, _ := cmd.Flags().GetString("updated-at-lteq")

	var includeArchive *bool
	if cmd.Flags().Changed("include-archive-flg") {
		v, _ := cmd.Flags().GetBool("include-archive-flg")
		includeArchive = &v
	}

	return boardapi.ProjectCostListOptions{
		ProjectIDEq:       projectIDEq,
		UpdatedAtGteq:     updatedAtGteq,
		UpdatedAtLteq:     updatedAtLteq,
		IncludeArchiveFlg: includeArchive,
	}
}

func newAPIProjectCostsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List project_costs (optionally filtered by Ransack-style query params)",
		Long: `List project costs. Filters are forwarded to the BOARD API as Ransack-style
query parameters (e.g. --project-id sends project_id_eq). A zero-filter request
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
			filter := projectCostListFlagsFromCmd(cmd)
			result, err := svc.ListProjectCosts(cmd.Context(), readOpts, filter)
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
	cmd.Flags().Int("project-id", 0, "Filter by project ID (Ransack project_id_eq, exact match)")
	cmd.Flags().String("updated-at-gteq", "", `updated_at >= (YYYY-MM-DD HH:MM:SS)`)
	cmd.Flags().String("updated-at-lteq", "", `updated_at <= (YYYY-MM-DD HH:MM:SS)`)
	cmd.Flags().Bool("include-archive-flg", false, "Include archived project costs (send include_archive_flg=1)")
	cmd.Flags().Bool("show-meta", true, "Include _meta (pagination / rate limit / ETag) in JSON output")
	return cmd
}

func newAPIProjectCostsGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a project cost by ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id is required")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetProjectCost(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "Project cost ID (required)")
	return cmd
}
