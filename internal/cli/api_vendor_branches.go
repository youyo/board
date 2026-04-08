package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIVendorBranchesCmd は board api vendor_branches サブコマンドグループを返す。
func NewAPIVendorBranchesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vendor_branches",
		Short: "発注先支社（vendor_branches）を操作する",
	}
	cmd.AddCommand(
		newAPIVendorBranchesListCmd(),
		newAPIVendorBranchesGetCmd(),
		newAPIVendorBranchesSearchCmd(),
	)
	return cmd
}

func newAPIVendorBranchesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "発注先支社の一覧を取得する",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.ListVendorBranches(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
}

func newAPIVendorBranchesGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "指定 ID の発注先支社を取得する",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id は必須です")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetVendorBranch(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "発注先支社 ID（必須）")
	return cmd
}

func newAPIVendorBranchesSearchCmd() *cobra.Command {
	var vendorID int
	var updatedAtFrom string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "発注先支社を条件で検索する",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			params := boardapi.VendorBranchSearchParams{
				VendorID:      vendorID,
				UpdatedAtFrom: updatedAtFrom,
			}
			result, err := svc.SearchVendorBranches(cmd.Context(), params, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&vendorID, "vendor-id", 0, "発注先 ID でフィルタ")
	cmd.Flags().StringVar(&updatedAtFrom, "updated-at-from", "", "更新日時（ISO 8601）以降でフィルタ")
	return cmd
}
