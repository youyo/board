# Phase K ロードマップ: Entity 準拠 + プロダクト完成

## Meta
| 項目 | 値 |
|------|---|
| ゴール | 残 Entity 3 件（Client/Project/ProjectCost）の実 API 準拠化と、プロダクト仕上げ（lint/docs/archive）で v0.4.0 リリース可能状態に到達する |
| 成功基準 | 全 6 M 完了 + `go test -tags e2e` 全 Green + `./board api clients get <id>` / `./board api projects get <id>` が実 API と full-fidelity で動作 |
| 制約 | 破壊的変更（SQLite cache invalidate 必要）、Rate Limit 3 req/sec, 3000/day 遵守 |
| 対象リポジトリ | /Users/youyo/src/github.com/youyo/board |
| 作成日 | 2026-04-22 |
| 最終更新 | 2026-04-22 |
| ステータス | 完了（M43-M48 全完走）✅ |
| 親計画 | plans/tender-squishing-dusk.md |
| 併走計画 | plans/board-compliance-roadmap.md（42 M 完走済） |

## 背景

- Phase A〜J（42 M）で準拠検証は完走したが、Phase E/D で発見された **Entity 3 件の大規模フィールド不整合** が未解決
- Release/CI/goreleaser/Homebrew は既に整備済（v0.3.0 公開中）
- 補助的に lint/docs/archive を閉じて v0.4.0 リリースへ

## Current Focus
- **ステータス**: Phase K 全マイルストーン完走 ✅
- **直近の完了**: M48 旧ロードマップ archive 化（2026-04-22）
- **次のアクション**: v0.4.0 リリース準備（ユーザー手動でタグ付け。CHANGELOG に Breaking changes と `board cache clear` 必須案内を記載）

## Progress

### M43: ClientEntity 全面再設計（Breaking）✅ 完了
- [x] `internal/boardapi/clients.go` の 6 フィールド → 実 API 準拠 33 フィールド（削除 2 + 既存 4 + 追加 29）に書き換え
- [x] `Code` / `Memo` フィールドを削除（逆方向不整合を解消）
- [x] 15（List/Search 共通）+ 14（Get 限定）= 29 フィールドを追加
- [x] `*string` nullable 方針で Get 限定 14 フィールドを保持
- [x] downstream 修正（service/find: derefString ヘルパー追加 + find_client.go + テスト更新）
- [x] `go build / vet / test` 全 Green
- [x] 実 API smoke test PASS（List 299 件 / Get 1 件 / Search 299 件、未マップ 0）
- 📄 詳細: plans/board-phase-k-m43-client-entity.md ✅

### M44: ProjectEntity 全面再設計（Breaking / 最大規模）✅ 完了
- [x] `internal/boardapi/projects.go` の 13 フィールド → 72 フィールドに書き換え
- [x] `Status` / `Code` / `Memo` / `Delivery/Invoice/Receipt 単一ポインタ` を削除
- [x] 21 + 47 = 68 フィールドを追加
- [x] `Deliveries/Invoices/Receipts` を `[]DocumentSummary` 配列化
- [x] nested 型の確定（ClientRef 再利用、ContactRef/UserRef/ClientBranchRef/HubspotRef 新規）
- [x] DocumentSummary に 17 フィールド追加（共通 5 + type-specific 12）
- [x] downstream 9 ファイル修正
- [x] response_group 全 6 バリアントで smoke test PASS（未マップ 0）
- 📄 詳細: plans/board-phase-k-m44-project-entity.md ✅

### M45: ProjectCostEntity 全面再設計（Breaking）✅ 完了
- [x] `internal/boardapi/project_costs.go` を概念モデルから書き直し
- [x] `Name` / `CostType` / `Amount` / `Memo` を削除
- [x] `Description` / `Cost` / `InvoiceDate *string` / `PaymentDate *string` を追加
- [x] `ProjectEntity.ProjectCosts` を `json.RawMessage` から `[]ProjectCostEntity` へ変換
- [x] downstream 4 ファイル修正（client_test.go, project_costs_test.go, e2e_project_costs_test.go, repository/project_costs_test.go）
- [x] smoke test PASS（List 22 件 / Get 1 件 / Search 22 件、未マップ 0）
- [x] M44 TestE2E_Projects_GetWithGroup 全 6 variants 引き続き PASS
- 📄 詳細: plans/board-phase-k-m45-project-cost-entity.md ✅

### M46: golangci-lint + .editorconfig 導入 ✅ 完了
- [x] `.golangci.yml` 作成（errcheck/govet/staticcheck/ineffassign/unused/gofmt/goimports）
- [x] `.editorconfig` 作成
- [x] `.github/workflows/ci.yml` に lint step 追加
- [x] `mise.toml` に `lint` task 追加
- [x] 既存 warning を一括解消（gofmt: 3件、errcheck: 8件、staticcheck: 4件 → 0件）
- 📄 詳細: plans/board-phase-k-m46-lint-config.md ✅

### M47: ユーザー向けドキュメント一括整備 ✅ 完了
- [x] `docs/installation.md`（Homebrew / バイナリ / go install）
- [x] `docs/guides/getting-started.md`（5 分クイックスタート）
- [x] `docs/guides/mcp-server.md`（Claude Desktop / Claude Code 連携）
- [x] `docs/api-reference.md`（api/find コマンドリファレンス）
- [x] README.md / README_ja.md に docs/ 動線追加
- 📄 詳細: plans/board-phase-k-m47-user-docs.md ✅

