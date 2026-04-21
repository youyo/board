# M25: FindClient 厳格化（Phase H 1 件目）

## Scope

Phase H（service/find 層）1 件目。
FindClient の結果に**欠損がない**ことを独立 raw API と突合して検証する。
実装変更と E2E テスト追加の両方を含む（M25 実施中に compliance 欠損を検出・修正）。

## 対象コード

| ファイル | 役割 |
|---------|------|
| `internal/service/find/find_client.go` | FindClient 実装（3 モード: ID/Name/Text + resolveClientDetails） |
| `internal/service/find/types.go` | ClientResult 型 |
| `internal/service/find/e2e_test.go` | E2E テスト |
| `internal/repository/client_branches.go` | ClientBranchRepository.Search（修正対象） |
| `internal/repository/contacts.go` | ContactRepository.Search（修正対象） |
| `internal/repository/client_branches_test.go` | T_R23 修正 |
| `internal/repository/contacts_test.go` | T_R37 修正 |

## API コール数

| テスト | リクエスト数 | 備考 |
|--------|------------|------|
| TestE2E_FindClient_ByName | ~3 | ListClients + FindClient(Name) + branches/contacts enrichment |
| TestE2E_FindClient_ByText | ~3 | ListClients + FindClient(Text=全件) + enrichment |
| TestE2E_FindClient_StrictEnrichment | ~4 | ListClientsPage(1) + FindClient(ID) + SearchClientBranchesRaw + SearchContactsRaw |
| **合計** | **~10 req** | |

## 実施内容

### タスク1: 失敗するテストの追加（Red フェーズ）

`internal/service/find/e2e_test.go` に以下を追加:
- `TestE2E_FindClient_StrictEnrichment`: FindClient(ID) の結果を
  `SearchClientBranchesRaw` / `SearchContactsRaw` と件数・ID 集合で突合
- `idSet` / `intSetEqual` helper 関数
- `TestE2E_FindClient_ByName` に integrity チェック追加
- `TestE2E_FindClient_ByText` に integrity チェック追加

### タスク2: Compliance 欠損の検出

`TestE2E_FindClient_StrictEnrichment` が失敗（Red）:
- FindClient(ID=51285623): branches=0, contacts=0
- SearchClientBranchesRaw: 10件
- SearchContactsRaw: 171件

### タスク3: Root Cause 分析

**原因**: `ClientBranchRepository.Search` / `ContactRepository.Search` が
`List` → in-memory filter パターンを使用。

BOARD API の `/v1/client_branches` / `/v1/contacts` は親クライアントを
フラット `client_id` フィールドでなく `{"client": {"id": N, ...}}` の
ネスト構造で返す（M09/M10 で記録済みの「逆方向不整合」）。

`ClientBranchEntity.ClientID` / `ContactEntity.ClientID` は
`json:"client_id"` タグを持つが、API レスポンスにフラットな `client_id`
フィールドは存在しないため unmarshal 後は常に 0 になる。

結果として `filterClientBranches(e.ClientID != params.ClientID)` が
全件 true → 全件除外 → branches=0, contacts=0 を返す。

### タスク4: Repository 修正（Green フェーズ）

`ClientBranchRepository.Search` と `ContactRepository.Search` を修正:
- `params.ClientID != 0` の場合は `api.SearchClientBranches` /
  `api.SearchContacts` を直接呼び出す（API-side filter）
- `Name`/`Email` のみのフィルタは従来通り `List` + in-memory filter
  （BOARD API は name/email パラメータを無視するため）
- 対応する unit test（T_R23 / T_R37）をモックデータ付きに修正

### タスク5: 既存コンパイルエラー修正（M35 波及漏れ）

`TestE2E_FindProject_WithEstimate` の `r.Estimate.Title` 参照を修正。
`EstimateEntity` に `Title` フィールドは存在しない。

## コミット

1. `fix(find): M35 波及漏れ - EstimateEntity.Title 参照を ID に修正` (1c60956)
2. `fix(repository): client_branches/contacts Search を API-side filter 経由に修正` (7a9a6a9)
3. `test(e2e): M25 FindClient の enrichment 厳格化 E2E を追加` (9a0154c)
4. `docs(plans): M25 FindClient 厳格化完了をロードマップに反映`

## 実績

- 実消費: ~10 req（ByName ~3 + ByText ~3 + StrictEnrichment ~4）
- E2E 結果（修正後）:
  - TestE2E_FindClient_ByName: **PASS** — branches=10, contacts=171
  - TestE2E_FindClient_ByText: **PASS**
  - TestE2E_FindClient_StrictEnrichment: **PASS** — branches=10 contacts=171（独立 API と一致）

## 発見事項

1. **Compliance 欠損（修正済み）**: ClientID フィルタ付き Search が in-memory
   filter の誤動作で全件除外していた（BOARD API のネスト構造対応漏れ）。
   fix(repository) で API-side filter 経由に変更し解消。

2. **残存 Entity 不整合（別 M 対応）**: `ClientBranchEntity.ClientID` /
   `ContactEntity.ClientID` は API レスポンス上 0 のまま（ネスト構造の
   `client.id` にマップされていない）。integrity チェック
   `b.ClientID != 0 && b.ClientID != targetID` は ClientID==0 で
   短絡評価されるため現時点は通過する。Entity 全面改訂マイルストーンで対応。

3. **潜在バグ（M25 スコープ外）**: VendorBranchRepository /
   VendorContactRepository も同じ in-memory filter パターンを持つが、
   当該アカウントにデータがないため表面化していない。

4. **EstimateEntity.Title コンパイルエラー（修正済み）**: M35 の波及漏れ。
   `-tags e2e` ビルドでコンパイルエラーになる問題を修正。
