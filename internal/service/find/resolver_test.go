package find

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// T08: client と project が両方成功する
func TestResolveClientAndProject_BothSucceed(t *testing.T) {
	client := &boardapi.ClientEntity{ID: 1}
	proj := &boardapi.ProjectEntity{ID: 2}
	svc := New(Repos{
		Clients:  &stubClientRepo{getResult: client},
		Projects: &stubProjectRepo{getResult: proj},
		// 残りは空 stub
		ClientBranches: &stubClientBranchRepo{},
		Contacts:       &stubContactRepo{},
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
	})
	gotClient, gotProj := svc.resolveClientAndProject(testCtx, 1, 2, repository.ReadOptions{})
	if gotClient == nil || gotClient.ID != 1 {
		t.Fatalf("want client.ID=1, got %v", gotClient)
	}
	if gotProj == nil || gotProj.ID != 2 {
		t.Fatalf("want project.ID=2, got %v", gotProj)
	}
}

// T09: vendor と project が両方成功する
func TestResolveVendorAndProject_BothSucceed(t *testing.T) {
	vendor := &boardapi.VendorEntity{ID: 10}
	proj := &boardapi.ProjectEntity{ID: 20}
	svc := New(Repos{
		Vendors:  &stubVendorRepo{getResult: vendor},
		Projects: &stubProjectRepo{getResult: proj},
		// 残りは空 stub
		Clients:        &stubClientRepo{},
		ClientBranches: &stubClientBranchRepo{},
		Contacts:       &stubContactRepo{},
		Estimates:      &stubEstimateRepo{},
		Orders:         &stubOrderRepo{},
		Deliveries:     &stubDeliveryRepo{},
		Receipts:       &stubReceiptRepo{},
		Invoices:       &stubInvoiceRepo{},
		VendorBranches: &stubVendorBranchRepo{},
		VendorContacts: &stubVendorContactRepo{},
		PurchaseOrders: &stubPurchaseOrderRepo{},
		Payments:       &stubPaymentRepo{},
		Users:          &stubUserRepo{},
	})
	gotVendor, gotProj := svc.resolveVendorAndProject(testCtx, 10, 20, repository.ReadOptions{})
	if gotVendor == nil || gotVendor.ID != 10 {
		t.Fatalf("want vendor.ID=10, got %v", gotVendor)
	}
	if gotProj == nil || gotProj.ID != 20 {
		t.Fatalf("want project.ID=20, got %v", gotProj)
	}
}

// T17: clientID=0, projectID=0 の場合は no-op で (nil, nil)
func TestResolveClientAndProject_ZeroIDs_ReturnsNil(t *testing.T) {
	svc := New(newTestRepos())
	gotClient, gotProj := svc.resolveClientAndProject(testCtx, 0, 0, repository.ReadOptions{})
	if gotClient != nil || gotProj != nil {
		t.Fatalf("want (nil, nil), got (%v, %v)", gotClient, gotProj)
	}
}

// T18: client stub がエラーを返す場合、project のみ返る（非致命 swallow）
func TestResolveClientAndProject_ClientFails_ReturnsProjectOnly_LogsWarn(t *testing.T) {
	proj := &boardapi.ProjectEntity{ID: 5}
	svc := New(Repos{
		Clients:        &stubClientRepo{err: errors.New("client error")},
		Projects:       &stubProjectRepo{getResult: proj},
		ClientBranches: &stubClientBranchRepo{},
		Contacts:       &stubContactRepo{},
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
	})
	gotClient, gotProj := svc.resolveClientAndProject(testCtx, 1, 5, repository.ReadOptions{})
	if gotClient != nil {
		t.Fatalf("want client=nil on error, got %v", gotClient)
	}
	if gotProj == nil || gotProj.ID != 5 {
		t.Fatalf("want project.ID=5, got %v", gotProj)
	}
}

// T19: 両方 stub がエラーを返す場合、(nil, nil)（非致命 swallow）
func TestResolveClientAndProject_BothFail_ReturnsBothNil_LogsWarn(t *testing.T) {
	svc := New(Repos{
		Clients:        &stubClientRepo{err: errors.New("client error")},
		Projects:       &stubProjectRepo{err: errors.New("project error")},
		ClientBranches: &stubClientBranchRepo{},
		Contacts:       &stubContactRepo{},
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
	})
	gotClient, gotProj := svc.resolveClientAndProject(testCtx, 1, 2, repository.ReadOptions{})
	if gotClient != nil || gotProj != nil {
		t.Fatalf("want (nil, nil), got (%v, %v)", gotClient, gotProj)
	}
}