### M48: 旧ロードマップ archive 化 ✅ 完了
- [x] `plans/archive/` ディレクトリ作成
- [x] `plans/board-roadmap.md` → `plans/archive/board-roadmap.md` 移動（併せて旧 `board-m01`〜`board-m36` 系 24 件も archive に移設）
- [x] 先頭に archived 日付と理由を追記
- [x] CLAUDE.md / phase-k-roadmap の参照を更新
- 📄 詳細: plans/board-phase-k-m48-roadmap-archive.md ✅

## 依存関係

```
M43 (Client) ──┐
               ├→ M44 (Project, ClientRef 再利用)
M45 (ProjectCost, 独立) ──┘
                          ├→ M46 (lint, Entity 後)
                          ├→ M47 (docs, Entity 後)
                          └→ M48 (archive, 独立)
```

実行順: **M43 → M44 → M45 → M46 → M47 → M48**

## リリース計画

- M43 完了: v0.4.0-rc1（Breaking: ClientEntity schema 変更）
- M44 完了: v0.4.0-rc2（Breaking: ProjectEntity schema 変更）
- M45 完了: v0.4.0-rc3
- M46-M48 完了: **v0.4.0 正式リリース** + CHANGELOG で breaking changes 案内

## Blockers
なし

## Architecture Decisions
| # | 決定 | 理由 | 日付 |
|---|------|------|------|
| 1 | 完全破壊的変更（後方互換レイヤー不採用） | M39-M42 方針踏襲、コードが複雑化する移行コード不要、v0.4.0 で major 変化として案内 | 2026-04-22 |
| 2 | Client → Project → ProjectCost 順 | ClientRef 共通型が Project で再利用可、依存順に自然 | 2026-04-22 |
| 3 | docs は 4 ファイル一括整備 | 部分整備だと導線が分断、一気通貫の UX 提供 | 2026-04-22 |
| 4 | 旧 roadmap は archive（削除しない） | 歴史的記録として価値、40 M 完走の軌跡を残す | 2026-04-22 |

## Changelog
| 日時 | 種別 | 内容 |
|------|------|------|
| 2026-04-22 17:10 | 作成 | Phase K ロードマップ初版。親計画 plans/tender-squishing-dusk.md に基づき 6 M 構成で起票 |
| 2026-04-22 18:03 | M43 完了 | ClientEntity を 6 フィールドから 33 フィールドへ全面再設計。Code/Memo を削除し CustomNo/Note に置換。Unit test 6 件追加（U1-U6）。downstream 修正 4 ファイル（find_client.go, text_match.go, find_client_test.go, e2e_clients_test.go）。E2E smoke: List 299 件 / Get / Search 全 PASS 未マップ 0。commit: 3b87f78, 5ee83fa, 71d0b4f |
| 2026-04-22 18:45 | M44 完了 | ProjectEntity を 13 フィールドから 72 フィールドへ全面再設計（最大規模）。Status/Code/Memo/Delivery・Invoice・Receipt 単数ポインタを削除。nested 型 5 件新規追加（UserRef/ContactRef/ClientBranchRef/CompanyBranchRef/HubspotRef）。DocumentSummary を 17 フィールド拡張。Deliveries/Invoices/Receipts を []DocumentSummary 配列化。Unit test 12 件追加（U1-U12）。downstream 修正 9 ファイル（projects.go, find_project.go, text_match.go, find_estimate/order/delivery/receipt/invoice.go, helpers_test.go 等）。E2E smoke: List/Get/Search/GetWithGroup 全 6 variants PASS 未マップ 0。commit: 88139f9, b20c138, 0d1f652, f1c2eee, 2c5e254 |
| 2026-04-22 19:10 | M45 完了 | ProjectCostEntity を 8 フィールド（概念モデルから全面再設計）。Name/CostType/Amount/Memo の 4 フィールドを削除し Description/Cost/InvoiceDate*/PaymentDate* を追加。ProjectEntity.ProjectCosts を json.RawMessage から []ProjectCostEntity へ変換。Unit test 3 件追加（U1-U3）。downstream 修正 4 ファイル（boardapi/client_test.go, project_costs_test.go, e2e_project_costs_test.go, repository/project_costs_test.go）。E2E smoke: List 22 件 / Get 1 件 / Search 22 件 PASS 未マップ 0。M44 GetWithGroup 全 6 variants 引き続き PASS。commit: 8ce558f, 38f656c |
| 2026-04-22 19:30 | M46 完了 | golangci-lint v2 + .editorconfig 導入。.golangci.yml（errcheck/govet/staticcheck/ineffassign/unused/gofmt/goimports）を作成。既存コード違反 15 件（gofmt: 3、errcheck: 8、staticcheck: 4）を解消。GitHub Actions CI に golangci-lint-action@v6 ステップ追加。mise run lint タスク追加。go build/vet/test 全 PASS。commit: 6708a80, 25093ee, c106001, 2e16f3a, 3f97de8, 5b58570 |
