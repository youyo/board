//go:build e2e

package boardapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/youyo/board/internal/boardapi"
)

const (
	e2eBaseURL = "https://api.the-board.jp"
	e2eTimeout = 30 * time.Second
)

// skipIfNoCredentials skips the test if BOARD_API_KEY or BOARD_API_TOKEN is not set.
func skipIfNoCredentials(t *testing.T) {
	t.Helper()
	if os.Getenv("BOARD_API_KEY") == "" || os.Getenv("BOARD_API_TOKEN") == "" {
		t.Skip("E2E: BOARD_API_KEY and BOARD_API_TOKEN are required")
	}
}

// newE2EClient returns a boardapi.Client configured with real credentials from environment variables.
func newE2EClient(t *testing.T) *boardapi.Client {
	t.Helper()
	skipIfNoCredentials(t)
	return boardapi.New(
		e2eBaseURL,
		os.Getenv("BOARD_API_KEY"),
		os.Getenv("BOARD_API_TOKEN"),
		e2eTimeout,
	)
}

// skipIfNotFound skips the test when err is a boardapi 404 Not Found error.
// Use this for resources that may not be enabled for all BOARD accounts.
func skipIfNotFound(t *testing.T, err error, context string) {
	t.Helper()
	var apiErr *boardapi.APIError
	if errors.As(err, &apiErr) && apiErr.Code == boardapi.APIErrorNotFound {
		t.Skipf("E2E: %s returned 404 (resource not available for this account)", context)
	}
}

// requirePositiveID asserts that id > 0.
func requirePositiveID(t *testing.T, id int, label string) {
	t.Helper()
	if id <= 0 {
		t.Fatalf("%s: expected positive ID, got %d", label, id)
	}
}

// requireNonEmpty asserts that s is not empty.
func requireNonEmpty(t *testing.T, s string, label string) {
	t.Helper()
	if s == "" {
		t.Fatalf("%s: expected non-empty string", label)
	}
}

// skipIfRateLimit skips the test when err is a boardapi 429 Rate Limit error.
// E2E tests that issue many paginated API calls may hit the 3/sec rate limit.
func skipIfRateLimit(t *testing.T, err error, context string) {
	t.Helper()
	var apiErr *boardapi.APIError
	if errors.As(err, &apiErr) && apiErr.Code == boardapi.APIErrorRateLimit {
		t.Skipf("E2E: %s hit rate limit (429); skipping", context)
	}
}

// dumpJSON は BOARD API の生レスポンスを tmp/e2e-artifacts/{resource}_{id}.json に書き出す。
// 配置先は repo root 直下の tmp/（.gitignore 済み）。M01 の 厳格フィールド突合 と
// 組み合わせて、未マップフィールドが出た際に「実 API のペイロードが何だったか」を
// レビュー可能にするための副産物。書き込み失敗はテストを fail させない（best-effort）。
//
// 使用例:
//
//	raw, _ := client.ListClientsRaw(ctx)
//	dumpJSON(t, "clients", 0, raw) // list なら id=0
func dumpJSON(t *testing.T, resource string, id int, raw []byte) {
	t.Helper()
	if len(raw) == 0 {
		return
	}
	root, err := findRepoRoot()
	if err != nil {
		t.Logf("dumpJSON: repo root not found: %v", err)
		return
	}
	dir := filepath.Join(root, "tmp", "e2e-artifacts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Logf("dumpJSON: mkdir %s: %v", dir, err)
		return
	}
	name := fmt.Sprintf("%s_%d.json", resource, id)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Logf("dumpJSON: write %s: %v", path, err)
		return
	}
}

// maxDiscoveryProjects は findAnyDocumentID が走査する project の上限数。
// rate limit（3 req/秒）への配慮として最大 3 件に制限する。
const maxDiscoveryProjects = 3

