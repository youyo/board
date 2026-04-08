package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIOrdersCmd は board api orders サブコマンドグループを返す。
func NewAPIOrdersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orders",
		Short: "受注（orders）を操作する",
	}
	cmd.AddCommand(
		newAPIOrdersListCmd(),
		newAPIOrdersGetCmd(),
		newAPIOrdersSearchCmd(),
	)
	return cmd
}

func newAPIOrdersListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "受注の一覧を取得する",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.ListOrders(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
}

func newAPIOrdersGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "指定 ID の受注を取得する",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id は必須です")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetOrder(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "受注 ID（必須）")
	return cmd
}

func newAPIOrdersSearchCmd() *cobra.Command {
	var clientID int
	var projectID int
	var status, updatedAtFrom string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "受注を条件で検索する",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			params := boardapi.OrderSearchParams{
				ClientID:      clientID,
				ProjectID:     projectID,
				Status:        status,
				UpdatedAtFrom: updatedAtFrom,
			}
			result, err := svc.SearchOrders(cmd.Context(), params, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&clientID, "client-id", 0, "顧客 ID でフィルタ")
	cmd.Flags().IntVar(&projectID, "project-id", 0, "案件 ID でフィルタ")
	cmd.Flags().StringVar(&status, "status", "", "ステータスでフィルタ")
	cmd.Flags().StringVar(&updatedAtFrom, "updated-at-from", "", "更新日時（ISO 8601）以降でフィルタ")
	return cmd
}
