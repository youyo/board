# Phase K ロードマップ計画書

## Context

board CLI / MCP サーバーは既にロードマップ全 42 M（board-compliance-roadmap.md の Phase A〜J）を完走済み。release/CI/goreleaser/Homebrew tap も整備済で v0.3.0 までタグ付けされている。

しかし **コンプライアンス検証の過程で Entity 3 件の大規模フィールド不整合** が発見されており、未マイルストーン化のまま残っている:

- **ClientEntity**: M12 で発見、現行 6 フィールド中 2 つが逆方向不整合、Get で 29 フィールド未マップ（適合率 67%）
- **ProjectEntity**: M13 で発見、68 フィールド未マップ（ロードマップ最大規模）、DocumentSummary 配列化必須
- **ProjectCostEntity**: M11 で発見、概念モデル自体が根本ズレ（「カテゴリ集計」想定 → 実 API は「仕訳的 expense entry」）

これらは 271cba3（UserEntity 修正）と同種のバグで、LLM/MCP 経由で silent data loss を生むリスクがある。

加えて、補助的整備として:
- golangci-lint 未導入（CI は `go vet` のみ）
- ユーザー向けドキュメント未整備（README のみ、docs/guides/ 空）
- 旧 `plans/board-roadmap.md`（40 M 機能実装ロードマップ）がチェックボックス stale 状態で plans/ に残存

**Phase K** として 6 マイルストーンでこれらを閉じ、プロダクトの正確性とドキュメント品質を完成させる。

## 意思決定（ユーザー確認済）

| # | 決定 | 選択 |
|---|------|------|
| 1 | Entity 再設計順序 | Client → Project → ProjectCost（依存順。ClientRef 共通型を Project で再利用） |
| 2 | 後方互換 | **完全な破壊的変更**（M39-M42 方針踏襲、v0.4.0 で major 変化扱い、SQLite cache invalidate） |
| 3 | docs 優先度 | 全部まとめて（installation / getting-started / mcp-server / api-reference を 1 M で一括整備） |
| 4 | 旧 roadmap 扱い | `plans/archive/` へ移動（歴史記録として保持、新ロードマップに README リンク追加） |

## 成果物

### 新規作成ファイル
- `plans/board-phase-k-roadmap.md` — Phase K ロードマップ本体（Layer 1）
- `plans/board-phase-k-m43-client-entity.md` — M43 詳細計画（Layer 2、M43 着手時に詳細化）
- `plans/board-phase-k-m44-project-entity.md` — M44 詳細計画（遅延生成）
- `plans/board-phase-k-m45-project-cost-entity.md` — M45 詳細計画（遅延生成）
- `plans/board-phase-k-m46-lint-config.md` — M46 詳細計画（遅延生成）
- `plans/board-phase-k-m47-user-docs.md` — M47 詳細計画（遅延生成）
- `plans/board-phase-k-m48-roadmap-archive.md` — M48 詳細計画（遅延生成）

### 移動・更新ファイル
- `plans/board-roadmap.md` → `plans/archive/board-roadmap-v1.md`
- `plans/board-compliance-roadmap.md` — Current Focus を Phase K 参照に更新
- `CLAUDE.md` — 計画ファイル節に Phase K ロードマップを追記

## マイルストーン概要

### M43: ClientEntity 全面再設計（Breaking）
- **Scope**: `internal/boardapi/clients.go` を M39-M42 パターンで実 API 準拠化
- **削除フィールド**: `Code` / `Memo`（逆方向不整合）
- **追加フィールド**: 15（List/Search 共通）+ 14（Get 限定）= 計 **29 フィールド**
- **新規型**: なし（ClientRef は M39 で既存、client は親なしなので不要）
- **GetEntity 分岐**: Get で追加取得される 14 フィールドを `*string` / nullable で保持
- **Accessor**: `Code()` / `Memo()` の旧呼び出しは削除（呼び出し側を `CustomNo` / `Note` に置換）
- **Downstream 影響**: 12-15 ファイル（repository/clients.go, service/find/find_client.go, cli/api_clients.go, mcpserver, output マスク, 8-10 テスト）
- **検証**: go build/vet/test/e2e 全 Green + 実 API smoke で fill rate 確認

