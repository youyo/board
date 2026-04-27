package cli

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/app"
	"github.com/youyo/board/internal/cache"
	"github.com/youyo/board/internal/service/find"
)

// findResourceMap は各 find サブコマンドが触りうる resource 名を返す。
// レスポンスの cache 配列は LLM が --refresh / --refresh-full の判断をする
// ための材料として、find が依存する resource ごとに cached_at /
// full_refreshed_at を返す。
var findResourceMap = map[string][]string{
	"client":         {"clients", "client_branches", "contacts"},
	"vendor":         {"vendors", "vendor_branches", "vendor_contacts"},
	"user":           {"users"},
	"project":        {"projects", "clients"},
	"estimate":       {"clients", "projects", "estimates"},
	"invoice":        {"invoices", "clients", "projects"},
	"order":          {"clients", "projects", "orders"},
	"delivery":       {"clients", "projects", "deliveries"},
	"receipt":        {"clients", "projects", "receipts"},
	"purchase_order": {"purchase_orders", "vendors", "projects"},
	"payment":        {"payments", "vendors"},
}

// FindOutput は find 系コマンドの統一レスポンス形式。
// LLM がキャッシュの新鮮度を判断できるよう、items と並列に cache 配列を同梱する。
type FindOutput[T any] struct {
	Items []T          `json:"items"`
	Cache []cache.Info `json:"cache"`
}

// wrapFindOutput は find 結果と該当 resource の cache info をラップする。
// items が nil の場合は空配列で表現する（JSON null を避ける）。
// cmd.Context() に App が居ない場合（unit test 等）は cache を空配列で返す。
func wrapFindOutput[T any](ctx context.Context, kind string, items []T) FindOutput[T] {
	if items == nil {
		items = []T{}
	}
	out := FindOutput[T]{Items: items, Cache: []cache.Info{}}
	resources, ok := findResourceMap[kind]
	if !ok {
		return out
	}
	a, ok := app.AppFromContext(ctx)
	if !ok || a == nil || a.SyncStore == nil {
		return out
	}
	out.Cache = cache.LoadInfos(ctx, a.SyncStore, a.ProfileName, resources)
	return out
}

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
	)

	return cmd
}

// findServiceFromCmd creates a find.Service from the App in context.
func findServiceFromCmd(cmd *cobra.Command) (*find.Service, error) {
	a, ok := app.AppFromContext(cmd.Context())
	if !ok {
		return nil, errNoApp
	}
	return a.FindService(), nil
}
