# M02: accounting_types 完走（List / Get / Search E2E + 厳格フィールド突合）

## Overview
| 項目 | 値 |
|------|---|
| ステータス | 実装完了（unit 5/5 Green） / 実 API E2E は実行環境の TLS 問題で未検証 |
| 依存 | M01（testhelper.StrictFieldDiff, dumpJSON, tmp/）完了済み |
| 見積 API 呼び出し | **~5 req**（うち実体は `List 1ページ + Get 1件 + Search 1ページ = 3 req` + 予備 2 req）|
| 対象ファイル | `internal/boardapi/accounting_types.go`（追記: Raw メソッド 3 本）, `internal/boardapi/accounting_types_test.go`（新規: Raw メソッド単体 TDD）, `internal/boardapi/e2e_accounting_types_test.go`（新規）|

## Goal
- **all 22 リソースの先頭**として `accounting_types` を L / G / S で E2E 通過させ、`StrictFieldDiff` による未マップフィールド検知が実 API JSON で機能することを確認する。
- Raw 版 API メソッド（`ListAccountingTypesRaw`, `GetAccountingTypeRaw`, `SearchAccountingTypesRaw`）を追加し、**M03 以降で踏襲する「Raw 層の追加パターン」** を確立する。
- 実 API の JSON が `AccountingTypeEntity` と完全一致していることを証明するか、未マップフィールドを炙り出す。

## 非ゴール
- `repository.AccountingTypeRepository` 層の変更（cache バイパスは E2E では boardapi を直接叩くため不要）
- 他 22 リソースへの Raw 追加（M03 以降で 1 リソースずつ）
- Refactor による既存 `ListAccountingTypes` / `GetAccountingType` の削除（互換維持、Raw は並列追加）

## Architecture Decision: Raw メソッドの責務分離

### 決定
Raw メソッドは既存 Entity 返し API を呼ばず、**独立した HTTP 呼び出しロジックを持つ**。

### 理由
- `ListAccountingTypes` は既に `ListAll` 内で `[]json.RawMessage` を得てから unmarshal している。これを途中で横取りすると互換破壊になる。
- `StrictFieldDiff` は「raw JSON → map[string]any → struct tag 突合」の経路で動くため、**実 HTTP body を 1 byte も加工せず受け取る必要がある**。struct にシリアライズした後で `json.Marshal` し直すと round-trip で未マップ検知が効かない（M01 ハンドオフ注意点 #1）。
- List / Search は ページごとの body（JSON 配列）を `json.RawMessage` として保持し、**1 ページ分の生 body をそのまま配列としてテストに渡す**（`[{...},{...}]` 形式）。複数ページでも同形式に連結して正しい JSON として返せる。
- Get は `DoWithRetry(req)` の返り値がそのまま生 body なので、**追加加工なしで byte を返す**。

### API シグネチャ（確定）

```go
// 全て新規関数として追加（既存は無変更）

// GetAccountingTypeRaw returns the raw HTTP response body for GET /v1/accounting_types/{id}.
// Intended for E2E strict field diff; regular callers should use GetAccountingType.
func (c *Client) GetAccountingTypeRaw(ctx context.Context, id int) ([]byte, error)

// ListAccountingTypesRaw returns the raw HTTP response body of GET /v1/accounting_types
// merged across all pages as a single JSON array.
// Intended for E2E strict field diff.
func (c *Client) ListAccountingTypesRaw(ctx context.Context) ([]byte, error)

// SearchAccountingTypesRaw returns the raw HTTP response body of GET /v1/accounting_types
// with the given search params, merged across all pages as a single JSON array.
// Intended for E2E strict field diff.
func (c *Client) SearchAccountingTypesRaw(ctx context.Context, params AccountingTypeSearchParams) ([]byte, error)
```

### Raw List/Search の内部実装方針

```go
// 疑似コード
items, err := c.ListAll(ctx, makeReq) // -> []json.RawMessage
// 既存の ListAll は per-page の JSON array を展開して []json.RawMessage で返す。
// これを json.Marshal すると [...] 形式の valid JSON になる。
return json.Marshal(items)
```

