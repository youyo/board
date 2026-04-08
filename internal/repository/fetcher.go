package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"strconv"
	"time"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/cache"
	"github.com/youyo/board/internal/refresh"
)

// entitiesToRaw は任意のエンティティスライスを []json.RawMessage に変換する。
func entitiesToRaw[T any](entities []T) ([]json.RawMessage, error) {
	result := make([]json.RawMessage, 0, len(entities))
	for _, e := range entities {
		raw, err := json.Marshal(e)
		if err != nil {
			return nil, err
		}
		result = append(result, raw)
	}
	return result, nil
}

// decodeEntries は []cache.Entry を []T に変換する。
func decodeEntries[T any](entries []cache.Entry) ([]T, error) {
	result := make([]T, 0, len(entries))
	for _, e := range entries {
		var entity T
		if err := json.Unmarshal(e.PayloadJSON, &entity); err != nil {
			return nil, err
		}
		result = append(result, entity)
	}
	return result, nil
}

// upsertRaw は json.RawMessage をキャッシュに upsert する。
// id と updated_at を raw から抽出する。
func upsertRaw(ctx context.Context, rc *cache.ResourceCache, profile, resource string, raw json.RawMessage) error {
	var idHolder struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(raw, &idHolder); err != nil {
		return err
	}
	var updatedAtHolder struct {
		UpdatedAt string `json:"updated_at"`
	}
	_ = json.Unmarshal(raw, &updatedAtHolder)

	var updatedAt sql.NullString
	if updatedAtHolder.UpdatedAt != "" {
		updatedAt = sql.NullString{String: updatedAtHolder.UpdatedAt, Valid: true}
	}

	return rc.Upsert(ctx, cache.Entry{
		Key: cache.EntityKey{
			Profile:  profile,
			Resource: resource,
			EntityID: strconv.Itoa(idHolder.ID),
		},
		PayloadJSON: raw,
		UpdatedAt:   updatedAt,
	})
}

// maybeRefresh はリフレッシュ要否を判定し、必要なら実行する。
// ForceRefresh が true なら ForceRefresh を優先。
// Refresh が true または autoRefresh かつ NeedsDailyRefresh なら DeltaRefresh。
// DeltaRefresh エラー時は stale 返却（ログのみ）。
// ForceRefresh エラー時はエラーを伝播。
func maybeRefresh(
	ctx context.Context,
	profile, resource string,
	opts ReadOptions,
	state *cache.SyncState,
	autoRefresh bool,
	tz *time.Location,
	lm *refresh.LockManager,
	refresher *refresh.Refresher,
	fetcher refresh.Fetcher,
	now time.Time,
) error {
	switch {
	case opts.ForceRefresh:
		return lm.WithLock(ctx, profile, resource, func() error {
			_, err := refresher.ForceRefresh(ctx, profile, fetcher, now, tz)
			return err
		})
	case opts.Refresh || (autoRefresh && refresh.NeedsDailyRefresh(state, now, tz)):
		err := lm.WithLock(ctx, profile, resource, func() error {
			_, err := refresher.DeltaRefresh(ctx, profile, fetcher, now, tz)
			return err
		})
		if opts.Refresh && err != nil {
			// DeltaRefresh 失敗は stale 返却（ログのみ）
			slog.Warn("DeltaRefresh failed, returning stale cache",
				"profile", profile,
				"resource", resource,
				"error", err,
			)
			return nil
		}
		return err
	}
	return nil
}

// --- clients Fetcher ---

// clientsFetcher は boardapi.Client を refresh.Fetcher に適合させるアダプタ。
type clientsFetcher struct {
	api *boardapi.Client
}

func (f *clientsFetcher) ResourceName() string { return "clients" }

