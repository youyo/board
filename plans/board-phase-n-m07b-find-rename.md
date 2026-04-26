# N07b 詳細計画: 旧 find/ 削除 + find2/ → find/ rename

## Meta

| 項目 | 値 |
|------|---|
| マイルストーン | N07b |
| 親ロードマップ | `plans/board-phase-n-roadmap.md` |
| 関連 ADR | `docs/adr/ADR-001-find-layer.md`（B 採択、Group 削除確定） |
| 前提 | N07a 完走（find2 11 メソッド = 192 unit test PASS） |
| 革新ステータス | Ready |
| 作成日 | 2026-04-27 |

## ゴール

- 旧 `internal/service/find/` を全削除し、`internal/service/find2/` を `internal/service/find/` に rename。
- CLI / MCP は `service/find2` への切替に伴う **コンパイル維持のための最小修正**のみ実施（フィーチャー parity は N07c/N08 で対応）。
- `go test -race ./...` / `go vet ./...` / `gofmt -s -w .` が全 green。
- 1 コミット完結 revertable（`git revert` で完全戻し可能）を維持する。

## Non-Goals

- CLI command の刷新（N07c）
- MCP tool schema の刷新 / 11 tool 化（N08）
- `find_groups` 機能の置き換え案の確定（ADR-001 で削除確定済み、後続再評価不要）
- E2E 再構築（N09）

## 設計判断

### 採択: 案 C（1 コミットで一括 git mv + import 一括置換）

- **案 A 旧 find 削除 → find2 rename**: 中間状態でビルド不可。1 コミット完結を破る。
- **案 B 別名退避 → rename → 別名削除**: 不要なファイル移動コスト、coverage が一時的に薄まる。
- **案 C 1 コミット一括**: 純粋に Best。`git mv` で履歴も保てる（Go では package 名と import path の同時変更が必須なため、結局 sed 置換が中心）。

### CLI/MCP の暫定対応方針（advisor 確認済み）

旧 find と find2 の Query 型は構造が違う:

| 旧 find | find2 | 影響 |
|---------|-------|------|
| `Limit:` / `Opts:` トップレベル | `FindCommonOpts` 埋め込み | 全 12 CLI / 12 MCP tool で書き換え |
| `FindGroupQuery` 等 Group 系 | 削除（ADR-001） | `cli/find_group.go` 削除、MCP `find_groups` tool 削除 |
| Project: `ClientName string` | `ClientID int` only | CLI フラグは残す、RunE で reject |
| Estimate/Order/Delivery/Receipt: `Status` `ClientName` `ProjectName` | 該当フィールドなし | 同上 reject |
| Invoice: `ClientName` `ProjectName` | `ClientID` only | 同上 reject |
| PurchaseOrder: `VendorName` `ProjectName` | `VendorID` only | 同上 reject |
| Payment: `VendorName` `PurchaseOrderID` | `VendorID` only | 同上 reject |

**方針**: cobra のフラグ宣言は維持（`--help` 出力の見た目を温存）し、RunE 冒頭で「フラグが N07b では未対応」エラーを返す。N07c で各フラグの正しい配線（name → ID 解決等）を実装する。

### MCP tool schema の扱い

- `find_groups` tool は削除（ADR-001 forced）。
- 残り 11 tool の input schema（`mcp.WithString("client_name", ...)` 等）は **そのまま残す**（schema 変更は N08）。Handler 内では未対応引数が渡された時に error result を返す。
- `tools_test.go` の 12 tool 期待値を 11 に修正、`find_groups` を expected list から外す。

### app.go の差し替え

- `FindService2()` 削除
- 新規 `FindService()` を追加（旧 find の `New()` ではなく find2 の `New()` を使う、`find.Repos` を find2 の Repos 構造で渡す）
- `cli/find.go::findServiceFromCmd` も `repos.Groups` 削除

## 実装手順

### Step 1: 旧 find/ 削除

```bash
rm -rf internal/service/find/
```

47 E2E test + unit test が消滅。N02 で代替設計済み（N09 で再構築）。