`json.RawMessage` は MarshalJSON 実装により**元の byte をそのまま吐く**。よって `json.Marshal([]json.RawMessage{...})` は「各要素の元 byte をそのまま並べた JSON 配列」になる。キーの並び・空白・数値表現が 1 byte も変わらない点が本設計の肝。

## Sequence Diagram

```mermaid
sequenceDiagram
  participant Test as E2E Test
  participant API as boardapi.Client
  participant BOARD as BOARD API
  participant Dump as dumpJSON
  participant Diff as StrictFieldDiff
  participant FS as tmp/e2e-artifacts/

  Test->>API: ListAccountingTypesRaw(ctx)
  API->>BOARD: GET /v1/accounting_types?page=1&per_page=100
  BOARD-->>API: 200 OK + JSON array
  API-->>Test: raw ([]byte; `[{...},{...}]`)
  Test->>Dump: dumpJSON(t, "accounting_types", 0, raw)
  Dump->>FS: write accounting_types_0.json
  Test->>Diff: StrictFieldDiff(t, raw, &[]AccountingTypeEntity{})
  Diff-->>Test: unmapped keys (should be empty)

  Test->>API: GetAccountingTypeRaw(ctx, firstID)
  API->>BOARD: GET /v1/accounting_types/{id}
  BOARD-->>API: 200 OK + JSON object
  API-->>Test: raw ([]byte)
  Test->>Dump: dumpJSON(t, "accounting_types", id, raw)
  Test->>Diff: StrictFieldDiff(t, raw, &AccountingTypeEntity{})
  Diff-->>Test: unmapped keys

  Test->>API: SearchAccountingTypesRaw(ctx, {Name:""})
  API->>BOARD: GET /v1/accounting_types?page=1&per_page=100
  BOARD-->>API: 200 OK + JSON array
  API-->>Test: raw ([]byte)
  Test->>Dump: dumpJSON(t, "accounting_types_search", 0, raw)
  Test->>Diff: StrictFieldDiff(t, raw, &[]AccountingTypeEntity{})
```

## TDD Test Design

### Unit（build tag なし、`accounting_types_test.go` 新規）

| # | テストケース | 入力 | 期待出力 |
|---|-------------|------|---------|
| U1 | `ListAccountingTypesRaw` が httptest server 応答の JSON 配列をそのまま連結して返す | 1 ページ応答 `[{"id":1,"name":"A"}]` | `[{"id":1,"name":"A"}]`（byte 一致） |
| U2 | `ListAccountingTypesRaw` の 複数ページ連結 | page1=100件、page2=1件 | 要素数 101 の配列 JSON として valid、各要素は元 byte 保持 |
| U3 | `GetAccountingTypeRaw` が body をそのまま返す | `{"id":1,"name":"A","memo":"m"}` | 完全一致 byte |
| U4 | `SearchAccountingTypesRaw` が name パラメータを送る | name=foo | reqURL query に `name=foo` が含まれる & body 返却 |
| U5 | `GetAccountingTypeRaw` の 404 は APIError に wrap | 404 応答 | `*APIError{Code: APIErrorNotFound}` |

### E2E（build tag `e2e`、`e2e_accounting_types_test.go` 新規）

1 test 1 endpoint の粒度。

| # | テスト名 | 呼び出す API | 検証内容 | req 数 |
|---|---------|-------------|---------|--------|
| E1 | `TestE2E_AccountingTypes_List` | `ListAccountingTypesRaw` | dumpJSON + StrictFieldDiff(&[]AccountingTypeEntity{}) で未マップ 0 | 1 (per_page=100 でアカウント内全件が 1 ページ内に収まる想定) |
| E2 | `TestE2E_AccountingTypes_Get` | `ListAccountingTypesRaw` で先頭 id 発見 → `GetAccountingTypeRaw(id)` | dumpJSON + StrictFieldDiff(&AccountingTypeEntity{}) | 2 |
| E3 | `TestE2E_AccountingTypes_Search` | `SearchAccountingTypesRaw({Name: "zzz_nonexistent"})` | dumpJSON + StrictFieldDiff（0 件でも JSON `[]` として valid） | 1 |

