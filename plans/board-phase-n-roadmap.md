# Phase N ロードマップ: find 層必要性評価 → v0.7.0

## Meta
| 項目 | 値 |
|------|---|
| ゴール | `internal/service/find/`（12 Find メソッド、47 E2E テスト）の存在意義を api 層の成熟度を踏まえて再評価し、**廃止 / 再設計 / 一部残す** のいずれかを ADR で意思決定。決定内容に従って実装を刷新し v0.7.0 をリリース |
| 成功基準 | `docs/adr/ADR-001-find-layer.md` で意思決定記録 + 決定内容に基づく実装完了 + MCP 接続部も新方針に沿って刷新 + E2E テスト全 Green（実 API で感応するケースのみ） + v0.7.0 タグ配信 |
| 制約 | Phase M 完了（v0.6.0）後に着手、調査中は既存 find 層は現状維持（機能欠落期間を作らない）、実装方針は N01 の意思決定結果に依存 |
| 対象リポジトリ | /Users/youyo/src/github.com/youyo/board |
| 作成日 | 2026-04-24 |
| 最終更新 | 2026-04-24 |
| ステータス | 未着手（Phase M 完了待ち） |
| 親計画 | plans/groovy-churning-valley.md（plan-mode 集約プラン） |
| 先行フェーズ | plans/board-phase-m-roadmap.md（Phase M: CLI/Docs 充実化、v0.6.0） |

## 背景

Phase H（M25-M32、2026-04-21 完走）で `internal/service/find/` 層の 12 Find メソッド + 47 E2E テストまで作った。
しかし api 層が Phase L（M49-M57）で Ransack 形式に準拠し、`--name-cont` / `--client-name-cont` / `--*-eq` / `--*-in[]`
などを直接使えるようになった今、find 層の存在意義は以下の観点で再評価が必要。

1. **TODO(M25-M32) の未解消**
   - `find_estimate.go:19,29`, `find_order.go:19,29`, `find_delivery.go:19,32`, `find_receipt.go:19,32` に
     4 箇所の TODO(M25-M32)「enrichment 復元 / Status post-filter 復元」
2. **E2E テストの SKIP が多い**
   - 47 テスト中 15+ が SKIP（vendor 0 件 / group 0 件 / cache-warm 必須 / 11,748 件 invoices タイムアウト）
3. **api 層が成熟した今、find の付加価値が薄い可能性**
   - `FindProject(ClientName=xxx)` は Phase L 以降 `api projects list --client-name-cont xxx` で完全代替できる可能性
   - 「柔軟検索」「統合ビュー」といった元々の目的は api 層の Ransack + `--response-group` で多くをカバー可能

ユーザー判断: **「api 層を実装してみた結果、そもそも find 層が必要かから問い直したい」**。
全廃棄 + ゼロベース再構築を前提に、N01 で必要性を評価し、意思決定する。

## Current Focus
- **マイルストーン**: N01（find 層必要性評価 + ADR）
- **直近の完了**: なし（Phase N 起票時点）
- **次のアクション**: Phase M 完了後に N01 着手

## Progress

### N01: find 層必要性評価 + 設計方針案 + ADR
- [ ] `plans/board-phase-n-m01-find-rationale.md` 雛形作成
- [ ] 観点 1-5 のデータ収集（grep / Read / 既存ドキュメント精読）
  - 観点 1: find 層の付加価値棚卸し
  - 観点 2: api 層 Ransack での代替可能性
  - 観点 3: BOARD 本体サイト機能とのギャップ
  - 観点 4: 既存利用実績（git blame / 実 MCP コール）
  - 観点 5: 現状の技術的負債（TODO / SKIP / timeout）
- [ ] 「12 Find メソッド × api 層代替可否」の大表作成
- [ ] 選択肢 A/B/C の trade-off 分析
  - A: 全廃止（MCP を api 層直接呼び出しに）
  - B: ゼロベース再設計
  - C: 数本に絞る（api 層で代替できないものだけ残す）
