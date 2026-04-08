package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIPaymentsCmd は board api payments サブコマンドグループを返す。
func NewAPIPaymentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "payments",
		Short: "支払（payments）を操作する",
	}
	cmd.AddCommand(
		newAPIPaymentsListCmd(),
		newAPIPaymentsGetCmd(),
		newAPIPaymentsSearchCmd(),
	)
	return cmd
}

func newAPIPaymentsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "支払の一覧を取得する",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.ListPayments(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
}

func newAPIPaymentsGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "指定 ID の支払を取得する",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id は必須です")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetPayment(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "支払 ID（必須）")
	return cmd
}

func newAPIPaymentsSearchCmd() *cobra.Command {
	var vendorID, purchaseOrderID int
	var status, updatedAtFrom string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "支払を条件で検索する",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			params := boardapi.PaymentSearchParams{
				VendorID:        vendorID,
				PurchaseOrderID: purchaseOrderID,
				Status:          status,
				UpdatedAtFrom:   updatedAtFrom,
			}
			result, err := svc.SearchPayments(cmd.Context(), params, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&vendorID, "vendor-id", 0, "発注先 ID でフィルタ")
	cmd.Flags().IntVar(&purchaseOrderID, "purchase-order-id", 0, "発注書 ID でフィルタ")
	cmd.Flags().StringVar(&status, "status", "", "ステータスでフィルタ")
	cmd.Flags().StringVar(&updatedAtFrom, "updated-at-from", "", "更新日時（ISO 8601）以降でフィルタ")
	return cmd
}
