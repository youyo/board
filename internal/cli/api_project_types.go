package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIProjectTypesCmd  returns the board api project_types subcommand group.
func NewAPIProjectTypesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project_types",
		Short: "Manage project_types",
	}
	cmd.AddCommand(
		newAPIProjectTypesListCmd(),
		newAPIProjectTypesGetCmd(),
		newAPIProjectTypesSearchCmd(),
	)
	return cmd
}

func newAPIProjectTypesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all project_types",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.ListProjectTypes(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
}

func newAPIProjectTypesGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a project_type by ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id is required")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetProjectType(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "Project type ID (required)")
	return cmd
}

func newAPIProjectTypesSearchCmd() *cobra.Command {
	var name, updatedAtFrom string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search project_types by criteria",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			params := boardapi.ProjectTypeSearchParams{
				Name:          name,
				UpdatedAtFrom: updatedAtFrom,
			}
			result, err := svc.SearchProjectTypes(cmd.Context(), params, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Filter by project type name")
	cmd.Flags().StringVar(&updatedAtFrom, "updated-at-from", "", "Filter by updated_at (ISO 8601, lower bound)")
	return cmd
}
