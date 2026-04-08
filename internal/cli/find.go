package cli

import (
	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/app"
	"github.com/youyo/board/internal/service/find"
)

// NewFindCmd returns the board find subcommand group.
func NewFindCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "find",
		Short: "High-level cross-resource search (LLM-friendly)",
		Long:  "High-level search commands that combine multiple BOARD API resources for convenient lookups.",
	}

	cmd.AddCommand(
		NewFindClientCmd(),
		NewFindProjectCmd(),
		NewFindEstimateCmd(),
		NewFindInvoiceCmd(),
		NewFindOrderCmd(),
		NewFindDeliveryCmd(),
		NewFindReceiptCmd(),
		NewFindVendorCmd(),
		NewFindPurchaseOrderCmd(),
		NewFindPaymentCmd(),
		NewFindUserCmd(),
		NewFindGroupCmd(),
	)

	return cmd
}

// findServiceFromCmd creates a find.Service from the App in context.
func findServiceFromCmd(cmd *cobra.Command) (*find.Service, error) {
	a, ok := app.AppFromContext(cmd.Context())
	if !ok {
		return nil, errNoApp
	}
	repos := a.Repos
	svc := find.New(find.Repos{
		Clients:        repos.Clients,
		ClientBranches: repos.ClientBranches,
		Contacts:       repos.Contacts,
		Projects:       repos.Projects,
		Estimates:      repos.Estimates,
		Invoices:       repos.Invoices,
		Orders:         repos.Orders,
		Deliveries:     repos.Deliveries,
		Receipts:       repos.Receipts,
		Vendors:        repos.Vendors,
		VendorBranches: repos.VendorBranches,
		VendorContacts: repos.VendorContacts,
		PurchaseOrders: repos.PurchaseOrders,
		Payments:       repos.Payments,
		Users:          repos.Users,
		Groups:         repos.Groups,
	})
	return svc, nil
}
