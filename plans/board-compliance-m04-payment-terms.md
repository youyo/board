# M04: payment_terms 完走（List / Get / Search E2E + 厳格フィールド突合）

## Overview
| 項目 | 値 |
|------|---|
| ステータス | 実装完了 / E2E 実行済（List・Search は 1 未マップ検出で Fail、Get は 404 = API 非対応） |
| 依存 | M01（`testhelper.StrictFieldDiff`, `dumpJSON`, `/tmp/` 整備）完了 / M02（Raw 層パターン確立）完了 / M03（Get 404 非対応・未マップ検知の運用ルール確立）完了 |
| 見積 API 呼び出し | **~4 req**（List 1 + Get 2（List 再呼 + Get 本体）+ Search 1 = 4 req。予備を含め 5 req 以下） |
| 対象ファイル | `internal/boardapi/payment_terms.go`（追記: Raw メソッド 3 本）<br>`internal/boardapi/payment_terms_test.go`（新規: Raw メソッド単体 TDD）<br>`internal/boardapi/e2e_payment_terms_test.go`（新規: 実 API E2E） |
| 複雑度 | M（中）: M03 と同形式の 1:1 移植のため、探索・弁証法レビュー相当は Leader 側で既に事前調査済。本 plan 自体が準拠検証の成果物。 |

## Goal
- `payment_terms` を L / G / S で E2E 通過させ、`StrictFieldDiff` で `PaymentTermEntity` に対する未マップフィールドを検知する。
- Raw 版 API メソッド（`ListPaymentTermsRaw`, `GetPaymentTermRaw`, `SearchPaymentTermsRaw`）を追加し、M02/M03 で確立したパターンを踏襲する。
- 実 API の JSON が `PaymentTermEntity` と完全一致していることを証明するか、未マップフィールドを炙り出す（Fail でも準拠検証の成果物として記録）。

## 非ゴール
- `repository.PaymentTermRepository` 層の変更
- 他リソースへの Raw 追加（M05 以降で 1 リソースずつ）
- 既存 `ListPaymentTerms` / `GetPaymentTerm` / `SearchPaymentTerms` / `ListPaymentTermsPage` の削除・シグネチャ変更（互換維持、Raw は並列追加）
- `PaymentTermEntity` / `PaymentTermSearchParams` の構造変更（発見した未マップフィールドは別 commit / 別 M で対応）
- Get エンドポイントが 404 を返した場合の `GetPaymentTerm` 系メソッド削除（M03 で提起済のフォローアップ案件と同様、別 M で扱う）

## Architecture Decision: M02/M03 パターンの直接継承

`internal/boardapi/payment_terms.go` の既存構造は `project_types.go` と**完全同形**:

| 項目 | project_types | payment_terms |
|------|---------------|---------------|
| Entity フィールド | `ID, Name, Memo, UpdatedAt, CreatedAt` | `ID, Name, Memo, UpdatedAt, CreatedAt`（同一） |
| SearchParams | `Name, UpdatedAtFrom` | `Name, UpdatedAtFrom`（同一） |
| 既存メソッド | `List / Get / Search / ListPage` | `List / Get / Search / ListPage`（同一） |
| API パス | `/v1/project_types{,/{id}}` | `/v1/payment_terms{,/{id}}` |

したがって **M03 の Raw 層実装・Unit テスト・E2E テストを payment_terms 向けに機械的に置換**すれば M04 は完走する。新規の型定義・新規のテスト戦略は一切不要。

### API シグネチャ（確定）

```go
// ListPaymentTermsRaw returns the raw HTTP response body of GET /v1/payment_terms
// merged across all pages as a single JSON array.
// Intended for E2E strict field diff; regular callers should use ListPaymentTerms.
func (c *Client) ListPaymentTermsRaw(ctx context.Context, opts ...ListAllOption) ([]byte, error)

// GetPaymentTermRaw returns the raw HTTP response body for GET /v1/payment_terms/{id}.
// Intended for E2E strict field diff; regular callers should use GetPaymentTerm.
func (c *Client) GetPaymentTermRaw(ctx context.Context, id int) ([]byte, error)

// SearchPaymentTermsRaw returns the raw HTTP response body of GET /v1/payment_terms
// with the given search params, merged across all pages as a single JSON array.
// Intended for E2E strict field diff; regular callers should use SearchPaymentTerms.
func (c *Client) SearchPaymentTermsRaw(ctx context.Context, params PaymentTermSearchParams, opts ...ListAllOption) ([]byte, error)
```

