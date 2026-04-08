package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/output"
)

// NewAPIContactsCmd は board api contacts サブコマンドグループを返す。
func NewAPIContactsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contacts",
		Short: "担当者（contacts）を操作する",
	}
	cmd.AddCommand(
		newAPIContactsListCmd(),
		newAPIContactsGetCmd(),
		newAPIContactsSearchCmd(),
	)
	return cmd
}

func newAPIContactsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "担当者の一覧を取得する",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.ListContacts(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
}

func newAPIContactsGetCmd() *cobra.Command {
	var id int
	cmd := &cobra.Command{
		Use:   "get",
		Short: "指定 ID の担当者を取得する",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == 0 {
				return fmt.Errorf("--id は必須です")
			}
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			result, err := svc.GetContact(cmd.Context(), id, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "担当者 ID（必須）")
	return cmd
}

func newAPIContactsSearchCmd() *cobra.Command {
	var clientID int
	var name, email string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "担当者を条件で検索する",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := apiServiceFromCmd(cmd)
			if err != nil {
				return err
			}
			opts := readOptionsFromCmd(cmd)
			params := boardapi.ContactSearchParams{
				ClientID: clientID,
				Name:     name,
				Email:    email,
			}
			result, err := svc.SearchContacts(cmd.Context(), params, opts)
			if err != nil {
				return err
			}
			return output.Write(os.Stdout, result, prettyFromCmd(cmd))
		},
	}
	cmd.Flags().IntVar(&clientID, "client-id", 0, "顧客 ID でフィルタ")
	cmd.Flags().StringVar(&name, "name", "", "担当者名でフィルタ")
	cmd.Flags().StringVar(&email, "email", "", "メールアドレスでフィルタ")
	return cmd
}
