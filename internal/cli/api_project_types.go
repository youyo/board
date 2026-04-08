package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIProjectTypesCmd は board api project_types サブコマンドグループを返す。
func NewAPIProjectTypesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project_types",
		Short: "プロジェクト種別（project_types）を操作する",
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
		Short: "プロジェクト種別の一覧を取得する",
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
		Short: "指定 ID のプロジェクト種別を取得する",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id は必須です")
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
	cmd.Flags().IntVar(&id, "id", 0, "プロジェクト種別 ID（必須）")
	return cmd
}

func newAPIProjectTypesSearchCmd() *cobra.Command {
	var name, updatedAtFrom string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "プロジェクト種別を条件で検索する",
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
	cmd.Flags().StringVar(&name, "name", "", "プロジェクト種別名でフィルタ")
	cmd.Flags().StringVar(&updatedAtFrom, "updated-at-from", "", "更新日時（ISO 8601）以降でフィルタ")
	return cmd
}
