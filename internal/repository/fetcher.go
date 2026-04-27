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

// entitiesToRaw converts a slice of entities of any type to []json.RawMessage.
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

// decodeEntries converts []cache.Entry to []T.
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

// upsertRaw upserts a json.RawMessage into the cache.
// It extracts id and updated_at from raw.
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

// refreshRetryAfterSeconds は同時 refresh 競合時の retry-after 推奨値（固定）。
const refreshRetryAfterSeconds = 30

// maybeRefresh determines whether a refresh is needed and executes it if so.
// 排他は TryWithLock で行い、ロック競合時は *refresh.RefreshInProgressError を即返却する。
//
//   - ForceRefresh / Refresh どちらも指定されていない場合は no-op（auto refresh は廃止）。
//   - ForceRefresh は full refresh を実行、エラーは伝播。
//   - Refresh (delta) はエラー時 stale cache を返す（log only）。
func maybeRefresh(
	ctx context.Context,
	profile, resource string,
	opts ReadOptions,
	_ *cache.SyncState, // state は将来の判断材料用に残す（現状未使用）
	_ bool,
	tz *time.Location,
	lm *refresh.LockManager,
	refresher *refresh.Refresher,
	fetcher refresh.Fetcher,
	now time.Time,
) error {
	switch {
	case opts.ForceRefresh:
		return lm.TryWithLock(ctx, profile, resource, refreshRetryAfterSeconds, func() error {
			_, err := refresher.ForceRefresh(ctx, profile, fetcher, now, tz)
			return err
		})
	case opts.Refresh:
		err := lm.TryWithLock(ctx, profile, resource, refreshRetryAfterSeconds, func() error {
			_, err := refresher.DeltaRefresh(ctx, profile, fetcher, now, tz)
			return err
		})
		if err != nil {
			// 同時実行競合は呼び出し元で 429 として返したいので伝播。
			if refresh.IsRefreshInProgress(err) {
				return err
			}
			// それ以外（API エラー等）は stale cache を返す（log only）。
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

// clientsFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type clientsFetcher struct {
	api *boardapi.Client
}

func (f *clientsFetcher) ResourceName() string { return "clients" }

func (f *clientsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	result, err := f.api.ListClients(ctx, boardapi.ClientListOptions{})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

func (f *clientsFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	// since は cache.SyncState.CursorUpdatedAt 由来の ISO 8601（BOARD API の
	// エンティティ updated_at）。BOARD API の Ransack `_gteq` は
	// `YYYY-MM-DD HH:MM:SS` 形式を期待するため、ここで変換する。
	// 空文字はそのまま渡し（filter 無効化）、全件取得。
	gteq := isoToBoardDateTime(since)
	result, err := f.api.ListClients(ctx, boardapi.ClientListOptions{UpdatedAtGteq: gteq})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

// isoToBoardDateTime converts an ISO 8601 datetime string (e.g.
// "2024-12-26T10:14:11.000+09:00") to the BOARD API Ransack `_gteq`
// compatible form "YYYY-MM-DD HH:MM:SS" in the original timezone.
// Empty input is returned as-is to signal "no filter".
//
// The function accepts common ISO 8601 variants. Unparseable input is
// returned unchanged as a best-effort fallback; the server may reject it
// but the alternative (silently dropping the cursor) would cause wholesale
// refetch — explicit error feedback from the server is preferable.
func isoToBoardDateTime(s string) string {
	if s == "" {
		return ""
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000-07:00",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02 15:04:05")
		}
	}
	return s
}

// --- client_branches Fetcher ---

// clientBranchesFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type clientBranchesFetcher struct {
	api *boardapi.Client
}

func (f *clientBranchesFetcher) ResourceName() string { return "client_branches" }

func (f *clientBranchesFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	result, err := f.api.ListClientBranches(ctx, boardapi.ClientBranchListOptions{})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

func (f *clientBranchesFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	// since は cache.SyncState.CursorUpdatedAt 由来の ISO 8601（BOARD API の
	// エンティティ updated_at）。BOARD API の Ransack `_gteq` は
	// `YYYY-MM-DD HH:MM:SS` 形式を期待するため、ここで変換する。
	gteq := isoToBoardDateTime(since)
	result, err := f.api.ListClientBranches(ctx, boardapi.ClientBranchListOptions{UpdatedAtGteq: gteq})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

// --- contacts Fetcher ---

// contactsFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type contactsFetcher struct {
	api *boardapi.Client
}

func (f *contactsFetcher) ResourceName() string { return "contacts" }

func (f *contactsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	result, err := f.api.ListContacts(ctx, boardapi.ContactListOptions{})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

func (f *contactsFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	// since は cache.SyncState.CursorUpdatedAt 由来の ISO 8601（BOARD API の
	// エンティティ updated_at）。BOARD API の Ransack `_gteq` は
	// `YYYY-MM-DD HH:MM:SS` 形式を期待するため、ここで変換する。
	gteq := isoToBoardDateTime(since)
	result, err := f.api.ListContacts(ctx, boardapi.ContactListOptions{UpdatedAtGteq: gteq})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

// --- projects Fetcher ---

// projectsFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type projectsFetcher struct {
	api *boardapi.Client
}

func (f *projectsFetcher) ResourceName() string { return "projects" }

func (f *projectsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	result, err := f.api.ListProjects(ctx, boardapi.ProjectListOptions{})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

func (f *projectsFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	// since は cache.SyncState.CursorUpdatedAt 由来の ISO 8601（BOARD API の
	// エンティティ updated_at）。BOARD API の Ransack `_gteq` は
	// `YYYY-MM-DD HH:MM:SS` 形式を期待するため、ここで変換する。
	gteq := isoToBoardDateTime(since)
	result, err := f.api.ListProjects(ctx, boardapi.ProjectListOptions{UpdatedAtGteq: gteq})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

// --- project_costs Fetcher ---

// projectCostsFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type projectCostsFetcher struct {
	api *boardapi.Client
}

func (f *projectCostsFetcher) ResourceName() string { return "project_costs" }

func (f *projectCostsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	result, err := f.api.ListProjectCosts(ctx, boardapi.ProjectCostListOptions{})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

func (f *projectCostsFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	// since は cache.SyncState.CursorUpdatedAt 由来の ISO 8601（BOARD API の
	// エンティティ updated_at）。BOARD API の Ransack `_gteq` は
	// `YYYY-MM-DD HH:MM:SS` 形式を期待するため、ここで変換する。
	gteq := isoToBoardDateTime(since)
	result, err := f.api.ListProjectCosts(ctx, boardapi.ProjectCostListOptions{UpdatedAtGteq: gteq})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

// --- invoices Fetcher ---

// invoicesFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type invoicesFetcher struct {
	api *boardapi.Client
}

func (f *invoicesFetcher) ResourceName() string { return "invoices" }

func (f *invoicesFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	result, err := f.api.ListInvoices(ctx, boardapi.InvoiceListOptions{})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

func (f *invoicesFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	gteq := isoToBoardDateTime(since)
	result, err := f.api.ListInvoices(ctx, boardapi.InvoiceListOptions{UpdatedAtGteq: gteq})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

// --- vendors Fetcher ---

// vendorsFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type vendorsFetcher struct {
	api *boardapi.Client
}

func (f *vendorsFetcher) ResourceName() string { return "vendors" }

func (f *vendorsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	result, err := f.api.ListVendors(ctx, boardapi.VendorListOptions{})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

func (f *vendorsFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	// since は cache.SyncState.CursorUpdatedAt 由来の ISO 8601（BOARD API の
	// エンティティ updated_at）。BOARD API の Ransack `_gteq` は
	// `YYYY-MM-DD HH:MM:SS` 形式を期待するため、ここで変換する。
	gteq := isoToBoardDateTime(since)
	result, err := f.api.ListVendors(ctx, boardapi.VendorListOptions{UpdatedAtGteq: gteq})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

// --- vendor_branches Fetcher ---

// vendorBranchesFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type vendorBranchesFetcher struct {
	api *boardapi.Client
}

func (f *vendorBranchesFetcher) ResourceName() string { return "vendor_branches" }

func (f *vendorBranchesFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	result, err := f.api.ListVendorBranches(ctx, boardapi.VendorBranchListOptions{})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

func (f *vendorBranchesFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	// since は cache.SyncState.CursorUpdatedAt 由来の ISO 8601（BOARD API の
	// エンティティ updated_at）。BOARD API の Ransack `_gteq` は
	// `YYYY-MM-DD HH:MM:SS` 形式を期待するため、ここで変換する。
	gteq := isoToBoardDateTime(since)
	result, err := f.api.ListVendorBranches(ctx, boardapi.VendorBranchListOptions{UpdatedAtGteq: gteq})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

// --- vendor_contacts Fetcher ---

// vendorContactsFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type vendorContactsFetcher struct {
	api *boardapi.Client
}

func (f *vendorContactsFetcher) ResourceName() string { return "vendor_contacts" }

func (f *vendorContactsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	result, err := f.api.ListVendorContacts(ctx, boardapi.VendorContactListOptions{})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

func (f *vendorContactsFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	// since は cache.SyncState.CursorUpdatedAt 由来の ISO 8601（BOARD API の
	// エンティティ updated_at）。BOARD API の Ransack `_gteq` は
	// `YYYY-MM-DD HH:MM:SS` 形式を期待するため、ここで変換する。
	gteq := isoToBoardDateTime(since)
	result, err := f.api.ListVendorContacts(ctx, boardapi.VendorContactListOptions{UpdatedAtGteq: gteq})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

// --- users Fetcher ---

// usersFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type usersFetcher struct {
	api *boardapi.Client
}

func (f *usersFetcher) ResourceName() string { return "users" }

func (f *usersFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	result, err := f.api.ListUsers(ctx, boardapi.UserListOptions{})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

func (f *usersFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	gteq := isoToBoardDateTime(since)
	result, err := f.api.ListUsers(ctx, boardapi.UserListOptions{UpdatedAtGteq: gteq})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

// --- groups Fetcher ---

// groupsFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type groupsFetcher struct {
	api *boardapi.Client
}

func (f *groupsFetcher) ResourceName() string { return "groups" }

func (f *groupsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	result, err := f.api.ListGroups(ctx, boardapi.GroupListOptions{})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

func (f *groupsFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	gteq := isoToBoardDateTime(since)
	result, err := f.api.ListGroups(ctx, boardapi.GroupListOptions{UpdatedAtGteq: gteq})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

// --- payment_terms Fetcher ---

// paymentTermsFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type paymentTermsFetcher struct {
	api *boardapi.Client
}

func (f *paymentTermsFetcher) ResourceName() string { return "payment_terms" }

func (f *paymentTermsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	result, err := f.api.ListPaymentTerms(ctx, boardapi.PaymentTermListOptions{})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

func (f *paymentTermsFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	gteq := isoToBoardDateTime(since)
	result, err := f.api.ListPaymentTerms(ctx, boardapi.PaymentTermListOptions{UpdatedAtGteq: gteq})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

// --- project_types Fetcher ---

// projectTypesFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type projectTypesFetcher struct {
	api *boardapi.Client
}

func (f *projectTypesFetcher) ResourceName() string { return "project_types" }

func (f *projectTypesFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	result, err := f.api.ListProjectTypes(ctx, boardapi.ProjectTypeListOptions{})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

func (f *projectTypesFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	gteq := isoToBoardDateTime(since)
	result, err := f.api.ListProjectTypes(ctx, boardapi.ProjectTypeListOptions{UpdatedAtGteq: gteq})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

// --- purchase_types Fetcher ---

// purchaseTypesFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type purchaseTypesFetcher struct {
	api *boardapi.Client
}

func (f *purchaseTypesFetcher) ResourceName() string { return "purchase_types" }

func (f *purchaseTypesFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	result, err := f.api.ListPurchaseTypes(ctx, boardapi.PurchaseTypeListOptions{})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

func (f *purchaseTypesFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	gteq := isoToBoardDateTime(since)
	result, err := f.api.ListPurchaseTypes(ctx, boardapi.PurchaseTypeListOptions{UpdatedAtGteq: gteq})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

// --- accounting_types Fetcher ---

// accountingTypesFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type accountingTypesFetcher struct {
	api *boardapi.Client
}

func (f *accountingTypesFetcher) ResourceName() string { return "accounting_types" }

func (f *accountingTypesFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	result, err := f.api.ListAccountingTypes(ctx, boardapi.AccountingTypeListOptions{})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

func (f *accountingTypesFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	gteq := isoToBoardDateTime(since)
	result, err := f.api.ListAccountingTypes(ctx, boardapi.AccountingTypeListOptions{UpdatedAtGteq: gteq})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

// --- document_send_channels Fetcher ---

// documentSendChannelsFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type documentSendChannelsFetcher struct {
	api *boardapi.Client
}

func (f *documentSendChannelsFetcher) ResourceName() string { return "document_send_channels" }

func (f *documentSendChannelsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	result, err := f.api.ListDocumentSendChannels(ctx, boardapi.DocumentSendChannelListOptions{})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

func (f *documentSendChannelsFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	gteq := isoToBoardDateTime(since)
	result, err := f.api.ListDocumentSendChannels(ctx, boardapi.DocumentSendChannelListOptions{UpdatedAtGteq: gteq})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

// --- purchase_orders Fetcher ---

// purchaseOrdersFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type purchaseOrdersFetcher struct {
	api *boardapi.Client
}

func (f *purchaseOrdersFetcher) ResourceName() string { return "purchase_orders" }

func (f *purchaseOrdersFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	result, err := f.api.ListPurchaseOrders(ctx, boardapi.PurchaseOrderListOptions{})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

func (f *purchaseOrdersFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	gteq := isoToBoardDateTime(since)
	result, err := f.api.ListPurchaseOrders(ctx, boardapi.PurchaseOrderListOptions{UpdatedAtGteq: gteq})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

// --- payments Fetcher ---

// paymentsFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type paymentsFetcher struct {
	api *boardapi.Client
}

func (f *paymentsFetcher) ResourceName() string { return "payments" }

func (f *paymentsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	result, err := f.api.ListPayments(ctx, boardapi.PaymentListOptions{})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}

func (f *paymentsFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	gteq := isoToBoardDateTime(since)
	result, err := f.api.ListPayments(ctx, boardapi.PaymentListOptions{UpdatedAtGteq: gteq})
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(result.Items)
}
