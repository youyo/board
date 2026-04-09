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
		newAPIProjectsSearchCmd(),
	)
	return cmd
}

func newAPIProjectsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			page, _ := cmd.Flags().GetInt("page")
			perPage, _ := cmd.Flags().GetInt("per-page")
			if page > 0 {
				result, err := svc.ListProjectsPage(cmd.Context(), page, perPage)
				if err != nil {
					return err
				}
				totalPages := (result.TotalCount + result.PerPage - 1) / result.PerPage
				fmt.Fprintf(os.Stderr, "# Total: %d, Page: %d/%d, PerPage: %d\n",
					result.TotalCount, result.Page, totalPages, result.PerPage)
				return output.Write(os.Stdout, result.Items, prettyFromCmd(cmd))
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.ListProjects(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().Int("page", 0, "Page number (1-based, bypasses cache)")
	cmd.Flags().Int("per-page", 50, "Items per page (max 100, used with --page)")
	return cmd
}

func newAPIProjectsGetCmd() *cobra.Command {
	var id int
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
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetProject(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "Project ID (required)")
	return cmd
}

func newAPIProjectsSearchCmd() *cobra.Command {
	var clientID int
	var name, status, updatedAtFrom string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search projects by criteria",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			params := boardapi.ProjectSearchParams{
				ClientID:      clientID,
				Name:          name,
				Status:        status,
				UpdatedAtFrom: updatedAtFrom,
			}
			result, err := svc.SearchProjects(cmd.Context(), params, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&clientID, "client-id", 0, "Filter by client ID")
	cmd.Flags().StringVar(&name, "name", "", "Filter by project name")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status")
	cmd.Flags().StringVar(&updatedAtFrom, "updated-at-from", "", "Filter by updated_at (ISO 8601, lower bound)")
	return cmd
}