## Sequence Diagram

```mermaid
sequenceDiagram
  participant Test as E2E Test
  participant API as boardapi.Client
  participant BOARD as BOARD API
  participant Dump as dumpJSON
  participant Diff as StrictFieldDiff
  participant FS as tmp/e2e-artifacts/

  Note over Test,FS: E1: TestE2E_PaymentTerms_List (1 req)
  Test->>API: ListPaymentTermsRaw(ctx)
  API->>BOARD: GET /v1/payment_terms?page=1&per_page=100
  BOARD-->>API: 200 OK + JSON array
  API-->>Test: raw ([]byte; `[{...},{...}]`)
  Test->>Dump: dumpJSON(t, "payment_terms", 0, raw)
  Dump->>FS: write payment_terms_0.json
  Test->>Diff: StrictFieldDiff(t, raw, &[]PaymentTermEntity{})
  Diff-->>Test: unmapped keys (t.Errorf if > 0)

  Note over Test,FS: E2: TestE2E_PaymentTerms_Get (1-2 req)
  Test->>API: ListPaymentTermsRaw(ctx) (discovery)
  API->>BOARD: GET /v1/payment_terms?page=1&per_page=100
  BOARD-->>API: 200 OK
  alt items == 0
    Test->>Test: t.Skipf("pending re-verification")
  else items > 0
    Test->>API: GetPaymentTermRaw(ctx, firstID)
    API->>BOARD: GET /v1/payment_terms/{id}
    alt 200 OK
      BOARD-->>API: 200 OK + JSON object
      API-->>Test: raw
      Test->>Dump: dumpJSON(t, "payment_terms", id, raw)
      Test->>Diff: StrictFieldDiff(t, raw, &PaymentTermEntity{})
    else 404 (API 非対応の疑い)
      BOARD-->>API: 404
      API-->>Test: *APIError{Code: APIErrorNotFound}
      Test->>Test: t.Fatalf (compliance 記録)
    end
  end

  Note over Test,FS: E3: TestE2E_PaymentTerms_Search (1 req)
  Test->>API: SearchPaymentTermsRaw(ctx, {Name:"zzz_..."})
  API->>BOARD: GET /v1/payment_terms?page=1&per_page=100&name=zzz_...
  BOARD-->>API: 200 OK + JSON array
  API-->>Test: raw
  Test->>Dump: dumpJSON(t, "payment_terms_search", 0, raw)
  Test->>Diff: StrictFieldDiff(t, raw, &[]PaymentTermEntity{})
```

## TDD Test Design

### Unit（build tag なし、`payment_terms_test.go` 新規）

`project_types_test.go`（M03 の正本）と同形式で `http.RoundTripper` モックを使う。`roundTripperFunc` / `jsonResp` ヘルパは `accounting_types_test.go`（M02）で既にパッケージスコープに定義済のため、payment_terms 側では再定義しない。

| # | テストケース | 入力 | 期待出力 |
|---|-------------|------|---------|
| U1 | `ListPaymentTermsRaw` 単一ページ | `[{"id":1,"name":"A","memo":"m","updated_at":"...","created_at":"..."}]` | 要素 1 の JSON 配列、キー列 `id/name/memo/updated_at/created_at` が残存、path=`/v1/payment_terms`、page=1 |
| U2 | `ListPaymentTermsRaw` 複数ページ連結 | per_page=2 で page1=2件、page2=1件 | 要素 3 の JSON 配列として valid、pageCount==2、id=1,2,3 の順を確認 |
| U3 | `GetPaymentTermRaw` 単体取得 | `{"id":42,"name":"Foo",...}` | body byte 完全一致、パス `/v1/payment_terms/42` |
| U4 | `SearchPaymentTermsRaw` パラメータ送信 | name=keyword, updated_at_from=... | query string に両者が含まれ、body 返却 |
| U5 | `GetPaymentTermRaw` 404 | status=404 | `*APIError{Code: APIErrorNotFound}` |

**正常系・異常系・エッジケース網羅**:
- 正常系: U1, U2, U3, U4
- 異常系: U5
- エッジケース: U2（複数ページ境界）

