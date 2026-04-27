package find

import (
	"context"
	"log/slog"
	"testing"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// --- テストコンテキスト ---

var testCtx = context.Background()

// --- Assertion helpers ---

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- Stub 実装 ---
// ClientBranchRepo / ContactRepo / VendorBranchRepo / VendorContactRepo は
// Search のみが interface 要件だが、stub は将来の拡張に備えて GetByID も持つ（余剰メソッドは無害）。

type stubClientRepo struct {
	getResult    *boardapi.ClientEntity
	searchResult []boardapi.ClientEntity
	err          error
	searchErr    error // Search 専用エラー（GetByID は err を使い、Search は searchErr を使う）
	searchCount  int   // Search 呼び出し回数カウンター（T06 で使用）
	getCount     int   // GetByID 呼び出し回数カウンター（N05-T02/T08 で使用）
	// getFunc が非 nil の場合、GetByID はこの関数を呼ぶ（ctx-aware テスト用）。
	getFunc func(ctx context.Context) (*boardapi.ClientEntity, error)
	// searchFunc が非 nil の場合、Search はこの関数を呼ぶ（ctx-aware テスト用）。
	searchFunc func(ctx context.Context, filter boardapi.ClientListOptions, opts repository.ReadOptions) ([]boardapi.ClientEntity, error)
}

func (s *stubClientRepo) GetByID(ctx context.Context, _ int, _ repository.ReadOptions) (*boardapi.ClientEntity, error) {
	s.getCount++
	if s.getFunc != nil {
		return s.getFunc(ctx)
	}
	return s.getResult, s.err
}
func (s *stubClientRepo) Search(ctx context.Context, filter boardapi.ClientListOptions, opts repository.ReadOptions) ([]boardapi.ClientEntity, error) {
	s.searchCount++
	if s.searchFunc != nil {
		return s.searchFunc(ctx, filter, opts)
	}
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	return s.searchResult, s.err
}

type stubClientBranchRepo struct {
	searchResult []boardapi.ClientBranchEntity
	err          error
	// searchFunc が非 nil の場合、Search はこの関数を呼ぶ（ctx-aware / handshake テスト用）。
	searchFunc func(ctx context.Context, filter boardapi.ClientBranchListOptions, opts repository.ReadOptions) ([]boardapi.ClientBranchEntity, error)
}

func (s *stubClientBranchRepo) Search(ctx context.Context, filter boardapi.ClientBranchListOptions, opts repository.ReadOptions) ([]boardapi.ClientBranchEntity, error) {
	if s.searchFunc != nil {
		return s.searchFunc(ctx, filter, opts)
	}
	return s.searchResult, s.err
}

type stubContactRepo struct {
	searchResult []boardapi.ContactEntity
	err          error
	// searchFunc が非 nil の場合、Search はこの関数を呼ぶ（ctx-aware / handshake テスト用）。
	searchFunc func(ctx context.Context, filter boardapi.ContactListOptions, opts repository.ReadOptions) ([]boardapi.ContactEntity, error)
}

func (s *stubContactRepo) Search(ctx context.Context, filter boardapi.ContactListOptions, opts repository.ReadOptions) ([]boardapi.ContactEntity, error) {
	if s.searchFunc != nil {
		return s.searchFunc(ctx, filter, opts)
	}
	return s.searchResult, s.err
}

// stubProjectRepo は searchFunc を持ち、テストごとに動作を制御できる。
type stubProjectRepo struct {
	getResult          *boardapi.ProjectEntity
	searchResult       []boardapi.ProjectEntity
	getWithGroupResult *boardapi.ProjectEntity
	getWithGroupErr    error // GetByIDWithGroup 専用エラー（N06）
	err                error
	getCount           int // GetByID 呼び出し回数カウンター（N06 で reverseMapper hit/miss / project enrichment fail テスト等で使用）
	getWithGroupCount  int // GetByIDWithGroup 呼び出し回数カウンター（N06 で priority テスト等で使用）
	searchCount        int // Search 呼び出し回数カウンター（N06 で priority テスト等で使用）
	// searchFunc が非 nil の場合、Search はこの関数を呼ぶ（singleflight/timeout テスト用）。
	searchFunc func(ctx context.Context, filter boardapi.ProjectListOptions, opts repository.ReadOptions) ([]boardapi.ProjectEntity, error)
	// getFunc が非 nil の場合、GetByID はこの関数を呼ぶ（ctx-aware テスト用）。
	// 既存 N03/N04/N05 の互換性のため引数は ctx のみ（id を観測したい場合は getFuncWithID を使う）。
	getFunc func(ctx context.Context) (*boardapi.ProjectEntity, error)
	// getFuncWithID は id を観測したい場合に使用する N06 拡張。getFunc と排他。
	getFuncWithID func(ctx context.Context, id int) (*boardapi.ProjectEntity, error)
	// getWithGroupFunc が非 nil の場合、GetByIDWithGroup はこの関数を呼ぶ（N06）。
	getWithGroupFunc func(ctx context.Context, id int, rg string) (*boardapi.ProjectEntity, error)
}

func (s *stubProjectRepo) GetByID(ctx context.Context, id int, _ repository.ReadOptions) (*boardapi.ProjectEntity, error) {
	s.getCount++
	if s.getFuncWithID != nil {
		return s.getFuncWithID(ctx, id)
	}
	if s.getFunc != nil {
		return s.getFunc(ctx)
	}
	return s.getResult, s.err
}
func (s *stubProjectRepo) Search(ctx context.Context, filter boardapi.ProjectListOptions, opts repository.ReadOptions) ([]boardapi.ProjectEntity, error) {
	s.searchCount++
	if s.searchFunc != nil {
		return s.searchFunc(ctx, filter, opts)
	}
	return s.searchResult, s.err
}
func (s *stubProjectRepo) GetByIDWithGroup(ctx context.Context, id int, rg string) (*boardapi.ProjectEntity, error) {
	s.getWithGroupCount++
	if s.getWithGroupFunc != nil {
		return s.getWithGroupFunc(ctx, id, rg)
	}
	if s.getWithGroupErr != nil {
		return nil, s.getWithGroupErr
	}
	if s.getWithGroupResult != nil {
		return s.getWithGroupResult, nil
	}
	if s.getResult != nil {
		return s.getResult, nil
	}
	return nil, s.err
}

type stubEstimateRepo struct {
	getByDocIDResult *boardapi.EstimateEntity
	err              error
	getByDocIDCount  int
	getByDocIDFunc   func(ctx context.Context, documentID int) (*boardapi.EstimateEntity, error)
}

func (s *stubEstimateRepo) GetByDocumentID(ctx context.Context, documentID int, _ repository.ReadOptions) (*boardapi.EstimateEntity, error) {
	s.getByDocIDCount++
	if s.getByDocIDFunc != nil {
		return s.getByDocIDFunc(ctx, documentID)
	}
	return s.getByDocIDResult, s.err
}

type stubOrderRepo struct {
	getByDocIDResult *boardapi.OrderEntity
	err              error
	getByDocIDCount  int
	getByDocIDFunc   func(ctx context.Context, documentID int) (*boardapi.OrderEntity, error)
}

func (s *stubOrderRepo) GetByDocumentID(ctx context.Context, documentID int, _ repository.ReadOptions) (*boardapi.OrderEntity, error) {
	s.getByDocIDCount++
	if s.getByDocIDFunc != nil {
		return s.getByDocIDFunc(ctx, documentID)
	}
	return s.getByDocIDResult, s.err
}

type stubDeliveryRepo struct {
	getByDocIDResult *boardapi.DeliveryEntity
	err              error
	getByDocIDCount  int
	getByDocIDFunc   func(ctx context.Context, documentID int) (*boardapi.DeliveryEntity, error)
}

func (s *stubDeliveryRepo) GetByDocumentID(ctx context.Context, documentID int, _ repository.ReadOptions) (*boardapi.DeliveryEntity, error) {
	s.getByDocIDCount++
	if s.getByDocIDFunc != nil {
		return s.getByDocIDFunc(ctx, documentID)
	}
	return s.getByDocIDResult, s.err
}

type stubReceiptRepo struct {
	getByDocIDResult *boardapi.ReceiptEntity
	err              error
	getByDocIDCount  int
	getByDocIDFunc   func(ctx context.Context, documentID int) (*boardapi.ReceiptEntity, error)
}

func (s *stubReceiptRepo) GetByDocumentID(ctx context.Context, documentID int, _ repository.ReadOptions) (*boardapi.ReceiptEntity, error) {
	s.getByDocIDCount++
	if s.getByDocIDFunc != nil {
		return s.getByDocIDFunc(ctx, documentID)
	}
	return s.getByDocIDResult, s.err
}

type stubInvoiceRepo struct {
	getResult    *boardapi.InvoiceEntity
	searchResult []boardapi.InvoiceEntity
	err          error
	searchErr    error // N07a: Search 専用エラー（GetByID は err を使い、Search は searchErr を使う）
	searchCount  int   // N07a: Search 呼び出し回数カウンター
	getCount     int   // N07a: GetByID 呼び出し回数カウンター
	// N07a: getFunc が非 nil の場合、GetByID はこの関数を呼ぶ。
	getFunc func(ctx context.Context, id int) (*boardapi.InvoiceEntity, error)
	// N07a: searchFunc が非 nil の場合、Search はこの関数を呼ぶ（filter 観測 / ctx-aware テスト用）。
	searchFunc func(ctx context.Context, filter boardapi.InvoiceListOptions, opts repository.ReadOptions) ([]boardapi.InvoiceEntity, error)
}

func (s *stubInvoiceRepo) GetByID(ctx context.Context, id int, _ repository.ReadOptions) (*boardapi.InvoiceEntity, error) {
	s.getCount++
	if s.getFunc != nil {
		return s.getFunc(ctx, id)
	}
	return s.getResult, s.err
}
func (s *stubInvoiceRepo) Search(ctx context.Context, filter boardapi.InvoiceListOptions, opts repository.ReadOptions) ([]boardapi.InvoiceEntity, error) {
	s.searchCount++
	if s.searchFunc != nil {
		return s.searchFunc(ctx, filter, opts)
	}
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	return s.searchResult, s.err
}

type stubVendorRepo struct {
	getResult    *boardapi.VendorEntity
	searchResult []boardapi.VendorEntity
	err          error
	searchErr    error // Search 専用エラー（GetByID は err を使い、Search は searchErr を使う）
	searchCount  int   // Search 呼び出し回数カウンター（FindVendor T16 相当で使用）
	// searchFunc が非 nil の場合、Search はこの関数を呼ぶ（ctx-aware テスト用）。
	searchFunc func(ctx context.Context, filter boardapi.VendorListOptions, opts repository.ReadOptions) ([]boardapi.VendorEntity, error)
}

func (s *stubVendorRepo) GetByID(_ context.Context, _ int, _ repository.ReadOptions) (*boardapi.VendorEntity, error) {
	return s.getResult, s.err
}
func (s *stubVendorRepo) Search(ctx context.Context, filter boardapi.VendorListOptions, opts repository.ReadOptions) ([]boardapi.VendorEntity, error) {
	s.searchCount++
	if s.searchFunc != nil {
		return s.searchFunc(ctx, filter, opts)
	}
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	return s.searchResult, s.err
}

type stubVendorBranchRepo struct {
	searchResult []boardapi.VendorBranchEntity
	err          error
	// searchFunc が非 nil の場合、Search はこの関数を呼ぶ（ctx-aware / handshake テスト用）。
	searchFunc func(ctx context.Context, filter boardapi.VendorBranchListOptions, opts repository.ReadOptions) ([]boardapi.VendorBranchEntity, error)
}

func (s *stubVendorBranchRepo) Search(ctx context.Context, filter boardapi.VendorBranchListOptions, opts repository.ReadOptions) ([]boardapi.VendorBranchEntity, error) {
	if s.searchFunc != nil {
		return s.searchFunc(ctx, filter, opts)
	}
	return s.searchResult, s.err
}

type stubVendorContactRepo struct {
	searchResult []boardapi.VendorContactEntity
	err          error
	// searchFunc が非 nil の場合、Search はこの関数を呼ぶ（ctx-aware / handshake テスト用）。
	searchFunc func(ctx context.Context, filter boardapi.VendorContactListOptions, opts repository.ReadOptions) ([]boardapi.VendorContactEntity, error)
}

func (s *stubVendorContactRepo) Search(ctx context.Context, filter boardapi.VendorContactListOptions, opts repository.ReadOptions) ([]boardapi.VendorContactEntity, error) {
	if s.searchFunc != nil {
		return s.searchFunc(ctx, filter, opts)
	}
	return s.searchResult, s.err
}

type stubPurchaseOrderRepo struct {
	getResult    *boardapi.PurchaseOrderEntity
	searchResult []boardapi.PurchaseOrderEntity
	err          error
	searchErr    error
	searchCount  int
	getCount     int
	getFunc      func(ctx context.Context, id int) (*boardapi.PurchaseOrderEntity, error)
	searchFunc   func(ctx context.Context, filter boardapi.PurchaseOrderListOptions, opts repository.ReadOptions) ([]boardapi.PurchaseOrderEntity, error)
}

func (s *stubPurchaseOrderRepo) GetByID(ctx context.Context, id int, _ repository.ReadOptions) (*boardapi.PurchaseOrderEntity, error) {
	s.getCount++
	if s.getFunc != nil {
		return s.getFunc(ctx, id)
	}
	return s.getResult, s.err
}
func (s *stubPurchaseOrderRepo) Search(ctx context.Context, filter boardapi.PurchaseOrderListOptions, opts repository.ReadOptions) ([]boardapi.PurchaseOrderEntity, error) {
	s.searchCount++
	if s.searchFunc != nil {
		return s.searchFunc(ctx, filter, opts)
	}
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	return s.searchResult, s.err
}

type stubPaymentRepo struct {
	getResult    *boardapi.PaymentEntity
	searchResult []boardapi.PaymentEntity
	err          error
	searchErr    error
	searchCount  int
	getCount     int
	getFunc      func(ctx context.Context, id int) (*boardapi.PaymentEntity, error)
	searchFunc   func(ctx context.Context, filter boardapi.PaymentListOptions, opts repository.ReadOptions) ([]boardapi.PaymentEntity, error)
}

func (s *stubPaymentRepo) GetByID(ctx context.Context, id int, _ repository.ReadOptions) (*boardapi.PaymentEntity, error) {
	s.getCount++
	if s.getFunc != nil {
		return s.getFunc(ctx, id)
	}
	return s.getResult, s.err
}
func (s *stubPaymentRepo) Search(ctx context.Context, filter boardapi.PaymentListOptions, opts repository.ReadOptions) ([]boardapi.PaymentEntity, error) {
	s.searchCount++
	if s.searchFunc != nil {
		return s.searchFunc(ctx, filter, opts)
	}
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	return s.searchResult, s.err
}

type stubUserRepo struct {
	getResult    *boardapi.UserEntity
	searchResult []boardapi.UserEntity
	err          error
	searchErr    error
	searchCount  int
	getCount     int
	getFunc      func(ctx context.Context, id int) (*boardapi.UserEntity, error)
	searchFunc   func(ctx context.Context, filter boardapi.UserListOptions, opts repository.ReadOptions) ([]boardapi.UserEntity, error)
}

func (s *stubUserRepo) GetByID(ctx context.Context, id int, _ repository.ReadOptions) (*boardapi.UserEntity, error) {
	s.getCount++
	if s.getFunc != nil {
		return s.getFunc(ctx, id)
	}
	return s.getResult, s.err
}
func (s *stubUserRepo) Search(ctx context.Context, filter boardapi.UserListOptions, opts repository.ReadOptions) ([]boardapi.UserEntity, error) {
	s.searchCount++
	if s.searchFunc != nil {
		return s.searchFunc(ctx, filter, opts)
	}
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	return s.searchResult, s.err
}

// newTestRepos は全 stub を埋めた Repos を返すヘルパー（ゼロ値 stub で OK な場合用）。
func newTestRepos() Repos {
	return Repos{
		Clients:        &stubClientRepo{},
		ClientBranches: &stubClientBranchRepo{},
		Contacts:       &stubContactRepo{},
		Projects:       &stubProjectRepo{},
		Estimates:      &stubEstimateRepo{},
		Orders:         &stubOrderRepo{},
		Deliveries:     &stubDeliveryRepo{},
		Receipts:       &stubReceiptRepo{},
		Invoices:       &stubInvoiceRepo{},
		Vendors:        &stubVendorRepo{},
		VendorBranches: &stubVendorBranchRepo{},
		VendorContacts: &stubVendorContactRepo{},
		PurchaseOrders: &stubPurchaseOrderRepo{},
		Payments:       &stubPaymentRepo{},
		Users:          &stubUserRepo{},
	}
}

// recordingHandler は slog.Warn 等を観測するためのテスト用 slog.Handler。
// withRecordedSlog(t) で slog default handler をテスト中だけ差し替える。
// N05 T22 / N06+ で reuse 可能。
type recordingHandler struct {
	records []slog.Record
}

func (h *recordingHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return nil
}
func (h *recordingHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(_ string) slog.Handler      { return h }

// withRecordedSlog は slog default handler を recordingHandler に差し替え、
// t.Cleanup で元に戻す。返値の *recordingHandler に記録されたログを参照できる。
// 注意: t.Parallel() と組み合わせると slog default が競合するため使用不可。
func withRecordedSlog(t *testing.T) *recordingHandler {
	t.Helper()
	h := &recordingHandler{}
	orig := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(orig) })
	return h
}

