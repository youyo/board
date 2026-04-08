package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIProjectCostsCmd は board api project_costs サブコマンドグループを返す。
func NewAPIProjectCostsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project_costs",
		Short: "案件原価（project_costs）を操作する",
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
		Short: "案件原価の一覧を取得する",
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
		Short: "指定 ID の案件原価を取得する",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id は必須です")
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
	cmd.Flags().IntVar(&id, "id", 0, "案件原価 ID（必須）")
	return cmd
}

func newAPIProjectCostsSearchCmd() *cobra.Command {
	var projectID int
	cmd := &cobra.Command{
		Use:   "search",
		Short: "案件原価を条件で検索する",
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
	cmd.Flags().IntVar(&projectID, "project-id", 0, "案件 ID でフィルタ")
	return cmd
}
