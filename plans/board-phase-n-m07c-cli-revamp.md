# N07c 詳細計画: CLI/MCP の name → ID 解決配線 + 未対応フラグ整理

## Meta

| 項目 | 値 |
|------|---|
| マイルストーン | N07c |
| 親ロードマップ | `plans/board-phase-n-roadmap.md` |
| 関連 ADR | `docs/adr/ADR-001-find-layer.md`（B 採択、Group 削除確定） |
| 前提 | N07b 完走（旧 find/ 削除 + find2/ → find/ rename） |
| 作成日 | 2026-04-27 |
| ステータス | Ready |

## ゴール

N07b で `RunE` / handler 内で reject していた未対応フラグを段階的に動作させる。
具体的には `name → ID` 解決のみが必要な 4 種を **CLI/MCP 共通の resolver** で配線し、
構造的に対応不可能なフラグは「未対応」エラー文言を「resource does not support …」に書き直して
最終形に固める。`board find <sub>` 階層化は N07b 時点で完了済みなので scope 外。

## Non-Goals

- `board find_*` flat → `board find <sub>` 階層化（N07b で既に完了済、今回は再確認のみ）
- MCP tool input schema の刷新（N08）
- E2E 再構築（N09）
- 動的補完（Phase H で out of scope 確定）
- find_project の `--status` alone を CLI 側で早期 reject（service.validate() で既に reject 済、DRY 違反になるため不要）

## Scope 確定（advisor + self-review 反映）

### CLI 階層化の現状

`internal/cli/find.go` の `NewFindCmd()` で既に `board find <sub>` の階層化が完了している。
N07c では subcommand 構造に手を入れない（見直すと breaking change を再生成する）。

### name → ID 解決を配線する対象（4 件）

service 側が `*ID` フィールドのみ受ける構造で、CLI のフラグ名解決経路が必要なもの。

| CLI flag | service field | resolver |
|----------|---------------|----------|
| `find_project --client-name` | `FindProjectQuery.ClientID` | `resolveClientByName` |
| `find_invoice --client-name` | `FindInvoiceQuery.ClientID` | `resolveClientByName` |
| `find_purchase_order --vendor-name` | `FindPurchaseOrderQuery.VendorID` | `resolveVendorByName` |
| `find_payment --vendor-name` | `FindPaymentQuery.VendorID` | `resolveVendorByName` |

### 値を service に pass-through するだけで済む対象（4 件 × 各 2 フラグ）

service 層の Find\*Query が既に `ClientName` / `ProjectName` フィールドを持ち、internal で resolver を呼んで
fanout 検索する Document 4 種（Estimate / Order / Delivery / Receipt）。

| CLI flag | service field | 必要な作業 |
|----------|---------------|------------|
| `find_estimate --client-name` | `FindEstimateQuery.ClientName` | reject 解除、Query に pass |
| `find_estimate --project-name` | `FindEstimateQuery.ProjectName` | reject 解除、Query に pass |
| `find_order --client-name` / `--project-name` | `FindOrderQuery.{ClientName,ProjectName}` | 同上 |
| `find_delivery --client-name` / `--project-name` | `FindDeliveryQuery.{ClientName,ProjectName}` | 同上 |
| `find_receipt --client-name` / `--project-name` | `FindReceiptQuery.{ClientName,ProjectName}` | 同上 |

### 構造的不可能 reject 2 件（does not support）

Entity 自体にフィールドが無く、将来的にも実装不可能なもの。

| CLI flag | 理由 | 最終エラー文言 |
|----------|------|----------------|
| `find_estimate/order/delivery/receipt --status` | Document Entity に Status フィールドなし | `--status filtering is not supported for documents (no Status field on entity)` |
| `find_payment --project-name` | PaymentEntity に ProjectID なし（D1） | `--project-name is not supported for payments (no project_id on entity)` |

### 将来拡張で対応可能 reject 3 件（not yet supported）

API filter は存在するが service Query が現状 expose していない、または service Query にフィールドが無いだけ。
将来 service 拡張で対応できる。