**Red 段階**: U1–U5 を先に書く。`Client.ListPaymentTermsRaw` 等が未実装なので compile error で Red。

**Green 段階**: `payment_terms.go` に Raw 3 メソッドを追記して U1–U5 全 Green。

**Refactor 段階**: `project_types.go` の該当メソッドと**字面レベルで対称**（リソース名/エンティティ名のみ差し替え）であることを確認。過度な DRY 化は行わない（M02/M03 と同方針）。

### E2E（build tag `e2e`、`e2e_payment_terms_test.go` 新規）

1 test 1 endpoint。

| # | テスト名 | 呼び出す API | 検証内容 | req 数 |
|---|---------|-------------|---------|--------|
| E1 | `TestE2E_PaymentTerms_List` | `ListPaymentTermsRaw` | `dumpJSON("payment_terms", 0, raw)` + `StrictFieldDiff(&[]PaymentTermEntity{})` + 件数 Logf | 1 |
| E2 | `TestE2E_PaymentTerms_Get` | `ListPaymentTermsRaw` で先頭 id 発見 → `GetPaymentTermRaw(id)` | List 0 件 → `t.Skipf("... pending re-verification ...")`。Get 成功 → `dumpJSON("payment_terms", id, raw)` + `StrictFieldDiff(&PaymentTermEntity{})` + ID 一致確認。Get 404 → `t.Fatalf` | 1-2 |
| E3 | `TestE2E_PaymentTerms_Search` | `SearchPaymentTermsRaw({Name: "zzz_nonexistent_keyword_for_e2e"})` | `dumpJSON("payment_terms_search", 0, raw)` + `StrictFieldDiff` + 件数 Logf | 1 |

**E1/E2/E3 は各々独立**。`go test -tags e2e -run TestE2E_PaymentTerms_List -count=1 ./internal/boardapi/` で単独実行可能。

## 403/429 ハンドリング（運用ルール厳守）

- `skipIfNotFound` / `skipIfRateLimit` は呼ばない。
- `*APIError{Code: APIErrorRateLimit|APIErrorForbidden}` なら `t.Fatalf` 即失敗。
- RATE_LIMITED 時はロードマップ Blockers に記録、status: RATE_LIMITED で完了報告。

## データ依存 skip 規約（List 0 件 → Get のみ）

- List 0 件 → Get は `t.Skipf` で Pending Re-verification 入り（M02 で確立、ロードマップ運用ルール済）。
- Skip メッセージに `pending re-verification` を含める。

## Get 404 ハンドリング（M03 で確立したルール）

- `GET /v1/payment_terms/{id}` が 404 を返した場合、これは API 非対応の疑い。
- E2 は `t.Fatalf` で Fail 扱い（Entity 修正や `GetPaymentTerm` 削除は別 M に回す）。
- 結果を「フォローアップ」節に転記し、ロードマップにも反映。

## ForceRefresh の扱い

本 M04 は boardapi 直呼び（cache を経由しない）のため、cache バイパスは構造的に保証される。

## Implementation Steps

### Red（Unit テスト先行）
- [x] `internal/boardapi/payment_terms_test.go` を新規作成（package `boardapi_test`）
- [x] `newPaymentTermsMockClient` ヘルパ追加（M03 の `newProjectTypesMockClient` と同形）
- [x] U1 `TestListPaymentTermsRaw_SinglePage` 記述
- [x] U2 `TestListPaymentTermsRaw_MultiPage` 記述
- [x] U3 `TestGetPaymentTermRaw_Success` 記述
- [x] U4 `TestSearchPaymentTermsRaw_QueryParams` 記述
- [x] U5 `TestGetPaymentTermRaw_NotFound` 記述
- [x] compile error を確認（`ListPaymentTermsRaw undefined` / `GetPaymentTermRaw undefined` / `SearchPaymentTermsRaw undefined` — Red 確認済）

### Green（Raw 3 本を追加）
- [x] `internal/boardapi/payment_terms.go` に `ListPaymentTermsRaw` を追加（M03 の `ListProjectTypesRaw` と同形）
- [x] `GetPaymentTermRaw` を追加
- [x] `SearchPaymentTermsRaw` を追加
- [x] Unit 5/5 Green: `TestListPaymentTermsRaw_SinglePage`, `TestListPaymentTermsRaw_MultiPage`, `TestGetPaymentTermRaw_Success`, `TestSearchPaymentTermsRaw_QueryParams`, `TestGetPaymentTermRaw_NotFound`

