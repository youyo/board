# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## プロジェクト概要

BOARD API（https://api.the-board.jp/v1/）向けの Go 製 CLI + ローカル HTTP MCP サーバー。単一バイナリ `board` として提供。

- **CLI**: `board api ...`（low-level、API準拠）+ `board find ...`（high-level、LLM向け）
- **MCP**: `board mcp serve` でローカル HTTP MCP サーバー起動
- デフォルト出力は JSON（`--pretty` で整形表示）
- SQLite キャッシュで BOARD API rate limit（3000/日、3/秒）に対応

## 技術スタック

- Go 1.26（mise で管理: `mise use go@1.26`）
- CLI: spf13/cobra
- TOML: pelletier/go-toml/v2
- SQLite: modernc.org/sqlite（CGO不要）
- MCP: mark3labs/mcp-go
- ターゲット: macOS + Linux (amd64/arm64)

## ビルド・テスト

```bash
mise run build    # ./board バイナリ生成
mise run test     # go test ./...
mise run vet      # go vet ./...
mise run fmt      # gofmt -s -w .
```

## アーキテクチャ

```
CLI / MCP
  → service（api: low-level / find: high-level）
  → repository（cache参照 + refresh判定 + API fallback）
  → refresh（daily判定 / delta / force）+ cache（SQLite）
  → boardapi（HTTP client）
```

- **cli**: Cobra コマンド定義のみ。業務ロジック禁止
- **app**: DI コンテナ。config/DB/client/repository/service の組み立て
- **config**: config.toml の型定義・load/save。profile 管理
- **boardapi**: BOARD API 生クライアント。認証（x-api-key + Bearer）、retry、pagination
- **cache**: SQLite。resource_cache（entity単位JSON blob）+ sync_state + cache_meta
- **refresh**: daily auto refresh 判定、差分/全件取得、ロック制御
- **repository**: cache→refresh→API の統一参照窓口。resource ごとに1ファイル
- **service/api**: low-level ユースケース（API準拠 list/get/search）
- **service/find**: high-level ユースケース（複数resource横断検索）
- **mcpserver**: ローカル HTTP MCP。high-level service のみ公開
- **output**: JSON/pretty/mask

## BOARD API リソース（全22）

コア: clients, client_branches, contacts, projects, project_costs
ドキュメント: estimates, invoices, orders, deliveries, receipts
ベンダー: vendors, vendor_branches, vendor_contacts, purchase_orders, payments
マスタ: users, groups, payment_terms, project_types, purchase_types, accounting_types, document_send_channels

## 設計原則

- キャッシュ責務は low-level（repository）側に集約。high-level は独自キャッシュ禁止
- refresh は内部処理。独立 sync コマンドは提供しない
- daily auto refresh はデフォルト ON（config で OFF 可）
- CLI command に業務ロジックを書かない
- secrets をログ・エラー・pretty 出力に出さない

## テスト戦略

### BOARD API 準拠検証ロードマップ
- **目的**: 全 22 リソース × (List/Get/Search) × (boardapi/find 層) の E2E で、実 API との厳格フィールド突合
- **実施**: board-compliance roadmap（M01-M38）により 38 マイルストーン完了
- **検証手法**: StrictFieldDiff helper で生 JSON と Go Entity の全フィールド突合
- **Rate Limit 対応**: per-batch テスト（M 単位）で段階的実施。単発 `go test -tags e2e ./...` は 3 req/sec 制約で実施不可
- **詳細**: `plans/board-compliance-roadmap.md` および各 M の plan ファイル、仕様書§39 参照

### テスト実行コマンド
```bash
# ユニットテスト
go test -count=1 ./...

# E2E テスト（per-batch、M 単位で実行）
go test -tags e2e -v -count=1 -run TestE2E_XXX ./internal/boardapi/
go test -tags e2e -v -count=1 -run TestE2E_XXX ./internal/service/find/

# 型チェック・フォーマット
go vet ./...
gofmt -s -w .
```

## 計画ファイル

- スペック: `docs/specs/board_cli_mcp_ultra_detailed_design_ja.md`
- ロードマップ（機能実装、実装完走済）: `plans/board-roadmap.md`（※M48 で `plans/archive/` へ移設予定）
- ロードマップ（準拠検証、42 M 完走済）: `plans/board-compliance-roadmap.md`
- **ロードマップ（Phase K、進行中）**: `plans/board-phase-k-roadmap.md` — Entity 3 件の全面再設計 + 仕上げ（v0.4.0 向け）
- マイルストーン詳細: `plans/board-m{NN}-{slug}.md` / `plans/board-compliance-m{NN}-{slug}.md` / `plans/board-phase-k-m{NN}-{slug}.md`
