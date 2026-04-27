---
title: "N09: E2E テスト再構築（実 API 感応の代表 33-41 ケース）"
project: "board"
phase: "Phase N"
milestone: "N09"
created: "2026-04-27"
status: "Ready for Implementation"
complexity: "M"
related_adr: "docs/adr/ADR-001-find-layer.md"
parent_roadmap: "plans/board-phase-n-roadmap.md"
parent_design: "plans/wondrous-skipping-snowglobe.md"
---

# N09: E2E テスト再構築（実 API 感応の代表 33-41 ケース）

## 1. Context

N02 設計書 §8 に従い、現行 `find/` の E2E テストを **実 API 感応する代表 33-41 ケース**へ削減する。
旧 `internal/service/find/` 配下の E2E 47 関数 / 193 ケースは N07b で既に削除済み（unit test のみ残存）。

**ケース数の再解釈**: 本計画では Service 層 41 ケース + MCP handler 経由 5 ケース = **計 46 ケース**。
N02 §8.3 / roadmap の `33-41` 上限は Service 層に限定したものとして読み替える（タスク指示の
"MCP server 経由 ... in-process で MCP handler を直接呼び ... resolver / fanout を検証" 要請を満たすため、
handler 経由 5 ケースを別カテゴリで追加）。

本マイルストーンでは:

1. `internal/service/find/` 配下に新しい E2E テストを `//go:build e2e` で追加する（unit と完全分離）
2. SKIP 統一テンプレート 4 種（no-data / cache-warm / rate-limit / no-creds）を整備し `[SKIP:cat] msg` 形式で grep 可能にする
3. **実 API 感応** = handler の正常系 + 境界（resolver 重複ヒット / status post-filter）+ 異常系（reject）を最低 1 ケースずつ
4. per-batch（メソッド単位）実行を前提とし、CI では実行せずローカル開発者が `BOARD_API_KEY` + `BOARD_API_TOKEN` 設定下で `-tags e2e -run TestE2E_FindXxx` を順次実行する

## 2. Scope / Non-Goals

**In scope**:
- `internal/service/find/e2e_*_test.go` の新規作成（11 メソッド × 3 主要 + Document 4 種 × 2 分岐）
- `internal/service/find/e2e_helpers_test.go` 新設（SKIP 統一テンプレ・newE2EService・dumpJSON 等）
- **MCP in-process handler 経由 E2E**: `internal/mcpserver/e2e_handler_test.go` で `CallToolRequest` を直接 handler に渡し、構造的不可フラグの reject / disambiguate vs fanout 挙動差 / resolver 経路を実機検証（T42-T46 = 5 ケース）
- per-batch ガイド追記（README / docs/specs §39）
- ADR-001 Implementation Status 更新（N09 完了 + Payment.Project=nil 仮説の E2E 確認結果）

**Out of scope**:
- `boardapi/` 層 E2E（Phase compliance で完走済、本 N09 では触らない）
- repository 層 E2E（同上）
- HTTP MCP server プロセス起動を伴う E2E（in-process handler 直接呼びのみ）
- `--id と --client-name` 併用時の resolver skip ロジックの**追加実装**（N09 では現状の reject 動作を E2E で固定するのみ。skip ロジック追加は N10 以降で別途検討）
- 実 API への新規 fixture 作成（既存 BOARD アカウント環境のデータを使う）

## 3. テストケース設計

### 3.1 メソッド別代表ケース