### M44: ProjectEntity 全面再設計（Breaking / 最大規模）
- **Scope**: `internal/boardapi/projects.go` を実 API 準拠化
- **削除フィールド**: `Status` / `Code` / `Memo` / `Delivery/Invoice/Receipt 単一ポインタ`
- **追加フィールド**: 21（List/Search 共通）+ 47（Get 限定）= 計 **68 フィールド**
- **配列化**: `Deliveries []DocumentSummary` / `Invoices []DocumentSummary` / `Receipts []DocumentSummary` へ変更（M28/M29 で部分対応済の残り）
- **nested 再利用**: `ClientRef`（M39）、`ContactRef`（新規？要調査）、`UserRef`（新規？要調査）、`ClientBranchRef`（新規？要調査）
- **新規型**: `ProjectTypeRef` / `AccountingTypeRef` / `GroupRef` / `DocumentSettingRef` 等、ネスト型の共通化
- **DocumentSummary 拡張**: `details` / `seal_approval_status` / `delivery_place` / `blank_date_flg` / `document_amount_disp_kbn` / `valid_period` 等 6-7 フィールド追加
- **Downstream 影響**: 15-18 ファイル
- **検証**: response_group（estimate/order/delivery/invoice/receipt/project_cost/all）全 6 バリアントで smoke test

### M45: ProjectCostEntity 全面再設計（Breaking）
- **Scope**: `internal/boardapi/project_costs.go` を **概念モデルから書き直し**
- **削除フィールド**: `Name` / `CostType` / `Amount` / `Memo`（全て実 API 不在）
- **追加フィールド**: `Cost` / `Description` / `InvoiceDate` / `PaymentDate`
- **概念変更**: 「労務費/資材費のカテゴリ集計」→「プロジェクト原価の個別支払い記録（仕訳 entry）」
- **Downstream 影響**: 7-8 ファイル（一番小粒）
- **検証**: M11 で発見済なので smoke test 実行で fill rate 確認

### M46: golangci-lint + .editorconfig 導入（補助）
- `.golangci.yml` 作成（推奨プリセット: errcheck / govet / staticcheck / ineffassign / unused / gofmt / goimports）
- `.editorconfig` 作成（tab/space 統一、final newline, UTF-8）
- `.github/workflows/ci.yml` に lint step 追加
- `mise.toml` に `lint` task 追加
- 既存コードの lint 違反を解消（初回スキャンで発見された warning を Entity 再設計後の最終 commit で fix）

### M47: ユーザー向けドキュメント一括整備（補助）
- `docs/installation.md` — Homebrew / バイナリ / `go install` の 3 パターン
- `docs/guides/getting-started.md` — `configure` → `api clients list` → `find client` → `mcp serve` の 5 分クイックスタート
- `docs/guides/mcp-server.md` — MCP クライアント設定例（Claude Desktop / Claude Code）、利用可能ツール一覧、トラブルシューティング
- `docs/api-reference.md` — `api` / `find` コマンドの全 subcommand リファレンス（リソース × List/Get/Search マトリクス）
- README.md に docs/ への動線追加

### M48: 旧ロードマップ archive 化（補助・最小）
- `mkdir plans/archive/`
- `git mv plans/board-roadmap.md plans/archive/board-roadmap-v1.md`
- `plans/archive/board-roadmap-v1.md` 先頭に「archived 日付 + 理由」を追記
- `CLAUDE.md` / `plans/board-phase-k-roadmap.md` / `plans/board-compliance-roadmap.md` から参照を更新

## 依存関係

```
M43 (Client) ──┐
               ├→ M44 (Project, ClientRef 再利用)
M45 (ProjectCost, 独立) ──┘
                          ├→ M46 (lint) ←─ Entity 再設計後が理想（lint 違反を一括で直す）
                          ├→ M47 (docs) ←─ Entity スキーマ確定後が正確性担保
                          └→ M48 (archive, 独立)
```

推奨実行順: **M43 → M44 → M45 → M46 → M47 → M48**

## リリース計画（参考）

