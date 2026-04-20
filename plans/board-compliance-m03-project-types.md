# M03: project_types 完走（List / Get / Search E2E + 厳格フィールド突合）

## Overview
| 項目 | 値 |
|------|---|
| ステータス | 実装完了 / E2E 実行済（List・Search は 3 未マップ発見で Fail、Get は API 非対応で 404） |
| 依存 | M01（testhelper.StrictFieldDiff, dumpJSON, tmp/）完了済み / M02（Raw 層追加パターン確立）完了済み |
| 見積 API 呼び出し | **~4 req**（List 1 + Get 2（List 再呼 + Get 本体）+ Search 1 = 4 req。予備含めて 5 req 以下） |
| 対象ファイル | `internal/boardapi/project_types.go`（追記: Raw メソッド 3 本）, `internal/boardapi/project_types_test.go`（新規: Raw メソッド単体 TDD）, `internal/boardapi/e2e_project_types_test.go`（新規）|

## Goal
- `project_types` を L / G / S で E2E 通過させ、`StrictFieldDiff` で未マップフィールドを検知する。
- Raw 版 API メソッド（`ListProjectTypesRaw`, `GetProjectTypeRaw`, `SearchProjectTypesRaw`）を追加し、M02 で確立したパターンを踏襲する。
- 実 API の JSON が `ProjectTypeEntity` と完全一致していることを証明するか、未マップフィールドを炙り出す。

## 非ゴール
- `repository.ProjectTypeRepository` 層の変更
- 他リソースへの Raw 追加（M04 以降で 1 リソースずつ）
- Refactor による既存 `ListProjectTypes` / `GetProjectType` / `SearchProjectTypes` の削除（互換維持、Raw は並列追加）
- `ProjectTypeEntity` の構造変更（発見した未マップフィールドは別 commit / 別 M で対応）

## Architecture Decision: M02 パターン継承

既存 `internal/boardapi/project_types.go` は `accounting_types.go` と同じ構造:
- Entity: `ID`, `Name`, `Memo`, `UpdatedAt`, `CreatedAt` の 5 フィールド
- SearchParams: `Name`, `UpdatedAtFrom`
- 既存メソッド: `List` / `Get` / `Search` / `ListPage`

従って **M02 の Raw 層実装を 1:1 で移植**すれば良い。

### API シグネチャ（確定）

```go
// ListProjectTypesRaw returns the raw HTTP response body of GET /v1/project_types
// merged across all pages as a single JSON array.
// Intended for E2E strict field diff; regular callers should use ListProjectTypes.
func (c *Client) ListProjectTypesRaw(ctx context.Context, opts ...ListAllOption) ([]byte, error)

// GetProjectTypeRaw returns the raw HTTP response body for GET /v1/project_types/{id}.
// Intended for E2E strict field diff; regular callers should use GetProjectType.
func (c *Client) GetProjectTypeRaw(ctx context.Context, id int) ([]byte, error)

// SearchProjectTypesRaw returns the raw HTTP response body of GET /v1/project_types
// with the given search params, merged across all pages as a single JSON array.
// Intended for E2E strict field diff; regular callers should use SearchProjectTypes.
func (c *Client) SearchProjectTypesRaw(ctx context.Context, params ProjectTypeSearchParams, opts ...ListAllOption) ([]byte, error)
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

  Test->>API: ListProjectTypesRaw(ctx)
  API->>BOARD: GET /v1/project_types?page=1&per_page=100
  BOARD-->>API: 200 OK + JSON array
  API-->>Test: raw ([]byte; `[{...},{...}]`)
  Test->>Dump: dumpJSON(t, "project_types", 0, raw)
  Dump->>FS: write project_types_0.json
  Test->>Diff: StrictFieldDiff(t, raw, &[]ProjectTypeEntity{})
  Diff-->>Test: unmapped keys (should be empty)

  Test->>API: GetProjectTypeRaw(ctx, firstID)
  API->>BOARD: GET /v1/project_types/{id}
  BOARD-->>API: 200 OK + JSON object
  API-->>Test: raw ([]byte)
  Test->>Dump: dumpJSON(t, "project_types", id, raw)
  Test->>Diff: StrictFieldDiff(t, raw, &ProjectTypeEntity{})
  Diff-->>Test: unmapped keys

  Test->>API: SearchProjectTypesRaw(ctx, {Name:"zzz_..."})
  API->>BOARD: GET /v1/project_types?page=1&per_page=100&name=zzz_...
  BOARD-->>API: 200 OK + JSON array
  API-->>Test: raw ([]byte)
  Test->>Dump: dumpJSON(t, "project_types_search", 0, raw)
  Test->>Diff: StrictFieldDiff(t, raw, &[]ProjectTypeEntity{})
```