| CLI flag | 現状 | 最終エラー文言 |
|----------|------|----------------|
| `find_invoice --project-name` | API: `InvoiceListOptions.ProjectIDEq` あり。service Query に `ProjectName`/`ProjectID` 不在 | `--project-name is not yet supported for invoices (tracked for future enhancement)` |
| `find_purchase_order --project-name` | API: `PurchaseOrderListOptions.ProjectIDEq` あり。service Query に同フィールド不在 | `--project-name is not yet supported for purchase orders (tracked for future enhancement)` |
| `find_payment --purchase-order-id` | API: `PaymentListOptions.PurchaseOrderIDEq` あり。service Query に同フィールド不在 | `--purchase-order-id is not yet supported (tracked for future enhancement)` |

これらは N09/N10 以降で service 拡張すれば動かせる旨を残留 TODO 節に記録する。

### find_project --status alone の扱い

CLI 早期 reject は **追加しない**（advisor 助言）。理由:
- types.go:89-93 の `FindProjectQuery.validate()` が既に reject しており、CLI 側で重複させると DRY 違反 + メンテ二重化
- service 由来エラー（`Status/Statuses requires at least one of ID, Name, ClientID, or Text`）はそのまま CLI に流れて表示される
- ユーザーが `--client-name` のみ + `--status` を組み合わせれば narrowing が成立する → N07c で `--client-name` を ClientID 解決配線することでこのケースは自動的に動く

## 設計決定

### D1: name 解決は find package に置く（advisor 助言 4 採択）

`internal/service/find/resolver.go` に既に存在する resolver helper（`resolveProjectClient` 等の
private 関数）の隣に、**新規 public 関数として `ResolveClientByName` / `ResolveVendorByName` を追加する**。

理由:
- find package は CLI / MCP 両方から既に import 済（追加 import 不要）
- 重複ヒット検出ロジックは cross-resource search の前段として一貫した責務
- find package 内の既存 stub repo パターン（`helpers_test.go` の `stubClientRepo` 等）を再利用して
  resolver の unit test を 1 度だけ書けば良い
- CLI/MCP の test は「flag 定義 / 階層 / required-flag error」のみで、resolver 経路自体は find package で検証

resolver のシグネチャ（service receiver メソッドにする → repo へのアクセスが Service 内 private fields で済む）:

```go
// internal/service/find/resolver.go に追加
func (s *Service) ResolveClientByName(ctx context.Context, name string, opts repository.ReadOptions) (int, error)
func (s *Service) ResolveVendorByName(ctx context.Context, name string, opts repository.ReadOptions) (int, error)
```

CLI/MCP の handler は `svc.ResolveClientByName(ctx, name, opts)` を呼んで `int` を取り、
それを Query.ClientID に詰めるだけ。test は find package 内で 1 度書けば済む。

### D2: 重複ヒット時の挙動

NameCont は部分一致のため複数ヒットが起こり得る（一意ではない）。
**silent take-first は禁止**（CLI ツールがバグる典型）。代わりに以下のセマンティクス:

- 0 件 → error: `no client matches name "<query>"`
- 1 件 → ID 採用、resolver 完了
- 複数 → error + 候補列挙: `multiple clients match name "<query>": [{id:1, name:"…"}, …]; use --id to disambiguate`

候補列挙は最大 5 件まで（rate limit と error 表示のバランス）。

CLI: stderr に出力、exit code 非 0。
MCP: `errorResult` で返却、message に同等の候補列挙 JSON を入れる。

### D3: キャッシュ戦略

毎コマンド毎に `clients.Search(NameCont)` / `vendors.Search(NameCont)` を呼ぶ。
理由（advisor 助言反映）:
- 1 コマンド 1 回、3 req/sec rate limit に対して無視できる頻度
- repository 層 cache が透過的に効くため 2 回目以降は cache hit
- 専用 cache は over-engineering（profile 別 / TTL 等の無駄な複雑性が増える）

### D4: resolver は Service receiver メソッド（D1 統合）

D1 で確定。実装は `internal/service/find/resolver.go` の既存ファイルに追記。
private fields `s.clients` / `s.vendors` を直接使えるため accessor 追加不要。

### D5: --status alone は CLI 早期 reject 不要

advisor 助言。types.go:89-93 が既に reject。CLI 側追加は DRY 違反。

### D6: enrichment non-fatal の breaking change 告知

CHANGELOG / api-reference に簡潔に告知:
- `Project` / `Client` / `Vendor` フィールドが nil で返ることがあり得る（network 失敗、API timeout 等）
- ロード元の主検索 entity は必ず返る（fail-fast）
- 詳細は ADR-001 へリンク

## TDD 計画

### Red: テスト先行

新規 / 更新するテスト:

