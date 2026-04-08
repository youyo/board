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

## 計画ファイル

- スペック: `docs/specs/board_cli_mcp_ultra_detailed_design_ja.md`
- ロードマップ: `plans/board-roadmap.md`
- マイルストーン詳細: `plans/board-m{NN}-{slug}.md`