### Refactor
- [x] `project_types.go` の Raw メソッドと対称性を目視確認（リソース名差し替えのみの差分、import 追加なし）
- [x] 既存 `ListPaymentTerms` / `GetPaymentTerm` / `SearchPaymentTerms` / `ListPaymentTermsPage` に影響なし

### E2E（build tag `e2e`）
- [x] `internal/boardapi/e2e_payment_terms_test.go` を新規作成（M03 の `e2e_project_types_test.go` を基に payment_terms 向け置換）
- [x] E1 `TestE2E_PaymentTerms_List` 記述
- [x] E2 `TestE2E_PaymentTerms_Get` 記述（List 0 件 skip、Get 404 → Fatalf）
- [x] E3 `TestE2E_PaymentTerms_Search` 記述

### 静的検査
- [x] `gofmt -s -l internal/boardapi/payment_terms.go internal/boardapi/payment_terms_test.go internal/boardapi/e2e_payment_terms_test.go` 差分 0
- [x] `go vet ./...` 0 issue
- [x] `go vet -tags e2e ./...` 0 issue
- [x] `go test -count=1 ./...` 全 Green（既存 unit を壊していないこと。`mise run test` は既存 repository パッケージの httptest 関連キャッシュ問題で失敗することがあるが、-count=1 で全パッケージ Green を確認）

### E2E 実 API 実行
- [x] `go test -tags e2e -v -count=1 -run TestE2E_PaymentTerms ./internal/boardapi/` を実行
- [x] 実消費 req: 4 req（List 1 + List 再呼 1 + Get 1 + Search 1。計画通り）
- [x] 結果: List FAIL（16 items、未マップ 1）/ Get FAIL（404）/ Search FAIL（16 items、未マップ 1、filter 無効）

### ドキュメント反映
- [x] 本 plan の Implementation Steps / 結果記録を実測値で更新
- [x] `plans/board-compliance-roadmap.md` の M04 セクションを 🟡 に更新
- [x] Get Skip なし（List 16 件返却のため skip 発生せず）→ Pending Re-verification 転記なし
- [x] Get 404 / 未マップ `archive_flg` / 逆方向不整合 `Memo` / Search filter 無効を検出 → 本 plan のフォローアップ節に追記、ロードマップ Changelog にも反映
- [x] ロードマップ Changelog に M04 実行の 1 行追加（日時・発見事項・実消費 req）

### Commit
- [x] `git add` で対象ファイルのみ明示的に追加（`/tmp/` は .gitignore 済）:
  - `internal/boardapi/payment_terms.go`
  - `internal/boardapi/payment_terms_test.go`
  - `internal/boardapi/e2e_payment_terms_test.go`
  - `plans/board-compliance-m04-payment-terms.md`
  - `plans/board-compliance-roadmap.md`
- [x] `git commit` 実行（メッセージは下記「Commit Message」参照、HEREDOC 形式）
- [x] `git status` で clean 確認

## Commit Message（確定文言）

```
test(e2e): M04 payment_terms の List/Get/Search E2E を厳格フィールド突合付きで追加

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
```

## Verification

- `go test ./internal/boardapi/... -count=1` で unit 全 Green（新 U1-U5 + 既存）
- `go vet ./...` 0 issue
- `gofmt -s -l .` 差分 0
- `go test -tags e2e -v -count=1 -run TestE2E_PaymentTerms ./internal/boardapi/` で 3 テスト PASS/SKIP/FAIL の結果を記録
- 未マップフィールドが見つかった場合は Fail して結果セクションに記録（Entity 修正は M04 の Scope 外）
- Get 404 が返った場合は Fail して記録（Scope 外）

## Risks

