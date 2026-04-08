package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIInvoicesCmd は board api invoices サブコマンドグループを返す。
func NewAPIInvoicesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "invoices",
		Short: "請求書（invoices）を操作する",
	}
	cmd.AddCommand(
		newAPIInvoicesListCmd(),
		newAPIInvoicesGetCmd(),
		newAPIInvoicesSearchCmd(),
	)
	return cmd
}

func newAPIInvoicesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "請求書の一覧を取得する",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.ListInvoices(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
}

func newAPIInvoicesGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "指定 ID の請求書を取得する",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id は必須です")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetInvoice(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "請求書 ID（必須）")
	return cmd
}

func newAPIInvoicesSearchCmd() *cobra.Command {
	var clientID int
	var projectID int
	var status, updatedAtFrom string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "請求書を条件で検索する",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			params := boardapi.InvoiceSearchParams{
				ClientID:      clientID,
				ProjectID:     projectID,
				Status:        status,
				UpdatedAtFrom: updatedAtFrom,
			}
			result, err := svc.SearchInvoices(cmd.Context(), params, opts)
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