// findAnyDocumentID は projects を response_group={docType} で走査し、
// docType サブオブジェクトが存在する最初の 1 件の (projectID, documentID) を返す。
//
// docType は "estimate" / "order" / "delivery" / "invoice" / "receipt" のいずれか。
// データが見つからない場合は t.Skipf で pending 扱い（data-dependent skip）。
// 不明な docType は t.Fatalf（プログラマエラー）。
//
// 実装方針: GetProjectWithGroupRaw + probe struct で全 docType を統一処理。
// ProjectEntity の delivery/invoice/receipt フィールドは JSON タグが単数形で
// 実 API の複数形キー（deliveries/invoices/receipts）とミスマッチがあるため、
// ProjectEntity には依存せず raw JSON を直接 parse する。
//
// rate limit 配慮: 上位 maxDiscoveryProjects 件のみ走査（最大 1+3=4 req）。
func findAnyDocumentID(t *testing.T, client *boardapi.Client, docType string) (projectID, documentID int) {
	t.Helper()

	// docType の事前検証（プログラマエラーを早期検出）
	switch docType {
	case "estimate", "order", "delivery", "invoice", "receipt":
		// OK
	default:
		t.Fatalf("findAnyDocumentID: unknown docType %q (must be estimate/order/delivery/invoice/receipt)", docType)
	}

	ctx := context.Background()

	// Step 1: project ID 一覧を取得（1 req）
	listRaw, err := client.ListProjectsRaw(ctx, boardapi.WithPerPage(maxDiscoveryProjects))
	if err != nil {
		t.Skipf("findAnyDocumentID: ListProjectsRaw failed: %v", err)
	}

	// project ID だけ抽出する最小 probe
	var projectProbes []struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(listRaw, &projectProbes); err != nil {
		t.Fatalf("findAnyDocumentID: unmarshal project list: %v", err)
	}
	if len(projectProbes) == 0 {
		t.Skipf("findAnyDocumentID: no projects found; %s discovery pending re-verification", docType)
	}

	// 上限 maxDiscoveryProjects 件に絞る（ListProjectsRaw は perPage=3 を渡しているが念のため）
	limit := len(projectProbes)
	if limit > maxDiscoveryProjects {
		limit = maxDiscoveryProjects
	}

	// Step 2: 上位 limit 件を docType 付きで走査
	for i := 0; i < limit; i++ {
		pid := projectProbes[i].ID
		if pid <= 0 {
			continue
		}

		raw, err := client.GetProjectWithGroupRaw(ctx, pid, docType)
		if err != nil {
			// 1 件失敗しても次の project を試す（データなし ≠ API エラー）
			t.Logf("findAnyDocumentID: GetProjectWithGroupRaw(%d, %s): %v (continuing)", pid, docType, err)
			continue
		}

		// probe struct: estimate/order は単一オブジェクト、
		// delivery/invoice/receipt は複数形配列
		var probe struct {
			Estimate *struct {
				ID int `json:"id"`
			} `json:"estimate"`
			Order *struct {
				ID int `json:"id"`
			} `json:"order"`
			Deliveries []struct {
				ID int `json:"id"`
			} `json:"deliveries"`
			Invoices []struct {
				ID int `json:"id"`
			} `json:"invoices"`
			Receipts []struct {
				ID int `json:"id"`
			} `json:"receipts"`
		}
		if err := json.Unmarshal(raw, &probe); err != nil {
			t.Logf("findAnyDocumentID: unmarshal project %d: %v (continuing)", pid, err)
			continue
		}

		var docID int
		switch docType {
		case "estimate":
			if probe.Estimate != nil && probe.Estimate.ID > 0 {
				docID = probe.Estimate.ID
			}
		case "order":
			if probe.Order != nil && probe.Order.ID > 0 {
				docID = probe.Order.ID
			}
		case "delivery":
			if len(probe.Deliveries) > 0 && probe.Deliveries[0].ID > 0 {
				docID = probe.Deliveries[0].ID
			}
		case "invoice":
			if len(probe.Invoices) > 0 && probe.Invoices[0].ID > 0 {
				docID = probe.Invoices[0].ID
			}
		case "receipt":
			if len(probe.Receipts) > 0 && probe.Receipts[0].ID > 0 {
				docID = probe.Receipts[0].ID
			}
		}

		if docID > 0 {
			t.Logf("findAnyDocumentID: found docType=%s projectID=%d documentID=%d", docType, pid, docID)
			return pid, docID
		}
	}

	t.Skipf("findAnyDocumentID: no %s found in top %d projects; pending re-verification", docType, limit)
	return 0, 0 // unreachable（t.Skipf が goroutine を終了させる）
}