1. `internal/service/find/resolver_test.go` に追記
   - `TestResolveClientByName_NotFound` → error: `no client matches name "<query>"`
   - `TestResolveClientByName_SingleHit` → ID 返す、error nil
   - `TestResolveClientByName_MultipleHits` → error + 候補列挙（最大 5 件、id+name）
   - 同等を `ResolveVendorByName` 用に 3 ケース
   - 既存の `stubClientRepo` / `stubVendorRepo` パターン (`helpers_test.go`) を再利用
2. `internal/cli/find_test.go` の reject 文言確認テストを追加
   - 各 find_*.go の構造的 reject フラグを設定して `Execute()` を呼んだ場合のエラー文言確認
   - 例: `--status` フラグ単独で `find_estimate` 実行 → エラー文言 `--status filtering is not supported for documents (no Status field on entity)` を含む
   - 注: `--client-name` / `--vendor-name` の resolver 経路は cmd.Execute() 経由では `app.AppFromContext` 失敗で
     早期エラー（resolver まで到達しない）。よって CLI 側のテストは「reject 文言」のみに限定し、
     resolver の動作は find package のテストでカバーする
3. `internal/mcpserver/tools_test.go` の reject 文言更新（MCP handler の error message が CLI と一致することを確認）

### resolver 実装の擬似コード

```go
func (s *Service) ResolveClientByName(ctx context.Context, name string, opts repository.ReadOptions) (int, error) {
    if name == "" {
        return 0, errors.New("name is empty")
    }
    list, err := s.clients.Search(ctx, boardapi.ClientListOptions{NameCont: name}, opts)
    if err != nil {
        return 0, err
    }
    switch len(list) {
    case 0:
        return 0, fmt.Errorf("no client matches name %q", name)
    case 1:
        return list[0].ID, nil
    default:
        // 候補列挙、最大 5 件
        n := len(list)
        if n > 5 { n = 5 }
        var sb strings.Builder
        fmt.Fprintf(&sb, "multiple clients match name %q (%d hits); use --id to disambiguate:\n", name, len(list))
        for i := 0; i < n; i++ {
            fmt.Fprintf(&sb, "  - id=%d name=%q\n", list[i].ID, list[i].Name)
        }
        if len(list) > 5 {
            fmt.Fprintf(&sb, "  ... (%d more)\n", len(list)-5)
        }
        return 0, errors.New(sb.String())
    }
}
```

### Green: 最小実装

各テストを通す最小実装。具体的なステップは「実装手順」で詳細。

### Refactor: 共通化と整理

- resolver の重複（CLI / MCP）が許容範囲か再評価
- error message format の整理
- helper 関数の shadowing チェック（go vet）

## 実装手順

### Step 1: find package に resolver 実装（TDD Red → Green）

1. `internal/service/find/resolver_test.go` に Red テストを追記（6 ケース、stubClientRepo/stubVendorRepo 利用）
2. `internal/service/find/resolver.go` に `ResolveClientByName` / `ResolveVendorByName` を実装
3. `go test -race ./internal/service/find/...` で green を確認

### Step 2: CLI find_*.go の reject 解除 + resolver 配線

1. `find_project.go`: `--client-name` の reject 解除、resolver 経由で `q.ClientID` 設定
2. `find_invoice.go`: 同上 + `--project-name` reject 文言更新
3. `find_purchase_order.go`: `--vendor-name` 解除、`--project-name` 文言更新
4. `find_payment.go`: `--vendor-name` 解除、`--project-name` / `--purchase-order-id` 文言更新
5. Document 4 種: `--client-name` / `--project-name` reject 解除して Query に pass、`--status` 文言更新

各ファイル変更後 `go test -race ./internal/cli/...` で green 維持。

### Step 3: MCP tools.go の reject 解除 + resolver 配線

CLI と同じ matrix で MCP handler を更新（`svc.ResolveClientByName` / `ResolveVendorByName` を呼ぶ）。
`tools_test.go` の expected error message も更新。Document 4 種は handler 内で
`getStringArg(req, "client_name")` / `getStringArg(req, "project_name")` を Query に pass する。

### Step 4: CHANGELOG.md 更新

`[Unreleased]` に追記:

- `### Changed`: enrichment non-fatal セマンティクス（breaking）
- `### Fixed` / `### Changed`: name → ID 解決の CLI/MCP 配線（N07c 完了）
- 詳細は ADR-001 / docs/api-reference へリンク

### Step 5: docs/api-reference.md 更新

