package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIVendorContactsCmd は board api vendor_contacts サブコマンドグループを返す。
func NewAPIVendorContactsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vendor_contacts",
		Short: "発注先担当者（vendor_contacts）を操作する",
	}
	cmd.AddCommand(
		newAPIVendorContactsListCmd(),
		newAPIVendorContactsGetCmd(),
		newAPIVendorContactsSearchCmd(),
	)
	return cmd
}

func newAPIVendorContactsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "発注先担当者の一覧を取得する",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.ListVendorContacts(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
}

func newAPIVendorContactsGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "指定 ID の発注先担当者を取得する",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id は必須です")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetVendorContact(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "発注先担当者 ID（必須）")
	return cmd
}

func newAPIVendorContactsSearchCmd() *cobra.Command {
	var vendorID int
	var name, updatedAtFrom string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "発注先担当者を条件で検索する",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			params := boardapi.VendorContactSearchParams{
				VendorID:      vendorID,
				Name:          name,
				UpdatedAtFrom: updatedAtFrom,
			}
			result, err := svc.SearchVendorContacts(cmd.Context(), params, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&vendorID, "vendor-id", 0, "発注先 ID でフィルタ")
	cmd.Flags().StringVar(&name, "name", "", "担当者名でフィルタ")
	cmd.Flags().StringVar(&updatedAtFrom, "updated-at-from", "", "更新日時（ISO 8601）以降でフィルタ")
	return cmd
}
