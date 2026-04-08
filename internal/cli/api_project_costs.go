package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIProjectCostsCmd  returns the board api project_costs subcommand group.
func NewAPIProjectCostsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project_costs",
		Short: "Manage project_costs",
	}
	cmd.AddCommand(
		newAPIProjectCostsListCmd(),
		newAPIProjectCostsGetCmd(),
		newAPIProjectCostsSearchCmd(),
	)
	return cmd
}

func newAPIProjectCostsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all project_costs",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.ListProjectCosts(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
}

func newAPIProjectCostsGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a project_cost by ID",
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

func newAPIProjectCostsSearchCmd() *cobra.Command {
	var projectID int
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search project_costs by criteria",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			params := boardapi.ProjectCostSearchParams{
				ProjectID: projectID,
			}
			result, err := svc.SearchProjectCosts(cmd.Context(), params, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&projectID, "project-id", 0, "Filter by project ID")
	return cmd
}