`board find` 節に以下を追加:
- `--client-name` / `--vendor-name` の挙動（重複時の error）
- enrichment non-fatal（Project=nil / Client=nil / Vendor=nil の可能性）
- 構造的に未対応のフラグ一覧

### Step 6: ADR-001 更新

Implementation status に「N07c: CLI/MCP の name → ID 解決配線完了 (2026-04-27)」を追記。

### Step 7: 全テスト・vet・fmt

```bash
go test -race -count=1 ./...
go vet ./...
gofmt -s -w .
```

### Step 8: コミット分割

機能単位で論理分割:

1. `feat(cli): N07c — name → ID 解決 resolver 実装 + CLI 配線`
2. `feat(mcp): N07c — name → ID 解決 resolver 実装 + handler 配線`
3. `chore(find): N07c — 構造的未対応フラグのエラー文言を最終形に書き換え`
4. `docs: N07c — CHANGELOG / api-reference / ADR-001 更新`

heredoc で `at` 単独単語回避（`one of` 等で言い換え）。

### Step 9: ロードマップ更新

`plans/board-phase-n-roadmap.md`:
- Current Focus → N08
- Progress 行の N07c に完了マーク + コミットハッシュ
- Changelog エントリ追加

## Rollback Plan

各コミットが論理的に独立しているため、問題のあるコミットだけを `git revert` 可能:
- resolver 実装単独 revert: name 解決機能が消えるが reject 状態に戻るだけで動作は安定
- ドキュメント更新の revert: 機能には無影響

## Risk 評価

| リスク | 影響 | 緩和 |
|--------|------|------|
| NameCont 複数ヒットの error 文言が長すぎる | LLM が解析できない / 切り詰められる | 候補上限 5 件、JSON 整形は MCP 側のみ、CLI は単純な改行リスト |
| name 解決の rate limit 影響 | 1 コマンド 1 追加 API call、3 req/sec 制限 | repository cache が透過的に効くため初回のみ。実運用で問題なし |
| breaking change（enrichment non-fatal）の利用者影響 | nil チェック未対応コードが panic | CHANGELOG / api-reference 強調、ADR リンク。Phase N の v0.7.0 リリースで Breaking Changes 節として明示 |
| Document 4 種の `ClientName`/`ProjectName` 値が pass-through 経路で機能しない | 実 API テスト未実施で不発見 | unit test で Query 構築までを検証、E2E は N09 で実 API 確認 |
| MCP handler test の breaking | tools_test.go の expected が変わる | テストもセット更新（N07c コミットに含める） |
| CLI 階層化が `find_*` flat 構造に retrograde | 既に階層化済を再確認、scope 外 | 確認のみ実施、変更しない |

## 完了基準

- [ ] `internal/cli/find_resolver.go` + test 実装
- [ ] CLI 4 種 (project/invoice/purchase_order/payment) の name 解決配線
- [ ] CLI Document 4 種の `--client-name` / `--project-name` reject 解除（pass-through）
- [ ] CLI 構造的未対応フラグのエラー文言を最終形に書き換え
- [ ] `internal/mcpserver/resolver.go` + test 実装
- [ ] MCP 同等配線 + tools_test 期待値更新
- [ ] `go test -race -count=1 ./...` 全 PASS
- [ ] `go vet ./...` PASS
- [ ] `gofmt -s -w .` 差分なし
- [ ] CHANGELOG / api-reference / ADR-001 更新
- [ ] 4 コミット（feat × 2 / chore × 1 / docs × 1）で分割
- [ ] ロードマップ更新

## 後続マイルストーンへのハンドオフ

- **N08（MCP tools 刷新）**: input schema 整理、未対応フラグの schema 削除、tool 説明文の LLM 向け磨き
- **N09（E2E 再構築）**: name 解決の実 API 動作確認、Document 4 種の ClientName/ProjectName fanout 検証、Payment.Project=nil の再評価
- **N10（v0.7.0 リリース）**: enrichment non-fatal を含む Breaking Changes を CHANGELOG に集約

## 残留 TODO（N07c では対応せず）

- `find_payment --purchase-order-id` の service 拡張（FindPaymentQuery に PurchaseOrderID 追加）
- name → ID 解決の resolver を CLI/MCP で完全共通化（現状は重複コード許容）
- 重複ヒット時の候補列挙フォーマットの最終形（CLI / MCP で異なる UX 最適化）
