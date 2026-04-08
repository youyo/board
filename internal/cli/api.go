package cli

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/app"
	"github.com/youyo/board/internal/repository"
	serviceapi "github.com/youyo/board/internal/service/api"
)

// errNoApp は Context に App が格納されていない場合のエラー。
var errNoApp = errors.New("board: app not found in context")

// NewAPICmd は board api サブコマンドグループを返す。
func NewAPICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api",
		Short: "BOARD API を直接操作する（low-level）",
		Long:  "BOARD API のエンドポイントを直接呼び出す低レベルコマンド群。",
	}

	cmd.AddCommand(
		NewAPIClientsCmd(),
		NewAPIClientBranchesCmd(),
		NewAPIContactsCmd(),
		NewAPIProjectsCmd(),
		NewAPIProjectCostsCmd(),
	)

	return cmd
}

// apiServiceFromCmd は AppFromContext から api.Service を生成して返す。
func apiServiceFromCmd(cmd *cobra.Command) (*serviceapi.Service, error) {
	a, ok := app.AppFromContext(cmd.Context())
	if !ok {
		return nil, errNoApp
	}
	repos := a.Repos
	svc := serviceapi.New(serviceapi.Repos{
		Clients:              repos.Clients,
		ClientBranches:       repos.ClientBranches,
		Contacts:             repos.Contacts,
		Projects:             repos.Projects,
		ProjectCosts:         repos.ProjectCosts,
		Estimates:            repos.Estimates,
		Invoices:             repos.Invoices,
		Orders:               repos.Orders,
		Deliveries:           repos.Deliveries,
		Receipts:             repos.Receipts,
		Vendors:              repos.Vendors,
		VendorBranches:       repos.VendorBranches,
		VendorContacts:       repos.VendorContacts,
		PurchaseOrders:       repos.PurchaseOrders,
		Payments:             repos.Payments,
		Users:                repos.Users,
		Groups:               repos.Groups,
		PaymentTerms:         repos.PaymentTerms,
		ProjectTypes:         repos.ProjectTypes,
		PurchaseTypes:        repos.PurchaseTypes,
		AccountingTypes:      repos.AccountingTypes,
		DocumentSendChannels: repos.DocumentSendChannels,
	})
	return svc, nil
}

// readOptionsFromCmd は PersistentFlags から ReadOptions を組み立てる。
func readOptionsFromCmd(cmd *cobra.Command) repository.ReadOptions {
	refresh, _ := cmd.Root().PersistentFlags().GetBool("refresh")
	forceRefresh, _ := cmd.Root().PersistentFlags().GetBool("force-refresh")
	limit, _ := cmd.Root().PersistentFlags().GetInt("limit")
	return repository.ReadOptions{
		Refresh:      refresh,
		ForceRefresh: forceRefresh,
		Limit:        limit,
	}
}

// prettyFromCmd は --pretty フラグの値を返す。
func prettyFromCmd(cmd *cobra.Command) bool {
	v, _ := cmd.Root().PersistentFlags().GetBool("pretty")
	return v
}