| リスク | 重大度 | 影響 | 対策 |
|--------|--------|------|------|
| `GET /v1/payment_terms/{id}` が 404 を返す（M03 と同様 API 非対応の可能性） | 中 | E2 Fail | `t.Fatalf` で記録、Entity / メソッド修正は別 M。運用ルール既定済。 |
| `SearchPaymentTerms` の `name` パラメータが API 側で無視される | 低 | Search が全件返却しても StrictFieldDiff は依然有効 | E3 は存在しない name を使い、空配列 or 全件どちらも差分検出可。結果記録にメモ。 |
| `PaymentTermEntity` 未マップフィールド発見（支払条件固有の数値・日付: `closing_day`, `payment_day`, `cycle_type`, `payment_cycle`, `archive_flg`, `company_bank_id`, `company_bank_name`, `discount_*` などの可能性） | 計画通り | List / Search が `t.Errorf` で Fail | 本 M のゴール。結果セクションに記録、Entity 修正は別 M。 |
| Raw List の複数ページ連結で JSON が壊れる | 中 | Unit U2 Fail | U2 で 2 ページ応答をシミュレート、`json.Unmarshal` で要素数検証（M02/M03 と同様） |
| E2E 実行直前にアカウントの payment_terms が 0 件 | 中 | E2 skip、E1/E3 は 0 件でも PASS 可 | `t.Skipf("pending re-verification")` で Pending 入り、再実行手順をロードマップに転記 |
| 100 件超で List 複数ページ化 | 低 | req 数増加 | payment_terms はマスタで 100 件超は非現実的。万一超えても `per_page` デフォルト（100）で対応 |
| `PaymentTermEntity.Memo` が実 API レスポンスに存在しない（M03 の `Memo` 同様の逆方向不整合の可能性） | 低 | StrictFieldDiff は未マップ方向のみ検知するため即時 Fail にはならないが、`PaymentTermEntity.Memo == ""` が常態化する | Get 実行時に JSON 内のキー列を Logf で記録、逆方向不整合を目視検出。Entity 修正は別 M。 |

## 事前見積と実測

| 区分 | 見積 | 実測 |
|------|------|------|
| Unit テスト (RoundTripper mock) | 0 req | **0 req**（mock のため BOARD API 非接続） |
| E1 List | 1 req | **1 req** FAIL（16 items 取得、1 未マップ検出） |
| E2 Get | 1-2 req（List 再呼 + Get）| **2 req** FAIL（List 再呼 1 + Get 1。Get は 404 = API 非対応） |
| E3 Search | 1 req | **1 req** FAIL（16 items 取得 = name フィルタ効かず、1 未マップ検出） |
| **合計** | **~4 req**（予備含め 5 req 以下）| **4 req** |

実行日時: 2026-04-20 17:30 JST

## 結果記録

### 発見した未マップフィールド（PaymentTermEntity に欠落）
List / Search 両方で同じ 1 キー:

- `archive_flg`（アーカイブフラグ）

M03 の `project_types` では `archive_flg` / `company_bank_id` / `company_bank_name` の 3 キーが未マップだったが、payment_terms は `archive_flg` のみ。

### 逆方向不整合（Entity にあるが API に無い）
現行 `PaymentTermEntity.Memo` フィールドは実 API レスポンスに存在しない（キー `memo` が返らない）。M03 の `ProjectTypeEntity.Memo` と同じ現象。本 M04 の Scope は「未マップ検知」であり、Entity 修正は別 commit / 別 M で対応（Out of Scope）。

Get は 404 のため単独 JSON サンプルが得られないが、List レスポンスの各要素キーで同内容を確認。

### 実 API エンドポイントの仕様差異

- **GET /v1/payment_terms/{id} は 404 を返す**（id=53928733、List で取得した有効 ID）。個別 Get エンドポイントは API として存在しない可能性が高い。M03 `project_types` と同じ現象。
- **name パラメータが無視される**: `SearchPaymentTermsRaw` に `name=zzz_nonexistent_keyword_for_e2e` を渡しても 16 件（List と同一件数）返る。API 側が name フィルタを実装していない可能性。M03 `project_types` と同じ現象。

### 実 API JSON サンプル（キー列挙のみ、値は掲載しない）
- `tmp/e2e-artifacts/payment_terms_0.json`: 2545 bytes, 16 items。各要素のキー = `archive_flg`, `created_at`, `id`, `name`, `updated_at`
- `tmp/e2e-artifacts/payment_terms_search_0.json`: 2545 bytes, 16 items（List と同一）。キー同上
- Get は 404 のため dump なし

### Blockers
なし（E2E は意図的に Fail 状態で記録、Entity 修正は Scope 外のため後続 M または別コミットで対応）