### Step 2: find2/ → find/ rename

```bash
git mv internal/service/find2 internal/service/find
```

### Step 3: 中の `package find2` → `package find` 一括置換

対象: `internal/service/find/*.go` 全ファイル
- `package find2` → `package find`
- import `"github.com/youyo/board/internal/service/find2"` → `".../service/find"`
- 自パッケージ内で `find2.` 参照は無いはず（再確認）

### Step 4: slog warn メッセージ "find2." → "find." 置換

```
find2.FindEstimate: ... → find.FindEstimate: ...
find2.FindDelivery: ... → find.FindDelivery: ...
find2.FindOrder: ...
find2.FindReceipt: ...
find2.FindPayment: ...
find2.lookupClient: ...
find2.resolveClientAndProject: ...
find2.resolveVendorAndProject: ...
find2.resolveProjectClient: ...
find2.resolveClientDetails: ...
find2.resolveVendorDetails: ...
find2.reverseMapper: ...
```

テスト側 (`*_test.go`) の期待文字列も同時置換（`find2.FindEstimate: project enrichment failed` → `find.FindEstimate: project enrichment failed` 等）。

### Step 5: app.go 修正

- import `service/find2` → `service/find`
- `FindService2()` → `FindService()` rename + コメント更新（暫定アクセサではなくなる）
- 戻り値 `*find2.Service` → `*find.Service`
- `find2.New(find2.Repos{...})` → `find.New(find.Repos{...})`

### Step 6: cli/find.go 修正

- `Groups: repos.Groups,` 行を削除（find2.Repos に Groups なし）
- `NewFindGroupCmd()` の AddCommand 削除
- import path はそのまま（`service/find` を引き続き使う）

### Step 7: cli/find_group.go 削除

```bash
rm internal/cli/find_group.go
```

### Step 8: 各 CLI find_*.go の Query 構築コード書き換え

埋め込み `FindCommonOpts` 形式へ。+ 未対応フラグの reject。

例 (find_client.go):

```go
q := find.FindClientQuery{
    FindCommonOpts: find.FindCommonOpts{
        Limit: opts.Limit,
        Opts: repository.ReadOptions{
            Refresh:      opts.Refresh,
            ForceRefresh: opts.ForceRefresh,
        },
    },
    ID:   id,
    Name: name,
    Text: text,
}
```

未対応フラグ reject の追加:

| ファイル | reject フラグ |
|----------|---------------|
| find_project.go | `--client-name`, `--status` |
| find_estimate.go | `--client-name`, `--project-name`, `--status` |
| find_order.go | 同上 |
| find_delivery.go | 同上 |
| find_receipt.go | 同上 |
| find_invoice.go | `--client-name`, `--project-name`, `--status`（注: find2 は Status を持つので status は通せる） |
| find_purchase_order.go | `--vendor-name`, `--project-name` |
| find_payment.go | `--vendor-name`, `--purchase-order-id` |

エラーメッセージ統一テンプレ:
```
"--<flag> is not supported in v0.7.0 service rename (N07b); will be wired in N07c"
```

**修正**: invoice / purchase_order / payment は find2 でも Status / Statuses を持つので `--status` フラグは通せる。要慎重マッピング:
- find_invoice.go: `--client-name`, `--project-name` reject。`--status` は OK（FindInvoiceQuery.Status が存在）。
- find_purchase_order.go: `--vendor-name`, `--project-name` reject。`--status` は OK（FindPurchaseOrderQuery.Status）。
- find_payment.go: `--vendor-name`, `--purchase-order-id` reject。`--status` は OK（FindPaymentQuery.Status）。

そして N07a で確立された Statuses-only narrowing reject は CLI からは Statuses を渡さないので影響なし。

ID/Text のみで動作する FindInvoice/PO/Payment は Service.validate を通る（Status 単独 or Statuses[] 単独で reject される範囲）。

### Step 9: mcpserver/server.go / tools.go 修正