| Find メソッド | 正常系 | 境界 | 異常系 | 計 |
|---|---|---|---|---|
| FindClient | T01 ID lookup / T02 NameCont enrichment | T03 重複候補（複数クライアント名 LIKE 一致） | T04 ID + NameCont 両方指定 reject | 4 |
| FindProject | T05 ClientID lookup / T06 ClientName 逆引き / T07 Statuses post-filter (OR) | T08 ClientName 重複ヒット時の disambiguate error | T09 Status-only クエリ reject (narrowing 必須) | 5 |
| FindEstimate | T10 ProjectID fanout / T11 DocumentID + reverseMapper (cache hit) | T12 reverseMapper cache miss + cold timeout フォールバック | T13 ID + ProjectID 両方指定 reject | 4 |
| FindOrder | T14 ProjectID fanout / T15 DocumentID + reverseMapper | T16 cache miss | T17 reject | 4 |
| FindDelivery | T18 ProjectID fanout（配列全件） / T19 DocumentID | T20 cache miss | T21 reject | 4 |
| FindReceipt | T22 ProjectID fanout / T23 DocumentID | T24 cache miss | T25 reject | 4 |
| FindInvoice | T26 ID lookup / T27 Status (single) API delegation | T28 Statuses[] (multi) narrowing reject | T29 ProjectName 構造的不可 reject | 4 |
| FindVendor | T30 ID lookup / T31 NameCont enrichment | T32 重複候補 | (T04 と同等のため省略) | 3 |
| FindPurchaseOrder | T33 ID lookup / T34 VendorName resolver | T35 Statuses[] reject | T36 ProjectName 構造的不可 reject | 4 |
| FindPayment | T37 ID lookup / T38 VendorName resolver | T39 Statuses[] reject | T40 PurchaseOrderID 未対応 reject | 4 |
| FindUser | T41 NameCont (text-OR name/email/first/last) | (single tier、境界省略) | | 1 |

**合計**: T01-T41 = **41 ケース**（N02 §8.3 上限値ピッタリ）。
データ不在で SKIP しても `[SKIP:no-data]` ログで明示。

注: 構造的不可 reject (T29 / T36 / T40) は Service 層では到達しない経路（N07c で CLI/MCP handler 側でガード）。
Service 直接呼びでは「Query 構築可能でも内部 validation で reject される」ケースに限定し、handler 経路の reject 検証は §3.1bis (T42-T46) で行う。

### 3.1bis MCP handler 経由の代表ケース (T42-T46)

`internal/mcpserver/` 配下に `e2e_handler_test.go` を新設し、N08 で primary defense として残した
handler reject + N07c の resolver 経路を in-process で検証する:

| # | ケース | 検証内容 |
|---|---|---|
| T42 | `find_estimates` に `status` 引数を渡す | handler が `status not supported (structural)` で reject |
| T43 | `find_invoices` に `project_name` を渡す | `(NOT YET SUPPORTED)` reject（D4 contingently unimplemented） |
| T44 | `find_payments` に `purchase_order_id` を渡す | 同上 reject（D4） |
| T45 | `find_projects` を `client_name` 単独で呼ぶ | resolver 経由で disambiguate が動作（実 API） |
| T46 | `find_orders` を `client_name` で呼ぶ | fanout 挙動（disambiguate なし、複数候補をそのまま fan-out） |

T45 / T46 は cache miss 時の cold path も含めて観測対象とし、cache miss 頻度を ADR-001 N09 entry に記録（D6 再判断用）。

### 3.2 SKIP 統一テンプレート

```go
// internal/service/find/e2e_helpers_test.go
//go:build e2e

func skipIfNoCreds(t *testing.T) {
    if os.Getenv("BOARD_API_KEY") == "" || os.Getenv("BOARD_API_TOKEN") == "" {
        t.Skipf("[SKIP:no-creds] BOARD_API_KEY and BOARD_API_TOKEN required")
    }
}
func skipIfNoData(t *testing.T, label string, got, want int) {
    if got < want {
        t.Skipf("[SKIP:no-data] %s got=%d want>=%d", label, got, want)
    }
}
func skipIfCacheWarmNeeded(t *testing.T, reason string) {
    t.Skipf("[SKIP:cache-warm] %s", reason)
}
func skipIfRateLimit(t *testing.T, err error) bool {
    var apiErr *boardapi.APIError
    if errors.As(err, &apiErr) && apiErr.Code == boardapi.APIErrorRateLimit {
        t.Skipf("[SKIP:rate-limit] %v", err)
        return true
    }
    return false
}
```