**E1/E2/E3 は各々独立。`go test -run TestE2E_AccountingTypes_List -count=1` で単独実行可能。**

## 403/429 ハンドリング（運用ルール厳守）

- E2E テスト内で**skip を使わない**。
- `skipIfNotFound` / `skipIfRateLimit` は M02 の新規 E2E では**呼ばない**（ロードマップ運用ルール #4「403/429 即停止」）。
- エラーが `*APIError{Code: APIErrorRateLimit}` または `APIErrorForbidden` なら `t.Fatalf` で即失敗。
- 失敗した場合は plan と roadmap の `Blockers` に記録、status: RATE_LIMITED で完了報告。

```go
// 実装パターン（skip 版は使わない）
raw, err := client.ListAccountingTypesRaw(ctx)
if err != nil {
    t.Fatalf("ListAccountingTypesRaw: %v", err)
}
```

## ForceRefresh の扱い

ロードマップ運用ルール #2「最初の 1 リソースは `ForceRefresh: true`」は `repository` 層の読み出し方針。
**本 M02 は boardapi 直呼び（cache を経由しない）** のため、cache バイパスは構造的に保証される。Repository 層経由のキャッシュ動作は後続 M（M25 以降の service/find）で別途検証する。

## Implementation Steps

- [x] **Red (Unit)**: `internal/boardapi/accounting_types_test.go` を新規作成し、U1–U5 を書く。**sandbox で httptest.NewServer が listen できないため、`http.RoundTripper` モック方式に切り替え**（既存 client_test.go の httptest 方式とは独立）。各テストで実在しない関数呼び出し → compile error (Red)。
- [x] **Green (Unit)**: `internal/boardapi/accounting_types.go` に `ListAccountingTypesRaw` / `GetAccountingTypeRaw` / `SearchAccountingTypesRaw` を追加。`ListAccountingTypesRaw`/`SearchAccountingTypesRaw` は `...ListAllOption` を受けるため `WithPerPage` を test で注入可能。Get は `DoWithRetry(req)` の返り値をそのまま返すだけのシンプル実装。U1–U5 全 Green。
- [x] **Refactor**: 既存関数とのシグネチャ一貫性を保ちつつ、Raw 版は独立経路で実装。過度な DRY 化は回避。
- [x] **E2E 追加**: `internal/boardapi/e2e_accounting_types_test.go`（`//go:build e2e`）を新規作成し E1–E3 を書いた。`testhelper.StrictFieldDiff` と `dumpJSON` 利用。403/429 の skip は行わず `t.Fatalf` で即停止。
- [x] `gofmt -s -l <M02 files>` 差分 0、`go vet ./...` 0 issue、`go vet -tags e2e ./...` 0 issue
- [x] Unit: `go test ./internal/boardapi/ -count=1 -run "TestListAccountingTypesRaw|TestGetAccountingTypeRaw|TestSearchAccountingTypesRaw"` 5/5 Green
- [ ] **実 API E2E は sandbox 環境で TLS 検証不能（OSStatus -26276）**: ユーザー手元で下記を実行してもらう必要あり
  - `BOARD_API_KEY=... BOARD_API_TOKEN=... go test -tags e2e -v -count=1 -run TestE2E_AccountingTypes_List ./internal/boardapi/`
  - 続けて `-run TestE2E_AccountingTypes_Get`, `-run TestE2E_AccountingTypes_Search`
  - `tmp/e2e-artifacts/accounting_types_*.json` が生成されることを確認
- [x] 本 plan の Implementation Steps を [x] に更新、`plans/board-compliance-roadmap.md` の M02 セクションを更新
- [x] 対象ファイルを明示して commit（`.gitignore` で `/tmp/` が守られているため顧客データが混入する心配なし）

## Commit Message（確定文言）

```
test(e2e): M02 accounting_types の List/Get/(Search) E2E を厳格フィールド突合付きで追加

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
```

## Verification