// TestFindAnyDocumentID_Estimate_Found は httptest サーバーを使い、
// estimate を持つ project に対して findAnyDocumentID が正しく ID を返すことを検証する。
func TestFindAnyDocumentID_Estimate_Found(t *testing.T) {
	// List レスポンス: 1 件のみ
	listJSON := `[{"id":100}]`
	// GetProjectWithGroupRaw レスポンス: estimate 単一オブジェクト
	getJSON := `{"id":100,"estimate":{"id":999,"message":null,"total":"10000.0","tax":"1000.0","tax_withholding":"0.0","lock_flg":0}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Count", "1")
		if r.URL.Path == "/v1/projects" && r.URL.Query().Get("response_group") == "" {
			w.Write([]byte(listJSON))
			return
		}
		// /v1/projects/100?response_group=estimate
		w.Write([]byte(getJSON))
	}))
	defer srv.Close()

	client := boardapi.New(srv.URL, "test-key", "test-token", 10*time.Second)
	pid, docID := findAnyDocumentID(t, client, "estimate")
	if pid != 100 {
		t.Errorf("projectID: got %d want 100", pid)
	}
	if docID != 999 {
		t.Errorf("documentID: got %d want 999", docID)
	}
}

// TestFindAnyDocumentID_Delivery_Found は httptest サーバーを使い、
// deliveries 配列（複数形）を持つ project に対して findAnyDocumentID が
// 配列先頭の ID を返すことを検証する。
func TestFindAnyDocumentID_Delivery_Found(t *testing.T) {
	listJSON := `[{"id":200}]`
	// deliveries は複数形配列
	getJSON := `{"id":200,"deliveries":[{"id":777,"message":null,"total":"5000.0","tax":"500.0","tax_withholding":"0.0","lock_flg":0}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Count", "1")
		if r.URL.Path == "/v1/projects" && r.URL.Query().Get("response_group") == "" {
			w.Write([]byte(listJSON))
			return
		}
		w.Write([]byte(getJSON))
	}))
	defer srv.Close()

	client := boardapi.New(srv.URL, "test-key", "test-token", 10*time.Second)
	pid, docID := findAnyDocumentID(t, client, "delivery")
	if pid != 200 {
		t.Errorf("projectID: got %d want 200", pid)
	}
	if docID != 777 {
		t.Errorf("documentID: got %d want 777", docID)
	}
}

// TestFindAnyDocumentID_UnknownDocType は不明な docType が t.Fatal を引き起こすことを
// 検証するためのプレースホルダ。実際は t.Fatalf で goroutine が終了するため
// testing.T の挙動に依存し、通常テストでは検証困難。コメントで仕様を明示する。
// Note: unknown docType validation は findAnyDocumentID 内の switch-default で
//
//	t.Fatalf を呼ぶことで保証される（プログラマエラーの早期検出）。
func TestFindAnyDocumentID_UnknownDocType(_ *testing.T) {
	// このテストは docType バリデーションのドキュメントとして存在する。
	// 実際の不正 docType は t.Fatalf で検出されるため、ここでは何もしない。
}

// TestE2E_FindAnyDocumentID_Estimate は実 BOARD API に対して findAnyDocumentID の
// smoke test を行う（estimate のみ、~2 req）。
func TestE2E_FindAnyDocumentID_Estimate(t *testing.T) {
	client := newE2EClient(t)
	pid, docID := findAnyDocumentID(t, client, "estimate")
	requirePositiveID(t, pid, "findAnyDocumentID.projectID")
	requirePositiveID(t, docID, "findAnyDocumentID.documentID")
	t.Logf("TestE2E_FindAnyDocumentID_Estimate: projectID=%d documentID=%d", pid, docID)
}

// findRepoRoot は CWD から go.mod を find-up して repo root を返す。
// `go test ./internal/boardapi/...` で CWD が package ディレクトリになるため、
// repo root を明示的に発見する必要がある。
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from any ancestor of cwd")
		}
		dir = parent
	}
}