func (f *clientsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListClients(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

func (f *clientsFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	params := boardapi.ClientSearchParams{UpdatedAtFrom: since}
	entities, err := f.api.SearchClients(ctx, params)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- client_branches Fetcher ---

// clientBranchesFetcher は boardapi.Client を refresh.Fetcher に適合させるアダプタ。
type clientBranchesFetcher struct {
	api *boardapi.Client
}

func (f *clientBranchesFetcher) ResourceName() string { return "client_branches" }

func (f *clientBranchesFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListClientBranches(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// ListUpdatedSince: ClientBranchSearchParams に UpdatedAtFrom がないため全件取得。
func (f *clientBranchesFetcher) ListUpdatedSince(ctx context.Context, _ string) ([]json.RawMessage, error) {
	entities, err := f.api.ListClientBranches(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- contacts Fetcher ---

// contactsFetcher は boardapi.Client を refresh.Fetcher に適合させるアダプタ。
type contactsFetcher struct {
	api *boardapi.Client
}

func (f *contactsFetcher) ResourceName() string { return "contacts" }

func (f *contactsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListContacts(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// ListUpdatedSince: ContactSearchParams に UpdatedAtFrom がないため全件取得。
func (f *contactsFetcher) ListUpdatedSince(ctx context.Context, _ string) ([]json.RawMessage, error) {
	entities, err := f.api.ListContacts(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- projects Fetcher ---

// projectsFetcher は boardapi.Client を refresh.Fetcher に適合させるアダプタ。
type projectsFetcher struct {
	api *boardapi.Client
}

func (f *projectsFetcher) ResourceName() string { return "projects" }

func (f *projectsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

func (f *projectsFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	params := boardapi.ProjectSearchParams{UpdatedAtFrom: since}
	entities, err := f.api.SearchProjects(ctx, params)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- project_costs Fetcher ---

// projectCostsFetcher は boardapi.Client を refresh.Fetcher に適合させるアダプタ。
type projectCostsFetcher struct {
	api *boardapi.Client
}

func (f *projectCostsFetcher) ResourceName() string { return "project_costs" }

func (f *projectCostsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListProjectCosts(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// ListUpdatedSince: ProjectCostSearchParams に UpdatedAtFrom がないため全件取得。
func (f *projectCostsFetcher) ListUpdatedSince(ctx context.Context, _ string) ([]json.RawMessage, error) {
	entities, err := f.api.ListProjectCosts(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- estimates Fetcher ---

// estimatesFetcher は boardapi.Client を refresh.Fetcher に適合させるアダプタ。
type estimatesFetcher struct {
	api *boardapi.Client
}

func (f *estimatesFetcher) ResourceName() string { return "estimates" }

func (f *estimatesFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListEstimates(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

func (f *estimatesFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	params := boardapi.EstimateSearchParams{UpdatedAtFrom: since}
	entities, err := f.api.SearchEstimates(ctx, params)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- invoices Fetcher ---

// invoicesFetcher は boardapi.Client を refresh.Fetcher に適合させるアダプタ。
type invoicesFetcher struct {
	api *boardapi.Client
}

func (f *invoicesFetcher) ResourceName() string { return "invoices" }

func (f *invoicesFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListInvoices(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

func (f *invoicesFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	params := boardapi.InvoiceSearchParams{UpdatedAtFrom: since}
	entities, err := f.api.SearchInvoices(ctx, params)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- orders Fetcher ---

// ordersFetcher は boardapi.Client を refresh.Fetcher に適合させるアダプタ。
type ordersFetcher struct {
	api *boardapi.Client
}

func (f *ordersFetcher) ResourceName() string { return "orders" }

func (f *ordersFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListOrders(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

func (f *ordersFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	params := boardapi.OrderSearchParams{UpdatedAtFrom: since}
	entities, err := f.api.SearchOrders(ctx, params)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- deliveries Fetcher ---

// deliveriesFetcher は boardapi.Client を refresh.Fetcher に適合させるアダプタ。
type deliveriesFetcher struct {
	api *boardapi.Client
}

func (f *deliveriesFetcher) ResourceName() string { return "deliveries" }

func (f *deliveriesFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListDeliveries(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

func (f *deliveriesFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	params := boardapi.DeliverySearchParams{UpdatedAtFrom: since}
	entities, err := f.api.SearchDeliveries(ctx, params)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- receipts Fetcher ---

// receiptsFetcher は boardapi.Client を refresh.Fetcher に適合させるアダプタ。
type receiptsFetcher struct {
	api *boardapi.Client
}

func (f *receiptsFetcher) ResourceName() string { return "receipts" }

func (f *receiptsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListReceipts(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

func (f *receiptsFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	params := boardapi.ReceiptSearchParams{UpdatedAtFrom: since}
	entities, err := f.api.SearchReceipts(ctx, params)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- vendors Fetcher ---

// vendorsFetcher は boardapi.Client を refresh.Fetcher に適合させるアダプタ。
type vendorsFetcher struct {
	api *boardapi.Client
}

func (f *vendorsFetcher) ResourceName() string { return "vendors" }

func (f *vendorsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListVendors(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

func (f *vendorsFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	params := boardapi.VendorSearchParams{UpdatedAtFrom: since}
	entities, err := f.api.SearchVendors(ctx, params)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- vendor_branches Fetcher ---

// vendorBranchesFetcher は boardapi.Client を refresh.Fetcher に適合させるアダプタ。
type vendorBranchesFetcher struct {
	api *boardapi.Client
}

func (f *vendorBranchesFetcher) ResourceName() string { return "vendor_branches" }

func (f *vendorBranchesFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListVendorBranches(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// ListUpdatedSince: VendorBranchSearchParams に UpdatedAtFrom がないため全件取得。
func (f *vendorBranchesFetcher) ListUpdatedSince(ctx context.Context, _ string) ([]json.RawMessage, error) {
	entities, err := f.api.ListVendorBranches(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- vendor_contacts Fetcher ---

// vendorContactsFetcher は boardapi.Client を refresh.Fetcher に適合させるアダプタ。
type vendorContactsFetcher struct {
	api *boardapi.Client
}

func (f *vendorContactsFetcher) ResourceName() string { return "vendor_contacts" }

func (f *vendorContactsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListVendorContacts(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// ListUpdatedSince: VendorContactSearchParams に UpdatedAtFrom がないため全件取得。
func (f *vendorContactsFetcher) ListUpdatedSince(ctx context.Context, _ string) ([]json.RawMessage, error) {
	entities, err := f.api.ListVendorContacts(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- purchase_orders Fetcher ---

// purchaseOrdersFetcher は boardapi.Client を refresh.Fetcher に適合させるアダプタ。
type purchaseOrdersFetcher struct {
	api *boardapi.Client
}

func (f *purchaseOrdersFetcher) ResourceName() string { return "purchase_orders" }

func (f *purchaseOrdersFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListPurchaseOrders(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

func (f *purchaseOrdersFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	params := boardapi.PurchaseOrderSearchParams{UpdatedAtFrom: since}
	entities, err := f.api.SearchPurchaseOrders(ctx, params)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- payments Fetcher ---

// paymentsFetcher は boardapi.Client を refresh.Fetcher に適合させるアダプタ。
type paymentsFetcher struct {
	api *boardapi.Client
}

func (f *paymentsFetcher) ResourceName() string { return "payments" }

func (f *paymentsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListPayments(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

func (f *paymentsFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	params := boardapi.PaymentSearchParams{UpdatedAtFrom: since}
	entities, err := f.api.SearchPayments(ctx, params)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}