## フォローアップ（別 commit / 別 M で対応予定）
- `PaymentTermEntity` に `archive_flg` フィールド追加検討（`ArchiveFlg bool`）
- `PaymentTermEntity.Memo` フィールド削除検討（実 API レスポンスに存在しない）
- `GetPaymentTerm` / `GetPaymentTermRaw` の公開 API 妥当性（そもそも API 非対応なら削除 or エラーメッセージ明確化）を検討
- `SearchPaymentTerms` の `Name` パラメータが効かない件をドキュメント化または削除
- M03 `project_types` と合わせて、マスタ系リソース全体で **個別 Get エンドポイント非対応の傾向** があることが確定しつつあるため、M05 `document_send_channels` / M06 `purchase_types` で同現象を確認後、統合的なフォローアップ M を設定する可能性あり

## Out of Scope（次 M 以降で扱う）
- M05: `document_send_channels` に同パターンを適用
- M06: `purchase_types` Search/Get 追補
- M07 以降: 他リソース
- `PaymentTermEntity` の構造変更（未マップフィールド追加、`Memo` の妥当性検証）
- `GetPaymentTerm` / `GetPaymentTermRaw` の公開 API 妥当性検討（404 が返る場合）

## チェックリスト

### 観点1: 実装実現可能性と完全性
- [x] 手順の抜け漏れがないか（Red→Green→Refactor→E2E→検証→ドキュメント→Commit の一貫フロー）
- [x] 各ステップが十分に具体的か（ファイル名・関数名・テスト名を全て列挙）
- [x] 依存関係が明示されているか（M01/M02/M03 完了が前提、Unit Red が E2E より先）
- [x] 変更対象ファイルが網羅されているか（3 ソース + 2 plan の計 5 ファイル）
- [x] 影響範囲が正確に特定されているか（既存 4 メソッドには触らない、Entity 不変）

### 観点2: TDD テスト設計の品質
- [x] 正常系テストケースが網羅されているか（U1/U2/U3/U4 + E1/E2/E3）
- [x] 異常系テストケースが定義されているか（U5 Get 404、E2 Get 404 Fatalf）
- [x] エッジケースが考慮されているか（U2 複数ページ境界、E2 List 0 件 Skip）
- [x] 入出力が具体的に記述されているか（JSON リテラル、クエリパラメータ名、path）
- [x] Red→Green→Refactor の順序が守られているか（Implementation Steps で明示）
- [x] モック/スタブの設計が適切か（`roundTripperFunc` は既存パッケージスコープを再利用）

### 観点3: アーキテクチャ整合性
- [x] 既存の命名規則に従っているか（`{Resource}Raw` 命名、ファイル名規約）
- [x] 設計パターンが一貫しているか（M02/M03 と完全対称）
- [x] モジュール分割が適切か（boardapi 直下、cache/repository を跨がない）
- [x] 依存方向が正しいか（test → Client → BOARD の単方向）
- [x] 類似機能との統一性があるか（project_types/accounting_types と同形式）

### 観点4: リスク評価と対策
- [x] リスクが適切に特定されているか（7 リスクを列挙）
- [x] 対策が具体的か（各リスクに具体対処を記述）
- [x] フェイルセーフが考慮されているか（Fail 時も意図的 commit、別 M で修正）
- [x] パフォーマンスへの影響が評価されているか（req 数 5 以下、日次上限 1500 に対し十分余裕）
- [x] セキュリティ観点が含まれているか（`/tmp/` は .gitignore 済、API key/token は env 経由、secrets をログに出さない）
- [x] ロールバック計画があるか（commit 単位が細かいため revert 可能）

### 観点5: シーケンス図の完全性
- [x] 正常フローが記述されているか（E1/E2/E3 の success パス）
- [x] エラーフローが記述されているか（E2 alt ブロックで 404 分岐）
- [x] ユーザー・システム・外部 API 間の相互作用が明確か（Test / Client / BOARD の 3 参加者）
- [x] タイミング・同期的な処理の制御が明記されているか（E2 は discovery → Get の順）
- [x] リトライ・タイムアウト等の例外ハンドリングが図に含まれているか（`DoWithRetry` は Client 内部、404 は retry 対象外であることを本文で明示）
