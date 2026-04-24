# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.6.0] - 2026-04-24

Phase M: CLI / ドキュメント / LLM 連携の拡充（v0.6.0 minor bump）

### New Features

- **shell completion の値補完対応（M58）**: `--response-group` / `--order-status-in` / `--delivery-status-in` / `--invoice-timing-kbn-in` の固定列挙値を zsh / bash / fish で TAB 補完可能。zsh では日本語説明付きで表示（例: `1	見積中(高)`）
- **`board docs` サブコマンド追加（M59）**: バイナリに埋め込まれた README / api-reference / installation / guides を CLI から参照可能
  - `board docs` — README 表示
  - `board docs --list` — 埋め込みドキュメント一覧
  - `board docs <resource>` — api-reference.md からリソース別セクションを抽出
  - `board docs --search <keyword>` — 全文検索（大文字小文字無視、±2 行コンテキスト、連続ヒットはマージ）
  - `--format json` で JSON 出力（`{mode, query, results[]}` スキーマ、LLM / MCP 向け機械可読）
- **`/board:docs` Claude Code スキル追加（M60）**: `.claude-plugin/plugin.json` + `skills/docs/SKILL.md` により、Claude Code プラグインとして登録すれば AI エージェントから BOARD CLI の使い方をオフライン参照可能。情報の一次ソースは `board docs` バイナリ埋め込みドキュメントに一元化（薄いラッパー設計）
- **`docs/api-reference.md` 拡充（M61）**: サンプル JSON（list / get / invoices）、エラー応答例（401 / 404 / 429 / CLI 自体のエラー）、Ransack フィルタ完全表（オペレータ俯瞰）、CLI 補完値一覧、`board docs` サブコマンド仕様（JSON スキーマ）を追加

### Internal

- `internal/docs/` パッケージ新設、`go:embed` で `docs/` 配下（`specs/` 除外）+ `README.md` をバイナリに取り込み
- `mise run sync-docs` / `mise run check-docs-sync` タスク追加。前者は `docs/` + `README.md` を `internal/docs/assets/` に同期、後者は drift を検知して fail
- GitHub Actions `ci.yml` に docs sync drift 検知ステップを組み込み（`rsync` + `diff -r`、依存ツール最小化）
- `.claude-plugin/plugin.json` version を `0.5.0` → `0.6.0` に bump

### Notes

- **バイナリサイズ**: 22.07 MB → 22.21 MB（+138 KB、+0.64%）。埋め込みドキュメント 52 KB + `go:embed` FS メタデータ + `encoding/json` / `io/fs` / `regexp` のリンク分が支配的。当初の +50 KB 目標は `go:embed` リンカ挙動の過小見積もりだったため事後受容
- **Breaking Changes なし**: 既存 API / コマンドの挙動変更はなし、純粋な機能追加のみ
- **キャッシュ再構築不要**: v0.5.0 からのアップグレードでキャッシュのスキーマ変更はありません

## [0.5.0] - 2026-04-24

### Breaking Changes

Phase L: api 層 BOARD API 完全準拠化（v0.5.0 major bump）

- **全 22 リソースの SearchParams 型を廃止**: `ClientSearchParams` → `ClientListOptions` 等に変更
- **全 22 リソースの `Search*` / `ListXPage` コマンドを廃止**: `list` コマンドに統合
- **`ListResult[T]{Items, Meta, Headers}` 型を導入**: 全 List/Search 系の戻り値が変更
- **Ransack 風クエリパラメータに移行**: `name` → `name_cont`、`updated_at_from` → `updated_at_gteq` 等
- **`PageResult[T]` / `ListPage` / `ListAll` を削除**: `ListAllWithResult` に統一
- **キャッシュ再構築が必要**: `board cache clear` を実行してください

### New Features

- 全 22 リソースに Ransack 風フィルタ追加（`--name-cont`、`--updated-at-gteq`、`--response-group` 等）
- レスポンスヘッダー（X-Total-Count / Rate Limit / ETag 等）を `_meta` フィールドで取得可能
- `--show-meta` フラグで `_meta` フィールドを JSON 出力に追加
- `projects` に 15+ の新フィルタ（`--order-status-in`、`--delivery-date-gteq` 等）
- `clients` に `response_group` フィルタ追加（`--response-group small|large`）

### Internal

- `ListAllWithResult` に全面移行（`ListAll` / `ListPage` / `PageResult` 撤去）
- `QueryBuilder` ヘルパにより Ransack 風パラメータ組み立てを共通化
- `parseListMeta` / `parseItemMeta` により X-Total-Count / Rate Limit / ETag 等のヘッダーを構造体として伝達

## [0.4.1] - 2026-04-22

### Fixed

- `board api` コマンドの `--limit` デフォルト値を 50 から 0（無制限）に変更
- `ProjectRepository.Search` の `ResponseGroup` 分岐に `applyLimit` を適用

## [0.4.0] - 2026-04-09

### Changed

- Phase K: Entity 全面再設計（`*Ref` 共通型 / `*string` nullable / `derefString` パターン確立）
- 全 22 リソースの Entity を BOARD API フィールド完全準拠に刷新
