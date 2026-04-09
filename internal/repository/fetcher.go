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

// maybeRefresh determines whether a refresh is needed and executes it if so.
// If ForceRefresh is true, ForceRefresh takes priority.
// If Refresh is true or autoRefresh and NeedsDailyRefresh, DeltaRefresh is performed.
// On DeltaRefresh error, stale data is returned (logged only).
// On ForceRefresh error, the error is propagated.
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
			// DeltaRefresh failure returns stale data (log only)
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

// clientBranchesFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
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

// ListUpdatedSince: ClientBranchSearchParams has no UpdatedAtFrom, so all entries are fetched.
func (f *clientBranchesFetcher) ListUpdatedSince(ctx context.Context, _ string) ([]json.RawMessage, error) {
	entities, err := f.api.ListClientBranches(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- contacts Fetcher ---

// contactsFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
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

// ListUpdatedSince: ContactSearchParams has no UpdatedAtFrom, so all entries are fetched.
func (f *contactsFetcher) ListUpdatedSince(ctx context.Context, _ string) ([]json.RawMessage, error) {
	entities, err := f.api.ListContacts(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- projects Fetcher ---

// projectsFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
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

// projectCostsFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
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

// ListUpdatedSince: ProjectCostSearchParams has no UpdatedAtFrom, so all entries are fetched.
func (f *projectCostsFetcher) ListUpdatedSince(ctx context.Context, _ string) ([]json.RawMessage, error) {
	entities, err := f.api.ListProjectCosts(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- invoices Fetcher ---

// invoicesFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
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

// --- vendors Fetcher ---

// vendorsFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
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

// vendorBranchesFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
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

// ListUpdatedSince: VendorBranchSearchParams has no UpdatedAtFrom, so all entries are fetched.
func (f *vendorBranchesFetcher) ListUpdatedSince(ctx context.Context, _ string) ([]json.RawMessage, error) {
	entities, err := f.api.ListVendorBranches(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- vendor_contacts Fetcher ---

// vendorContactsFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
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

// ListUpdatedSince: VendorContactSearchParams has no UpdatedAtFrom, so all entries are fetched.
func (f *vendorContactsFetcher) ListUpdatedSince(ctx context.Context, _ string) ([]json.RawMessage, error) {
	entities, err := f.api.ListVendorContacts(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- users Fetcher ---

// usersFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type usersFetcher struct {
	api *boardapi.Client
}

func (f *usersFetcher) ResourceName() string { return "users" }

func (f *usersFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

func (f *usersFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	params := boardapi.UserSearchParams{UpdatedAtFrom: since}
	entities, err := f.api.SearchUsers(ctx, params)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- groups Fetcher ---

// groupsFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type groupsFetcher struct {
	api *boardapi.Client
}

func (f *groupsFetcher) ResourceName() string { return "groups" }

func (f *groupsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListGroups(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

func (f *groupsFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	params := boardapi.GroupSearchParams{UpdatedAtFrom: since}
	entities, err := f.api.SearchGroups(ctx, params)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- payment_terms Fetcher ---

// paymentTermsFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type paymentTermsFetcher struct {
	api *boardapi.Client
}

func (f *paymentTermsFetcher) ResourceName() string { return "payment_terms" }

func (f *paymentTermsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListPaymentTerms(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

func (f *paymentTermsFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	params := boardapi.PaymentTermSearchParams{UpdatedAtFrom: since}
	entities, err := f.api.SearchPaymentTerms(ctx, params)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- project_types Fetcher ---

// projectTypesFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type projectTypesFetcher struct {
	api *boardapi.Client
}

func (f *projectTypesFetcher) ResourceName() string { return "project_types" }

func (f *projectTypesFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListProjectTypes(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

func (f *projectTypesFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	params := boardapi.ProjectTypeSearchParams{UpdatedAtFrom: since}
	entities, err := f.api.SearchProjectTypes(ctx, params)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- purchase_types Fetcher ---

// purchaseTypesFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type purchaseTypesFetcher struct {
	api *boardapi.Client
}

func (f *purchaseTypesFetcher) ResourceName() string { return "purchase_types" }

func (f *purchaseTypesFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListPurchaseTypes(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

func (f *purchaseTypesFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	params := boardapi.PurchaseTypeSearchParams{UpdatedAtFrom: since}
	entities, err := f.api.SearchPurchaseTypes(ctx, params)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- accounting_types Fetcher ---

// accountingTypesFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type accountingTypesFetcher struct {
	api *boardapi.Client
}

func (f *accountingTypesFetcher) ResourceName() string { return "accounting_types" }

func (f *accountingTypesFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListAccountingTypes(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

func (f *accountingTypesFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	params := boardapi.AccountingTypeSearchParams{UpdatedAtFrom: since}
	entities, err := f.api.SearchAccountingTypes(ctx, params)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- document_send_channels Fetcher ---

// documentSendChannelsFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
type documentSendChannelsFetcher struct {
	api *boardapi.Client
}

func (f *documentSendChannelsFetcher) ResourceName() string { return "document_send_channels" }

func (f *documentSendChannelsFetcher) ListAll(ctx context.Context) ([]json.RawMessage, error) {
	entities, err := f.api.ListDocumentSendChannels(ctx)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

func (f *documentSendChannelsFetcher) ListUpdatedSince(ctx context.Context, since string) ([]json.RawMessage, error) {
	params := boardapi.DocumentSendChannelSearchParams{UpdatedAtFrom: since}
	entities, err := f.api.SearchDocumentSendChannels(ctx, params)
	if err != nil {
		return nil, err
	}
	return entitiesToRaw(entities)
}

// --- purchase_orders Fetcher ---

// purchaseOrdersFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
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

// paymentsFetcher is an adapter that makes boardapi.Client conform to refresh.Fetcher.
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
