package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIPurchaseOrdersCmd は board api purchase_orders サブコマンドグループを返す。
func NewAPIPurchaseOrdersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "purchase_orders",
		Short: "発注書（purchase_orders）を操作する",
	}
	cmd.AddCommand(
		newAPIPurchaseOrdersListCmd(),
		newAPIPurchaseOrdersGetCmd(),
		newAPIPurchaseOrdersSearchCmd(),
	)
	return cmd
}

func newAPIPurchaseOrdersListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "発注書の一覧を取得する",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.ListPurchaseOrders(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
}

func newAPIPurchaseOrdersGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "指定 ID の発注書を取得する",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id は必須です")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetPurchaseOrder(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "発注書 ID（必須）")
	return cmd
}

func newAPIPurchaseOrdersSearchCmd() *cobra.Command {
	var vendorID, projectID int
	var status, updatedAtFrom string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "発注書を条件で検索する",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			params := boardapi.PurchaseOrderSearchParams{
				VendorID:      vendorID,
				ProjectID:     projectID,
				Status:        status,
				UpdatedAtFrom: updatedAtFrom,
			}
			result, err := svc.SearchPurchaseOrders(cmd.Context(), params, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&vendorID, "vendor-id", 0, "発注先 ID でフィルタ")
	cmd.Flags().IntVar(&projectID, "project-id", 0, "案件 ID でフィルタ")
	cmd.Flags().StringVar(&status, "status", "", "ステータスでフィルタ")
	cmd.Flags().StringVar(&updatedAtFrom, "updated-at-from", "", "更新日時（ISO 8601）以降でフィルタ")
	return cmd
}