- M43 完了時: v0.4.0-rc1（Breaking: ClientEntity schema 変更）
- M44 完了時: v0.4.0-rc2（Breaking: ProjectEntity schema 変更）
- M45 完了時: v0.4.0-rc3
- M46-M48 完了時: **v0.4.0 正式リリース**（Breaking changes を CHANGELOG にまとめて notice）

## リスクと対策

| # | リスク | 影響度 | 対策 |
|---|--------|--------|------|
| 1 | ProjectEntity 68 フィールド追加で nested 型の掘り下げが深く、工数超過 | 高 | M44 着手前に詳細計画で nested 型リストを確定、独立型をどこまで切るか判断 |
| 2 | Breaking change で既存ユーザー（設定済 cache ユーザー）が壊れる | 中 | M48 で CHANGELOG に cache invalidate 手順を記載、`board cache clear` を推奨 |
| 3 | M44 で発見される未マップフィールドが更に増える可能性 | 中 | 実 API smoke 優先で確定、仕様差分は detailed plan で明示 |
| 4 | docs 一括整備でボリューム過大 | 中 | M47 着手時に各 doc の骨子確定後、優先度付けで段階投入可能とする |
| 5 | lint 導入で既存コードの違反が大量発生 | 低 | M46 で段階的に opt-in（errcheck 先行、staticcheck 後発等） |

## 検証計画

各 M 完了時の受け入れ基準:
- [ ] `go build ./...` PASS
- [ ] `go vet ./...` PASS
- [ ] `go test -count=1 ./...` 全 PASS
- [ ] （Entity 再設計 M のみ）`go test -tags e2e -v -count=1 -run TestE2E_{Resource}_(List|Get|Search) ./internal/boardapi/` 全 PASS
- [ ] `./board --version` / `./board api {resource} list --pretty` 手動動作確認
- [ ] 該当 plan の Implementation Steps チェック完了

全 M 完了時:
- [ ] `git log --oneline` で Phase K の commit 一覧確認
- [ ] v0.4.0 タグ付け前に `CHANGELOG.md` に Breaking changes を記載
- [ ] `plans/archive/` 配下に旧 roadmap が移動済
- [ ] README.md の docs/ リンク有効

## 既存コードの再利用

- **M39-M42 で確立したパターン**:
  - `ClientRef` / `VendorRef` 共通型（M44 で ClientRef 再利用）
  - `*string` nullable フィールド方式
  - `VendorID()` / `ClientID()` accessor による後方互換
  - StrictFieldDiff helper（`internal/testhelper/`）
  - `dumpJSON` helper（e2e 用）
- **既存インフラ**:
  - goreleaser（`.goreleaser.yaml`） — そのまま使用
  - CI workflow（`.github/workflows/ci.yml`） — M46 で lint step 追加
  - README / README_ja — 動線追加のみ

## スケジュール見込（推定）

| M | 予想工数 | 理由 |
|---|---------|------|
| M43 | 中（M39 の 2-3 倍） | 29 フィールド追加、12-15 ファイル修正 |
| M44 | 大（M43 の 2 倍） | 68 フィールド + 配列化 + nested 型多数 |
| M45 | 小（M39 並） | 7-8 ファイル、概念シフトのドキュメント化が負荷 |
| M46 | 小 | 設定ファイル追加 + 既存 warning fix |
| M47 | 中 | 4 doc 一括、コンテンツ量多い |
| M48 | 最小 | ファイル移動 + リンク更新のみ |

## 次ステップ

本計画ファイル（`plans/tender-squishing-dusk.md`）の承認後:

1. `plans/board-phase-k-roadmap.md` を Layer 1 として生成
2. `plans/board-phase-k-m43-client-entity.md` を Layer 2 の詳細計画として即座に生成（M43 着手準備）
3. M44-M48 は遅延生成（各 M 着手時に `/devflow:plan` で起票）
4. `plans/board-compliance-roadmap.md` の Current Focus を「Phase K 進行中」に更新
5. `CLAUDE.md` の計画ファイル節に Phase K を追記

実装開始は `/devflow:implement` または M43 個別実装で。