- [ ] ユーザーレビュー → 意思決定
- [ ] `docs/adr/ADR-001-find-layer.md` に最終決定記録
- [ ] `docs/specs/board_cli_mcp_ultra_detailed_design_ja.md` の関連節に補記
- 📄 詳細: Phase N 着手時に `plans/board-phase-n-m01-find-rationale.md` を `/devflow:plan` で詳細化（遅延生成）
- 📝 groovy-churning-valley.md の N01 詳細セクションを参照

### N02+（N01 意思決定結果による分岐、概要のみ）

#### 選択肢 A（全廃止）になった場合
- N02: `internal/service/find/` 削除
- N03: `internal/cli/find_*.go` 削除
- N04: MCP tools（`internal/mcpserver/tools.go`）を api 層直接呼び出しに書き換え
- N05: v0.7.0 リリース（Breaking Change として CHANGELOG 強調）

#### 選択肢 B（ゼロベース再設計）になった場合
- N02: 新仕様策定（Query / Result 型、FindXxx API 設計）
- N03-N07: リソース別に実装（client / project / document / vendor / master）
- N08: MCP tools 刷新
- N09: E2E テスト再構築（実 API 感応のみ、SKIP 理由は明示ログ）
- N10: v0.7.0 リリース

#### 選択肢 C（数本に絞る）になった場合
- N02: 残すメソッドを確定（例: ClientName → Project 逆引きなど api 層で無理な機能）
- N03-N04: 残すメソッドの再実装（enrichment / post-filter 含む）
- N05: 不要メソッド削除
- N06: MCP tools 整理
- N07: v0.7.0 リリース

## Blockers

- **Phase M 未完了**: Phase N は v0.6.0 リリース後に開始する前提
- **N01 の意思決定未確定**: N02+ の具体的なマイルストーンは N01 結果で分岐

## Architecture Decisions

| # | 決定 | 理由 | 日付 |
|---|------|------|------|
| 1 | find 層は N01 の調査結果で意思決定 | api 層実装後に改めて必要性を見直したい（ユーザー判断） | 2026-04-24 |
| 2 | N01 調査中は既存 find 層を現状維持 | 機能欠落期間を作らない、v0.6.0 リリース後も既存 CLI/MCP を変えず使用可能 | 2026-04-24 |
| 3 | 成果物は「調査レポート + 設計方針案 + ADR」の 3 点セット | トレーサビリティ確保、将来の判断再考時に再利用可能 | 2026-04-24 |
| 4 | E2E テストは「実 API で感応するテストを網羅」方針 | vendor/group が 0 件の環境では SKIP を残しつつ、実データがあるケースは全て実行 | 2026-04-24 |
| 5 | MCP 接続部も再設計対象 | LLM が使いやすい引数名 / 説明に磨く余地がある | 2026-04-24 |

## Scope / Non-Goals

- **In scope**
  - find 層の存在意義評価 + 意思決定（ADR）
  - 決定内容に基づく `internal/service/find/` / `internal/cli/find_*.go` / MCP tools の刷新
  - E2E テストの再構築（SKIP 理由の明示ログ化を含む）

- **Out of scope**
  - 動的補完の実装（Phase M と同じく Phase N でも対象外）
  - 新たな API エンドポイント追加
  - cache / refresh 層のリファクタ

## Changelog

| 日時 | 種別 | 内容 |
|------|------|------|
| 2026-04-24 | 作成 | Phase N ロードマップ初版作成（N01 詳細 + N02+ 概要、v0.7.0 ターゲット） |

## Next Action

1. Phase M（v0.6.0）完走を待つ
2. Phase M 完了後、`plans/board-phase-n-m01-find-rationale.md` を `/devflow:plan` で詳細化
3. N01 実施 → ユーザーレビュー → ADR 確定
4. ADR に従い N02+ を逐次実装