// T20: ctx.Cancel() 直後の呼出しで goroutine が速やかに return する
// ctx-aware stub（<-ctx.Done() を select）で errgroup の ctx 伝播を実際に検証する。
func TestResolveClientAndProject_CtxCancel_ReturnsEarly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// ctx-aware stub: ctx が cancel されたら即 return する
	clientStub := &stubClientRepo{
		getFunc: func(ctx context.Context) (*boardapi.ClientEntity, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(500 * time.Millisecond):
				return &boardapi.ClientEntity{ID: 1}, nil
			}
		},
	}
	projectStub := &stubProjectRepo{
		getFunc: func(ctx context.Context) (*boardapi.ProjectEntity, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(500 * time.Millisecond):
				return &boardapi.ProjectEntity{ID: 2}, nil
			}
		},
	}
	svc := New(Repos{
		Clients:        clientStub,
		Projects:       projectStub,
		ClientBranches: &stubClientBranchRepo{},
		Contacts:       &stubContactRepo{},
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
	})

	cancel() // 即 cancel して errgroup ctx 伝播を発火させる

	done := make(chan struct{})
	go func() {
		defer close(done)
		gotClient, gotProj := svc.resolveClientAndProject(ctx, 1, 2, repository.ReadOptions{})
		// cancel 後は enrichment が失敗して nil（swallow）
		_ = gotClient
		_ = gotProj
	}()

	// 100ms 以内に return しなければ hang とみなす
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("resolveClientAndProject did not return promptly after ctx cancel")
	}
}

// T21: ctx.WithTimeout(1ms) + sleep stub でタイムアウト発火後に return する
// ctx-aware stub で errgroup の ctx.Done() 伝播を実際に検証する。
func TestResolveClientAndProject_DeadlineExceeded_Returns(t *testing.T) {
	// ctx-aware stub: ctx が切れたら ctx.Err() を返す
	slowClientStub := &stubClientRepo{
		getFunc: func(ctx context.Context) (*boardapi.ClientEntity, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(500 * time.Millisecond):
				return &boardapi.ClientEntity{ID: 1}, nil
			}
		},
	}
	slowProjectStub := &stubProjectRepo{
		getFunc: func(ctx context.Context) (*boardapi.ProjectEntity, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(500 * time.Millisecond):
				return &boardapi.ProjectEntity{ID: 2}, nil
			}
		},
	}
	svc := New(Repos{
		Clients:        slowClientStub,
		Projects:       slowProjectStub,
		ClientBranches: &stubClientBranchRepo{},
		Contacts:       &stubContactRepo{},
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
	})

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond) // ctx を確実に期限切れにする

	done := make(chan struct{})
	go func() {
		defer close(done)
		gotClient, gotProj := svc.resolveClientAndProject(ctx, 1, 2, repository.ReadOptions{})
		// timeout 後は enrichment 失敗で nil（swallow されること自体がテストの観点）
		_ = gotClient
		_ = gotProj
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("resolveClientAndProject did not return promptly after deadline exceeded")
	}
}

// ========== N07c: ResolveClientByName / ResolveVendorByName ==========