- `go test ./internal/boardapi/... -count=1` で unit 全 Green（新 U1-U5 + 既存）
- `go vet ./...` 0 issue
- `gofmt -s -l .` 差分 0
- `go test -tags e2e -v -count=1 -run TestE2E_AccountingTypes ./internal/boardapi/` で 3 テスト全 Green
- 未マップフィールドが見つかった場合はテスト Fail。plan の「結果」セクションに記録し、`AccountingTypeEntity` への追加は**本 M の Scope 外**（別 commit / 別 M で対応）。

## Risks

| リスク | 影響度 | 対策 |
|--------|--------|------|
| Raw List の複数ページ連結で JSON が壊れる | 高 | U2 で 2 ページ応答をシミュレートし、要素数とキー保持をバイト単位でなく `json.Unmarshal` で検証 |
| `SearchAccountingTypes` の API 側が Search param を無視した場合 Name 絞り込みが空にならず実全件ヒットする | 低 | E3 では存在しない name を使い、空配列が返ることを検証（絞り込みが効かないとしてもテストは通る） |
| E2E 実行直前にアカウントの accounting_types が 0 件 | 中 | E2 は `len(entities) == 0` の場合 t.Fatalf（M02 は skip 禁止）。ただし accounting_types はマスタのため 0 件は運用異常として扱う |
| 100 件超で List が複数ページになり 3 req 想定を超過 | 低 | アカウント規模から accounting_types が 100 件超は非現実的。万一超えたら per_page=500 で再実行（+1 req だが予備枠 2 req 以内） |
| `AccountingTypeEntity` が実 API と不一致で Fail | 計画通り | 本 M のゴール。plan に「発見した未マップフィールド」を記録し次 commit で修正（M02 外の commit） |
| `[]AccountingTypeEntity` への StrictFieldDiff 呼び出しが M01 の挙動通り配列として動くか | 中 | M01 テスト #6 で `[]any` → slice の突合が Green なので動作確認済み |

## 事前見積と実測

| 区分 | 見積 | 実測 |
|------|------|------|
| Unit テスト (RoundTripper mock) | 0 req | **0 req**（mock のため BOARD API 非接続） |
| E1 List | 1 req | 未計測（sandbox TLS 問題で実 API 未達） |
| E2 Get | 2 req | 未計測 |
| E3 Search | 1 req | 未計測 |
| **合計** | **~4 req**（予備含め 5 req 以下） | **0 req（unit のみ）** |

### 実 API 未計測の理由
sandbox 環境で Go runtime の macOS native TLS 検証が失敗する（`tls: failed to verify certificate: x509: OSStatus -26276`）。curl は同一ホスト `api.the-board.jp` に対して 403 を正しく返しており、ネットワーク到達性・証明書自体は有効。`SSL_CERT_FILE=/etc/ssl/cert.pem` / `GODEBUG=x509usefallbackroots=1` でも解消せず、これはインフラ側制約で M02 実装とは独立した課題。ユーザー手元（sandbox 外）での E2E 実行で検証完了できる。

## 結果記録（実装後 Fill）

### 発見した未マップフィールド
（実装後記入。無ければ「なし」）

### 実 API JSON サンプル（キー列挙のみ、値は掲載しない）
（実装後記入。`tmp/e2e-artifacts/accounting_types_*.json` の top-level キーをここに列挙）

### Blockers
- **sandbox 環境での実 API E2E 未実行**: Go 1.26 + macOS の native TLS verifier が sandbox で OSStatus -26276 を返すため、`go test -tags e2e` で実 API に到達できない。unit は RoundTripper mock で 5/5 Green、E2E コードは `go vet -tags e2e ./...` で compile 可能。ユーザー手元（sandbox 外）での 1 回実行で検証完了する想定。403/429 由来ではないため RATE_LIMITED ではなく、**環境依存のインフラ制約**として本ファイル / ロードマップに記録。

## Out of Scope（次 M 以降で扱う）
- M03: `project_types` に同パターンを適用
- M04: `payment_terms`
- M05: `document_send_channels`
- M06 以降: 他リソース
