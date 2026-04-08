package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIUsersCmd は board api users サブコマンドグループを返す。
func NewAPIUsersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "ユーザー（users）を操作する",
	}
	cmd.AddCommand(
		newAPIUsersListCmd(),
		newAPIUsersGetCmd(),
		newAPIUsersSearchCmd(),
	)
	return cmd
}

func newAPIUsersListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "ユーザーの一覧を取得する",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.ListUsers(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
}

func newAPIUsersGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "指定 ID のユーザーを取得する",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id は必須です")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetUser(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "ユーザー ID（必須）")
	return cmd
}

func newAPIUsersSearchCmd() *cobra.Command {
	var name, email, updatedAtFrom string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "ユーザーを条件で検索する",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			params := boardapi.UserSearchParams{
				Name:          name,
				Email:         email,
				UpdatedAtFrom: updatedAtFrom,
			}
			result, err := svc.SearchUsers(cmd.Context(), params, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "ユーザー名でフィルタ")
	cmd.Flags().StringVar(&email, "email", "", "メールアドレスでフィルタ")
	cmd.Flags().StringVar(&updatedAtFrom, "updated-at-from", "", "更新日時（ISO 8601）以降でフィルタ")
	return cmd
}
