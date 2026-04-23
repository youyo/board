# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