- import `service/find` を引き続き使う（rename 後）
- `find_groups` tool 削除
- `RegisterTools` 内 `findGroupsTool(s),` 行削除
- `findGroupsTool` 関数自体削除
- 残り 11 tool の handler 内 Query 構築を `FindCommonOpts` 埋め込みに変更
- 未対応引数（`client_name`/`project_name`/`vendor_name`/`purchase_order_id` 等）は schema は残しつつ handler で受け取った場合は error result 返却

### Step 10: mcpserver/tools_test.go 修正

- expected tool count 12 → 11
- `expected []string` から `"find_groups"` 削除
- `simpleCases` から `"find_groups"` 削除

### Step 11: boardapi/e2e_find_layer_poc_test.go の参照確認

`grep -n find2` 出力にあったため確認:
```bash
grep -n "find2" internal/boardapi/e2e_find_layer_poc_test.go
```
コメント中の `find2` 言及であれば残置（履歴 reference として）または `find` に置換。実コード参照ならテスト build tag (`e2e`) の関係で実害は無いがファイル冒頭のドキュメンテーションコメントとして更新する。

### Step 12: repository コメント中の `service/find` 参照

`internal/repository/clients.go:128` / `projects.go:139` のコメント。新しい `service/find` を引き続き指すので変更不要。

### Step 13: 動作確認

```bash
go vet ./...
go test -race -count=1 ./...
gofmt -s -w .
```

E2E は build tag `e2e` 付きの場合のみ走るので unit test の green を完了条件とする（rate limit 配慮）。

### Step 14: docs / ADR 更新

- `docs/adr/ADR-001-find-layer.md`: Implementation status に N07b 完了記録を追加
- `docs/specs/board_cli_mcp_ultra_detailed_design_ja.md`: §7.9 / §22 で `find2/` を `find/` と表記している箇所があれば修正（実際には Placeholder で抽象化されているはず）
- README.md: find2 言及がないことを `grep` で確認

### Step 15: コミット

```
refactor(find): N07b — 旧 find/ 削除 + find2/ → find/ rename

- internal/service/find/ を全削除（47 E2E + unit、N02/N09 で代替設計済み）
- internal/service/find2/ → internal/service/find/ rename
- package find2 → package find（slog warn メッセージ含む）
- app.go: FindService2() → FindService()
- cli/find_group.go 削除（ADR-001 Group 削除確定）
- mcpserver: find_groups tool 削除、12 → 11 tool
- CLI: Query 構築を FindCommonOpts 埋め込みに移行、未対応フラグは RunE で reject（N07c で配線予定）
- MCP: handler で未対応引数を error 返却（schema は N08 で整理）

1 コミット完結 revertable。go test -race ./... pass。
```

## Rollback Plan

```bash
git revert <commit-hash>
```

1 コミットで完結するため一発で完全復旧可能。

## Risk 評価

| リスク | 緩和 |
|--------|------|
| import path 変更漏れ | コンパイルエラーで即時検知 |
| Query 型書き換え漏れ | コンパイルエラー（FindCommonOpts は struct embed） |
| slog 警告文字列 update 漏れ | テスト assertion でキャッチ |
| 未対応フラグ reject 動作不整合 | unit test 追加（オプション、CLI 詳細は N07c で再整備） |
| rate limit 影響 | 本マイルストーンに E2E 実行は不要（unit test only） |
| MCP find_groups 利用者への影響 | N07b コミットメッセージで Breaking 明示、N08 でも CHANGELOG に再記載 |

## 完了基準

- [ ] 旧 `internal/service/find/` 配下が `find2` 内容で置き換わっている
- [ ] `internal/service/find2/` ディレクトリが消えている
- [ ] `cli/find_group.go` が消えている
- [ ] `mcpserver/tools.go` から `findGroupsTool` 関数 + 登録が消えている
- [ ] `tools_test.go` の expected が 11 tool になっている
- [ ] `go vet ./...` PASS
- [ ] `go test -race -count=1 ./...` 全 PASS
- [ ] `gofmt -s -w .` 差分なし
- [ ] 1 コミット完結
