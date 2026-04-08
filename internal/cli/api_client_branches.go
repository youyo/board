package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIClientBranchesCmd は board api client_branches サブコマンドグループを返す。
func NewAPIClientBranchesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client_branches",
		Short: "顧客支社（client_branches）を操作する",
	}
	cmd.AddCommand(
		newAPIClientBranchesListCmd(),
		newAPIClientBranchesGetCmd(),
		newAPIClientBranchesSearchCmd(),
	)
	return cmd
}

func newAPIClientBranchesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "顧客支社の一覧を取得する",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.ListClientBranches(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
}

func newAPIClientBranchesGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "指定 ID の顧客支社を取得する",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id は必須です")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetClientBranch(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "顧客支社 ID（必須）")
	return cmd
}

func newAPIClientBranchesSearchCmd() *cobra.Command {
	var clientID int
	var name string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "顧客支社を条件で検索する",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			params := boardapi.ClientBranchSearchParams{
				ClientID: clientID,
				Name:     name,
			}
			result, err := svc.SearchClientBranches(cmd.Context(), params, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&clientID, "client-id", 0, "顧客 ID でフィルタ")
	cmd.Flags().StringVar(&name, "name", "", "支社名でフィルタ")
	return cmd
}
