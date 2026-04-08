# README 作成プラン

## Context

プロジェクトに README がない。英語版 README.md を作成し、日本語版 README_ja.md も作成してリンクする。

## 作成ファイル

### 1. `README.md`（英語）

冒頭に日本語版へのリンク（`[日本語版はこちら](README_ja.md)`）

構成:
1. **タイトル + バッジ**: `board` — CI/release バッジ
2. **概要**: BOARD API 向け Go CLI + ローカル MCP サーバー。SQLite キャッシュで rate limit 対応
3. **Installation**: Homebrew (`brew install youyo/tap/board`) / GitHub Releases / ソースビルド
4. **Quick Start**: `board configure` → `board find client --name "..."` の最小フロー
5. **CLI Commands**:
   - `board configure` — 設定管理
   - `board api <resource> <operation>` — low-level API 操作（22 リソース）
   - `board find <resource>` — high-level 横断検索（12 リソース）
   - `board cache` — キャッシュ管理
   - `board mcp serve` — MCP サーバー起動
   - `board completion` — シェル補完
6. **Configuration**: TOML 設定ファイル、プロファイル管理、設定例
7. **MCP Server**: 起動方法、公開ツール概要
8. **Architecture**: レイヤー図（CLI → service → repository → cache/API）
9. **License**: MIT

### 2. `README_ja.md`（日本語）

冒頭に英語版へのリンク（`[English version](README.md)`）
内容は README.md の日本語翻訳。

## 検証

- リンクの相互参照が正しいこと
- GitHub 上での Markdown レンダリング確認（バッジ URL 等）