grep フォーマット: `[SKIP:no-data|cache-warm|rate-limit|no-creds] ...`

### 3.3 newE2EService helper

`app.New()` ではなく直接 boardapi/repository/find Service を組む薄い helper を用意。
キャッシュは tmp ディレクトリで毎回新規（cache miss の動作も検証可能）。

```go
func newE2EService(t *testing.T) *find.Service {
    skipIfNoCreds(t)
    // boardapi.Client → cache(temp dir) → repositories → find.New
    // ...
}
```

## 4. per-batch 実行設計

### 4.1 実行例

```bash
# Find メソッド単位で順次実行（rate-limit 配慮: 3 req/sec、3000/day）
go test -tags e2e -v -count=1 -run TestE2E_FindClient   ./internal/service/find/
go test -tags e2e -v -count=1 -run TestE2E_FindProject  ./internal/service/find/
go test -tags e2e -v -count=1 -run TestE2E_FindEstimate ./internal/service/find/
# ... 11 batch
```

- 並列禁止（`-parallel 1` を helper 側で `t.Setenv` 強制）
- バッチ間に 1-2 秒の手動待機で 3 req/sec 上限を回避
- 全件一括 `go test -tags e2e ./...` は禁止（README に明記）

### 4.2 CI / ローカル方針

- **CI**: `go test ./...` のみ（unit）。e2e タグは付けない
- **ローカル**: 任意のタイミングで開発者が手動実行
- ガイド: `README.md` の "Testing" セクション + `docs/specs §39` に per-batch 実行手順を追記

## 5. 構造観点の検証ポイント

### 5.1 Payment.Project = nil 仮説の検証 (T37-T39)

N07a で確定した「PaymentEntity に ProjectID 不在 + E2E dump 0 件」仮説を E2E で再確認:

```go
func TestE2E_FindPayment_ProjectIsNilFixed(t *testing.T) {
    // payments を実 API で取得し、Result.Project が常に nil であることを確認
    // → 0 件なら [SKIP:no-data]、件数 > 0 なら全件 nil で assert
}
```

データが入った場合の動作も観測（D1 維持 or ADR-002 起票）。

### 5.2 resolver 重複ヒット境界 (T03 / T08 / T32)

実 API 環境のクライアント / vendor で「同名で複数ヒットする」候補がない可能性が高い。
データが揃わない場合は `[SKIP:no-data] no duplicate names available` で明示し、unit test (resolver_test.go) で境界カバレッジを担保していることをコメントに記載。

### 5.3 Document fanout limit (T10 / T14 / T18 / T22)

`FindDelivery` / `FindReceipt` は配列で複数件返る Project があり得る。`Limit` を 1 / N で実測し、`Limit=N` が `len(Result.Items) <= N` を保証することを確認。

### 5.4 reverseMapper cold path (T12 / T16 / T20 / T24)

cache miss + cold で 10s timeout フォールバックが発火するケースを明示的にテスト。
fresh cache 環境を用意するため、テスト前に `find.Service` を毎回新規生成（キャッシュ DB は temp dir）。

### 5.5 MCP refresh 未公開の再判断 (D6)

E2E は `find.Service` を直接呼ぶため refresh 未公開の影響は受けないが、cache miss が頻発する場合
ADR-001 N09 entry に「実機での cache miss 頻度: X%」を記録し、N10（リリース前）に再判断する。

## 6. TDD サイクル

E2E は実 API 依存のため Refactor フェーズ寄り（unit test で Red→Green は既に完了している）。
helper 関数 (`skipIfXxx` 系) の動作確認は **e2e タグ内のメタテスト** (`TestSkipTemplates_*`) で実施する:

- `e2e_helpers_test.go` 自体が `//go:build e2e` でガードされるため、unit ビルドからは呼べない
- helper のロジックは e2e ビルド内の自己テスト (mock httptest server で `[SKIP:rate-limit]` などをトリガ) で確認
- e2e ファイル本体は build tag e2e で隔離 → unit ビルドに影響なし
- 最終的に以下が pass すること:
  1. `go test ./...` (unit、tag なし)
  2. `go vet -tags e2e ./...` (e2e 込みでコンパイル健全性)
  3. `go test -tags e2e -run '^$' ./internal/service/find/ ./internal/mcpserver/` (compile-only)
  4. `go test -tags e2e -run TestE2E_FindClient ./internal/service/find/` (実 API per-batch、credentials あり時)

## 7. ロールバック計画

- 影響範囲はテストファイル新規作成のみ（production code 0 行変更）
- ロールバックは `git revert <commit>` で完結
- 新 helper が unit ビルドを汚染した場合は build tag をチェックして即修正

## 8. リスク評価

| # | リスク | 確率 | 影響 | 緩和 |
|---|---|---|---|---|
| 1 | rate-limit 超過でテスト連鎖失敗 | 中 | 中 | per-batch 実行徹底、`skipIfRateLimit` で早期 skip |
| 2 | 実 API 仕様変更で不安定化 | 低 | 中 | recordingHandler は採用せず実 API 直接（CI 非実行のため安定性要求は緩い） |
| 3 | データ不在で 80% が SKIP | 中 | 低 | `[SKIP:no-data]` で明示、unit test 側で網羅性確保済み |
| 4 | T29 / T36 / T40 構造的 reject の文言が変わる | 低 | 低 | エラー文言は contains 比較で許容（厳格一致禁止） |
| 5 | Payment.Project=nil 仮説が現環境で破れる | 低 | 中 | E2E で観測 → ADR-001 に記録、コードは N09 では変更しない（N10 で判断） |
| 6 | MCP in-process handler 経由の E2E 追加が時間超過 | 中 | 低 | N09 ではスコープ外。Service 直接呼びのみ |

## 9. 完了基準（Definition of Done）

- [ ] `internal/service/find/e2e_helpers_test.go` 新規（SKIP 4 テンプレ + newE2EService + 自己メタテスト）
- [ ] `internal/service/find/e2e_find_*_test.go` 11 ファイル新規（T01-T41 計 41 ケース、build tag e2e）
- [ ] `internal/mcpserver/e2e_handler_test.go` 新規（T42-T46 計 5 ケース、build tag e2e）
- [ ] `go test ./...` (unit) Green、build tag なしで e2e ファイルが見えないこと
- [ ] `go vet ./...` pass、**`go vet -tags e2e ./...` pass**、`gofmt -s -w .` 差分なし
- [ ] **`go test -tags e2e -run '^$' ./internal/service/find/ ./internal/mcpserver/` で compile-only check が pass**
- [ ] credentials なし環境で `go test -tags e2e -run TestE2E_FindClient ./internal/service/find/` が `[SKIP:no-creds]` で 1 件も fail しないこと
- [ ] `README.md` の Testing セクションに per-batch 実行手順追記（**SKIP 多発時 Effective が 33 を割るリスクを注記**）
- [ ] `docs/specs §39` 更新（E2E 削減 193→46 の根拠 + SKIP テンプレ 4 種、内 5 ケースは MCP handler 経由）
- [ ] `docs/adr/ADR-001-find-layer.md` Implementation Status に N09 完了行追加（Payment.Project=nil 観測 + cache miss 頻度）
- [ ] `CHANGELOG.md` に Unreleased エントリ追加（E2E reduce）
- [ ] commit を Find メソッド単位 + helper + MCP handler + docs で論理分割

## 10. 工程

1. helpers + skeleton 作成（1 commit）
2. FindClient/Vendor/User の E2E（1 commit）
3. FindProject の E2E（1 commit）
4. Document 4 種の E2E（1 commit）
5. FindInvoice/PO/Payment の E2E（1 commit）
6. MCP in-process handler 経由 E2E (T42-T46)（1 commit）
7. docs / CHANGELOG / ADR 更新（1 commit）

合計 ~7 コミット、見積 4-5 日（N02 ロードマップ通り）。