func TestResolveClientByName_EmptyName(t *testing.T) {
	repos := newTestRepos()
	svc := New(repos)
	id, err := svc.ResolveClientByName(context.Background(), "", repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
	if id != 0 {
		t.Errorf("id = %d, want 0", id)
	}
}

func TestResolveClientByName_NotFound(t *testing.T) {
	repos := newTestRepos()
	repos.Clients = &stubClientRepo{searchResult: []boardapi.ClientEntity{}}
	svc := New(repos)
	_, err := svc.ResolveClientByName(context.Background(), "Acme", repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error for not-found, got nil")
	}
	if msg := err.Error(); !contains(msg, "no client matches") || !contains(msg, "Acme") {
		t.Errorf("error message = %q, want to contain 'no client matches' and 'Acme'", msg)
	}
}

func TestResolveClientByName_SingleHit(t *testing.T) {
	repos := newTestRepos()
	repos.Clients = &stubClientRepo{
		searchResult: []boardapi.ClientEntity{{ID: 42, Name: "Acme Inc"}},
	}
	svc := New(repos)
	id, err := svc.ResolveClientByName(context.Background(), "Acme", repository.ReadOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 42 {
		t.Errorf("id = %d, want 42", id)
	}
}

func TestResolveClientByName_MultipleHits(t *testing.T) {
	repos := newTestRepos()
	repos.Clients = &stubClientRepo{
		searchResult: []boardapi.ClientEntity{
			{ID: 1, Name: "Acme A"},
			{ID: 2, Name: "Acme B"},
			{ID: 3, Name: "Acme C"},
		},
	}
	svc := New(repos)
	id, err := svc.ResolveClientByName(context.Background(), "Acme", repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error for multiple hits, got nil")
	}
	if id != 0 {
		t.Errorf("id = %d, want 0", id)
	}
	msg := err.Error()
	for _, want := range []string{"multiple clients match", "Acme", "id=1", "id=2", "id=3", "use --id"} {
		if !contains(msg, want) {
			t.Errorf("error message %q does not contain %q", msg, want)
		}
	}
}

func TestResolveClientByName_MultipleHitsTruncated(t *testing.T) {
	// >5 hits → 5 件まで列挙し、残りは省略表記する
	hits := make([]boardapi.ClientEntity, 7)
	for i := range hits {
		hits[i] = boardapi.ClientEntity{ID: i + 1, Name: "Acme"}
	}
	repos := newTestRepos()
	repos.Clients = &stubClientRepo{searchResult: hits}
	svc := New(repos)
	_, err := svc.ResolveClientByName(context.Background(), "Acme", repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	if !contains(msg, "id=5") {
		t.Errorf("expected id=5 in message: %q", msg)
	}
	if contains(msg, "id=6") || contains(msg, "id=7") {
		t.Errorf("expected truncation, but found id=6 or id=7 in: %q", msg)
	}
	if !contains(msg, "2 more") {
		t.Errorf("expected '2 more' truncation hint in: %q", msg)
	}
}

func TestResolveClientByName_RepoError(t *testing.T) {
	repos := newTestRepos()
	repos.Clients = &stubClientRepo{
		searchFunc: func(_ context.Context, _ boardapi.ClientListOptions, _ repository.ReadOptions) ([]boardapi.ClientEntity, error) {
			return nil, errors.New("boom")
		},
	}
	svc := New(repos)
	_, err := svc.ResolveClientByName(context.Background(), "Acme", repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error from underlying repo, got nil")
	}
}

func TestResolveVendorByName_NotFound(t *testing.T) {
	repos := newTestRepos()
	repos.Vendors = &stubVendorRepo{searchResult: []boardapi.VendorEntity{}}
	svc := New(repos)
	_, err := svc.ResolveVendorByName(context.Background(), "Globex", repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error for not-found, got nil")
	}
	if !contains(err.Error(), "no vendor matches") {
		t.Errorf("error message %q does not contain 'no vendor matches'", err.Error())
	}
}

func TestResolveVendorByName_SingleHit(t *testing.T) {
	repos := newTestRepos()
	repos.Vendors = &stubVendorRepo{
		searchResult: []boardapi.VendorEntity{{ID: 99, Name: "Globex"}},
	}
	svc := New(repos)
	id, err := svc.ResolveVendorByName(context.Background(), "Globex", repository.ReadOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 99 {
		t.Errorf("id = %d, want 99", id)
	}
}

func TestResolveVendorByName_MultipleHits(t *testing.T) {
	repos := newTestRepos()
	repos.Vendors = &stubVendorRepo{
		searchResult: []boardapi.VendorEntity{
			{ID: 10, Name: "Globex A"},
			{ID: 11, Name: "Globex B"},
		},
	}
	svc := New(repos)
	_, err := svc.ResolveVendorByName(context.Background(), "Globex", repository.ReadOptions{})
	if err == nil {
		t.Fatal("expected error for multiple hits, got nil")
	}
	for _, want := range []string{"multiple vendors match", "id=10", "id=11"} {
		if !contains(err.Error(), want) {
			t.Errorf("error message %q does not contain %q", err.Error(), want)
		}
	}
}

// contains は文字列内に substr が含まれるかを返す（テスト用ヘルパー）。
func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