## TDD Test Design

### Unit（build tag なし、`project_types_test.go` 新規）

M02 の `accounting_types_test.go` と同形式で `http.RoundTripper` モックを使う。

| # | テストケース | 入力 | 期待出力 |
|---|-------------|------|---------|
| U1 | `ListProjectTypesRaw` 単一ページ | `[{"id":1,"name":"A",...}]` | 要素 1 の JSON 配列、キー列保持 |
| U2 | `ListProjectTypesRaw` 複数ページ連結 | per_page=2 で page1=2件、page2=1件 | 要素 3 の JSON 配列として valid |
| U3 | `GetProjectTypeRaw` 単体取得 | `{"id":42,"name":"Foo",...}` | body byte 完全一致、パス `/v1/project_types/42` |
| U4 | `SearchProjectTypesRaw` パラメータ送信 | name=keyword, updated_at_from=... | query string に両者が含まれ、body 返却 |
| U5 | `GetProjectTypeRaw` 404 | status=404 | `*APIError{Code: APIErrorNotFound}` |

### E2E（build tag `e2e`、`e2e_project_types_test.go` 新規）

1 test 1 endpoint。

| # | テスト名 | 呼び出す API | 検証内容 | req 数 |
|---|---------|-------------|---------|--------|
| E1 | `TestE2E_ProjectTypes_List` | `ListProjectTypesRaw` | dumpJSON + StrictFieldDiff(&[]ProjectTypeEntity{}) | 1 |
| E2 | `TestE2E_ProjectTypes_Get` | `ListProjectTypesRaw` で先頭 id 発見 → `GetProjectTypeRaw(id)` | dumpJSON + StrictFieldDiff(&ProjectTypeEntity{}) / List 0 件なら `t.Skipf` | 1-2 |
| E3 | `TestE2E_ProjectTypes_Search` | `SearchProjectTypesRaw({Name: "zzz_..."})` | dumpJSON + StrictFieldDiff | 1 |

**E1/E2/E3 は各々独立。`go test -run TestE2E_ProjectTypes_List -count=1` で単独実行可能。**

## 403/429 ハンドリング（運用ルール厳守）

- `skipIfNotFound` / `skipIfRateLimit` は呼ばない。
- `*APIError{Code: APIErrorRateLimit|APIErrorForbidden}` なら `t.Fatalf` 即失敗。
- RATE_LIMITED 時はロードマップ Blockers に記録、status: RATE_LIMITED で完了報告。

## データ依存 skip 規約

- List 0 件 → Get は `t.Skipf` で Pending Re-verification 入り（M02 で確立、ロードマップ運用ルール済）。
- Skip メッセージに `pending re-verification` を含める。

## ForceRefresh の扱い

本 M03 は boardapi 直呼び（cache を経由しない）のため、cache バイパスは構造的に保証される。

## Implementation Steps

- [x] **Red (Unit)**: `internal/boardapi/project_types_test.go` を新規作成（`package boardapi_test`）。U1–U5 を書く。**RoundTripper mock 方式**。Raw メソッド未実装で compile error → Red 確認済。
- [x] **Green (Unit)**: `internal/boardapi/project_types.go` に `ListProjectTypesRaw` / `GetProjectTypeRaw` / `SearchProjectTypesRaw` を追加。`accounting_types.go` の対応メソッドと完全同形式。U1–U5 全 Green。
- [x] **Refactor**: 既存関数との一貫性を保つ。過度な DRY 化は回避（accounting_types と同じ方針）。
- [x] **E2E 追加**: `internal/boardapi/e2e_project_types_test.go`（`//go:build e2e`）新規作成。E1-E3 を書く。`testhelper.StrictFieldDiff` と `dumpJSON` 利用。403/429 skip 禁止。List 0 件時 Get は `t.Skipf`。
- [x] `gofmt -s -l internal/boardapi/{project_types*.go,e2e_project_types_test.go}` 差分 0
- [x] `go vet ./...` 0 issue、`go vet -tags e2e ./...` 0 issue
- [x] Unit: `go test ./internal/boardapi/ -count=1 -run "TestListProjectTypesRaw|TestGetProjectTypeRaw|TestSearchProjectTypesRaw"` 5/5 Green
- [x] E2E: `go test -tags e2e -v -count=1 -run TestE2E_ProjectTypes ./internal/boardapi/` 実行、結果記録済
- [x] 本 plan の Implementation Steps / 結果記録を [x] に更新
- [x] `plans/board-compliance-roadmap.md` の M03 セクションを実測値に更新、未マップ発見 / Get 404 を記録
- [x] 対象ファイルを明示的に `git add` して commit（`.gitignore` で `/tmp/` 保護済）

