# Phase K ロードマップ: Entity 準拠 + プロダクト完成

## Meta
| 項目 | 値 |
|------|---|
| ゴール | 残 Entity 3 件（Client/Project/ProjectCost）の実 API 準拠化と、プロダクト仕上げ（lint/docs/archive）で v0.4.0 リリース可能状態に到達する |
| 成功基準 | 全 6 M 完了 + `go test -tags e2e` 全 Green + `./board api clients get <id>` / `./board api projects get <id>` が実 API と full-fidelity で動作 |
| 制約 | 破壊的変更（SQLite cache invalidate 必要）、Rate Limit 3 req/sec, 3000/day 遵守 |
| 対象リポジトリ | /Users/youyo/src/github.com/youyo/board |
| 作成日 | 2026-04-22 |
| 最終更新 | 2026-04-22 17:10 |
| ステータス | 未着手 |
| 親計画 | plans/tender-squishing-dusk.md |
| 併走計画 | plans/board-compliance-roadmap.md（42 M 完走済） |

## 背景

- Phase A〜J（42 M）で準拠検証は完走したが、Phase E/D で発見された **Entity 3 件の大規模フィールド不整合** が未解決
- Release/CI/goreleaser/Homebrew は既に整備済（v0.3.0 公開中）
- 補助的に lint/docs/archive を閉じて v0.4.0 リリースへ

## Current Focus
- **マイルストーン**: M43 ClientEntity 全面再設計
- **直近の完了**: Phase K ロードマップ起票（本ファイル）
- **次のアクション**: M43 詳細計画 `plans/board-phase-k-m43-client-entity.md` に従い実装開始

## Progress

### M43: ClientEntity 全面再設計（Breaking）
- [ ] `internal/boardapi/clients.go` の 6 フィールド → 実 API 準拠 33 フィールド（削除 2 + 既存 4 + 追加 29 - 削除分）に書き換え
- [ ] `Code` / `Memo` フィールドを削除（逆方向不整合）
- [ ] 15（List/Search 共通）+ 14（Get 限定）= 29 フィールドを追加
- [ ] `*string` nullable 方針で Get 限定 14 フィールドを保持
- [ ] downstream 12-15 ファイル修正（repository / service/find / cli / output / test）
- [ ] `go build / vet / test` 全 Green
- [ ] 実 API smoke test PASS（List/Get/Search）
- 📄 詳細: plans/board-phase-k-m43-client-entity.md ✅

### M44: ProjectEntity 全面再設計（Breaking / 最大規模）
- [ ] `internal/boardapi/projects.go` の 13 フィールド → 81 フィールドに書き換え
- [ ] `Status` / `Code` / `Memo` / `Delivery/Invoice/Receipt 単一ポインタ` を削除
- [ ] 21 + 47 = 68 フィールドを追加
- [ ] `Deliveries/Invoices/Receipts` を `[]DocumentSummary` 配列化
- [ ] nested 型の確定（ClientRef 再利用、ContactRef/UserRef/ClientBranchRef/ProjectTypeRef/AccountingTypeRef/GroupRef 等の新規型）
- [ ] DocumentSummary に 6-7 フィールド追加
- [ ] downstream 15-18 ファイル修正
- [ ] response_group 全 6 バリアントで smoke test
- 📄 詳細: plans/board-phase-k-m44-project-entity.md（着手時生成）

### M45: ProjectCostEntity 全面再設計（Breaking）
- [ ] `internal/boardapi/project_costs.go` を概念モデルから書き直し
- [ ] `Name` / `CostType` / `Amount` / `Memo` を削除
- [ ] `Cost` / `Description` / `InvoiceDate` / `PaymentDate` を追加
- [ ] downstream 7-8 ファイル修正
- [ ] smoke test PASS
- 📄 詳細: plans/board-phase-k-m45-project-cost-entity.md（着手時生成）

### M46: golangci-lint + .editorconfig 導入
- [ ] `.golangci.yml` 作成（errcheck/govet/staticcheck/ineffassign/unused/gofmt/goimports）
- [ ] `.editorconfig` 作成
- [ ] `.github/workflows/ci.yml` に lint step 追加
- [ ] `mise.toml` に `lint` task 追加
- [ ] 既存 warning を一括解消
- 📄 詳細: plans/board-phase-k-m46-lint-config.md（着手時生成）

### M47: ユーザー向けドキュメント一括整備
- [ ] `docs/installation.md`（Homebrew / バイナリ / go install）
- [ ] `docs/guides/getting-started.md`（5 分クイックスタート）
- [ ] `docs/guides/mcp-server.md`（Claude Desktop / Claude Code 連携）
- [ ] `docs/api-reference.md`（api/find コマンドリファレンス）
- [ ] README.md / README_ja.md に docs/ 動線追加
- 📄 詳細: plans/board-phase-k-m47-user-docs.md（着手時生成）

### M48: 旧ロードマップ archive 化
- [ ] `plans/archive/` ディレクトリ作成
- [ ] `plans/board-roadmap.md` → `plans/archive/board-roadmap-v1.md` 移動
- [ ] 先頭に archived 日付と理由を追記
- [ ] CLAUDE.md / phase-k-roadmap / compliance-roadmap の参照を更新
- 📄 詳細: plans/board-phase-k-m48-roadmap-archive.md（着手時生成）

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