// filterWarn は recordingHandler に記録された Records から Warn 以上のものを抽出する。
// reverseMapper.ensureBuilt が出力する `[SLOW:cold-reverse-map]` Info を除外したい場面で使用。
func filterWarn(records []slog.Record) []slog.Record {
	out := make([]slog.Record, 0, len(records))
	for _, r := range records {
		if r.Level >= slog.LevelWarn {
			out = append(out, r)
		}
	}
	return out
}

// --- インターフェース適合の静的検証（コンパイル時チェック）---
var (
	_ ClientRepo        = (*stubClientRepo)(nil)
	_ ClientBranchRepo  = (*stubClientBranchRepo)(nil)
	_ ContactRepo       = (*stubContactRepo)(nil)
	_ ProjectRepo       = (*stubProjectRepo)(nil)
	_ EstimateRepo      = (*stubEstimateRepo)(nil)
	_ OrderRepo         = (*stubOrderRepo)(nil)
	_ DeliveryRepo      = (*stubDeliveryRepo)(nil)
	_ ReceiptRepo       = (*stubReceiptRepo)(nil)
	_ InvoiceRepo       = (*stubInvoiceRepo)(nil)
	_ VendorRepo        = (*stubVendorRepo)(nil)
	_ VendorBranchRepo  = (*stubVendorBranchRepo)(nil)
	_ VendorContactRepo = (*stubVendorContactRepo)(nil)
	_ PurchaseOrderRepo = (*stubPurchaseOrderRepo)(nil)
	_ PaymentRepo       = (*stubPaymentRepo)(nil)
	_ UserRepo          = (*stubUserRepo)(nil)
)