## Commit Message（確定文言）

```
test(e2e): M03 project_types の List/Get/Search E2E を厳格フィールド突合付きで追加

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
```

## Verification

- `go test ./internal/boardapi/... -count=1` で unit 全 Green（新 U1-U5 + 既存）
- `go vet ./...` 0 issue
- `gofmt -s -l .` 差分 0
- `go test -tags e2e -v -count=1 -run TestE2E_ProjectTypes ./internal/boardapi/` で 3 テスト PASS/SKIP/PASS 以上
- 未マップフィールドが見つかった場合は Fail して結果セクションに記録（Entity 修正は M03 の Scope 外）

## Risks

| リスク | 影響度 | 対策 |
|--------|--------|------|
| Raw List の複数ページ連結で JSON が壊れる | 高 | U2 で 2 ページ応答をシミュレート、`json.Unmarshal` で要素数検証（M02 と同様） |
| `SearchProjectTypes` の API 側が name 無視 | 低 | E3 は存在しない name を使い、空配列 or 全件どちらも StrictFieldDiff は成立 |
| E2E 実行直前にアカウントの project_types が 0 件 | 中 | E2 は `t.Skipf` で Pending Re-verification 入り（運用ルール済） |
| 100 件超で List 複数ページ化 | 低 | project_types はマスタで 100 件超は非現実的。万一超えても per_page 調整で対応 |
| `ProjectTypeEntity` 未マップフィールド発見 | 計画通り | 本 M のゴール。結果セクションに記録、Entity 修正は別 commit |

## 事前見積と実測

| 区分 | 見積 | 実測 |
|------|------|------|
| Unit テスト (RoundTripper mock) | 0 req | **0 req**（mock のため BOARD API 非接続） |
| E1 List | 1 req | **1 req** FAIL（11 items 取得、3 未マップ検出） |
| E2 Get | 2 req（List 再呼 + Get）| **2 req** FAIL（Get は 404 = API 非対応） |
| E3 Search | 1 req | **1 req** FAIL（11 items 取得 = name フィルタ効かず、3 未マップ検出） |
| **合計** | **~4 req**（予備含め 5 req 以下）| **4 req** |

実行日時: 2026-04-20 17:19 JST

## 結果記録

### 発見した未マップフィールド（ProjectTypeEntity に欠落）
List / Search 両方で同じ 3 キー:

- `archive_flg`（アーカイブフラグ）
- `company_bank_id`（自社振込口座 ID）
- `company_bank_name`（自社振込口座名）

加えて、現行 `ProjectTypeEntity.Memo` フィールドは実 API レスポンスに存在しない（キー `memo` が返らない）。これも**不整合**として記録するが、本 M03 の Scope は「未マップ検知」であり、Entity 修正は別 commit / 別 M で対応（Out of Scope）。

### 実 API エンドポイントの仕様差異

- **GET /v1/project_types/{id} は 404 を返す**（id=48728974, List で取得した有効 ID）。個別 Get エンドポイントは API として存在しない可能性が高い。accounting_types の場合はデータ 0 件で Skip だったため発覚していなかった潜在問題。
- **name パラメータが無視される**: SearchProjectTypesRaw に `name=zzz_nonexistent_keyword_for_e2e` を渡しても 11 件（List と同一件数）返る。API 側が name フィルタを実装していない可能性。

### 実 API JSON サンプル（キー列挙のみ、値は掲載しない）
- `tmp/e2e-artifacts/project_types_0.json`: 2203 bytes, 11 items。各要素のキー = `archive_flg`, `company_bank_id`, `company_bank_name`, `created_at`, `id`, `name`, `updated_at`
- `tmp/e2e-artifacts/project_types_search_0.json`: 2203 bytes, 11 items（List と同一）。キー同上
- Get は 404 のため dump なし

### Blockers
なし（E2E は意図的に Fail 状態で記録、Entity 修正は Scope 外のため後続 M または別コミットで対応）

## Out of Scope（次 M 以降で扱う）
- M04: `payment_terms` に同パターンを適用
- M05: `document_send_channels`
- M06 以降: 他リソース
