package cli

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/app"
	"github.com/youyo/board/internal/repository"
	serviceapi "github.com/youyo/board/internal/service/api"
)

// errNoApp is the error returned when App is not found in the context.
var errNoApp = errors.New("board: app not found in context")

// NewAPICmd returns the board api subcommand group.
func NewAPICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api",
		Short: "Access the BOARD API directly (low-level)",
		Long:  "Low-level commands that call BOARD API endpoints directly.",
	}

	cmd.AddCommand(
		NewAPIClientsCmd(),
		NewAPIClientBranchesCmd(),
		NewAPIContactsCmd(),
		NewAPIProjectsCmd(),
		NewAPIProjectCostsCmd(),
		NewAPIEstimatesCmd(),
		NewAPIInvoicesCmd(),
		NewAPIOrdersCmd(),
		NewAPIDeliveriesCmd(),
		NewAPIReceiptsCmd(),
		NewAPIVendorsCmd(),
		NewAPIVendorBranchesCmd(),
		NewAPIVendorContactsCmd(),
		NewAPIPurchaseOrdersCmd(),
		NewAPIPaymentsCmd(),
		NewAPIUsersCmd(),
		NewAPIGroupsCmd(),
		NewAPIPaymentTermsCmd(),
		NewAPIProjectTypesCmd(),
		NewAPIPurchaseTypesCmd(),
		NewAPIAccountingTypesCmd(),
		NewAPIDocumentSendChannelsCmd(),
	)

	return cmd
}

// apiServiceFromCmd creates and returns an api.Service from the App in context.
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

// readOptionsFromCmd builds ReadOptions from the persistent flags.
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

// prettyFromCmd returns the value of the --pretty flag.
func prettyFromCmd(cmd *cobra.Command) bool {
	v, _ := cmd.Root().PersistentFlags().GetBool("pretty")
	return v
}
