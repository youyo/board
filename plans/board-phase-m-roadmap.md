# Phase M ロードマップ: CLI/Docs 充実化 → v0.6.0 リリース

## Meta
| 項目 | 値 |
|------|---|
| ゴール | `board` バイナリが CLI 補完（固定列挙値まで）・ドキュメント自己記述（`board docs` サブコマンド）・LLM スキル連携（`/board:docs`）を備え、人間にもエージェントにも親しいコマンドになる |
| 成功基準 | 全 4 M 完了 + `go test ./... -count=1` 全 Green + `./board completion zsh` が固定列挙値補完を含む + `./board docs --list` 動作 + `/board:docs` スキル呼び出し成功 + v0.6.0 タグ配信 |
| 制約 | Breaking Change なし（純粋追加のみ）、バイナリサイズ増分は +50KB を上限目標、動的補完は対象外 |
| 対象リポジトリ | /Users/youyo/src/github.com/youyo/board |
| 作成日 | 2026-04-24 |
| 最終更新 | 2026-04-24 |
| ステータス | 未着手 |
| 親計画 | plans/groovy-churning-valley.md（plan-mode 集約プラン） |
| 先行フェーズ | plans/board-phase-l-roadmap.md（Phase L: api 層 BOARD API 完全準拠、v0.5.0 完走） |
| 後続フェーズ | plans/board-phase-n-roadmap.md（Phase N: find 層必要性評価 → v0.7.0） |

## 背景

Phase L（M49–M57、v0.5.0）で `internal/boardapi/` 全 22 リソースが BOARD API 準拠に到達し、
Ransack 形式のクエリと `ListResult[T]{Items, Meta, Headers}` による統一的レスポンスヘッダー伝達が完了した。
一方で **ユーザー体験側** には 4 つの未解決領域が残っている。

1. **CLI の補完がフラグ名止まり**
   - `--response-group` / `--status-eq` など固定列挙フラグでも値補完が効かない
   - `internal/cli/completion.go` は Cobra の `GenZshCompletion` / `GenBashCompletion` を呼ぶだけで、
     `RegisterFlagCompletionFunc` を一度も使っていない
2. **LLM 向けの自己記述性が弱い**
   - 22 リソース × list/get × 全フラグの仕様が `docs/api-reference.md`（549 行）に集約されているが、
     バイナリ内から参照する手段がない（`go:embed` 未使用）
   - ecspresso v2.8 型の `docs` サブコマンドが欲しい
3. **/board:docs スキルが存在しない**
   - `skills/` ディレクトリ自体が未作成
   - エージェントが board の使い方を都度探索している
4. **README / api-reference のサンプル・エラー応答が不足**
   - サンプル JSON / エラー応答例 / Ransack フィルタ完全表が未整備

Phase M で上記 4 点を解消し、v0.6.0 としてリリースする。

## Current Focus
- **マイルストーン**: M61（README / api-reference 拡充 + v0.6.0 リリース）
- **直近の完了**: M60（/board:docs スキル作成、薄いラッパー）
- **次のアクション**: M61 着手 → `docs/api-reference.md` 拡充 + CHANGELOG + v0.6.0 タグ

## Progress

### M58: completion の固定列挙値補完 ✅
- [x] `internal/cli/completion_values.go` を新設（固定列挙マップ + ヘルパ）
- [x] 各 `api_*.go` の List/Get コマンドで `RegisterFlagCompletionFunc` を呼ぶ
- [x] ユニットテスト追加（`completion_values_test.go`、17ケース Green）
- [x] `./board __complete` 経由の補完動作確認（手動 zsh は未実施、CI 検証で代替）
- 📄 詳細: plans/board-phase-m-m58-completion.md

### M59: board docs サブコマンド + JSON 出力 ✅
- [x] `internal/docs/` 新設、`go:embed` で docs/ + README.md を assets 経由で取り込み
- [x] `internal/cli/docs.go` に `newDocsCmd()` + `--list` / `--search` / `--format` フラグ
- [x] リソース抽出関数 `ExtractSection(md, resource)` 実装（`#### <name> — ` アンカー）
- [x] 検索関数 `Search(md, keyword)` 実装（±2 行コンテキスト + 連続ヒットマージ）
- [x] ユニットテスト (docs 16 + cli 12 ケース) + バイナリサイズ計測（実測 +138KB、計画 +50KB を超過したが受容）
- [x] mise task `sync-docs` / `check-docs-sync` で drift 検知運用
- 📄 詳細: plans/board-phase-m-m59-docs-command.md
- 📝 バイナリサイズ増分 +138KB は CHANGELOG で言及

### M60: /board:docs スキル作成（薄いラッパー）✅
- [x] `skills/` ディレクトリ + `.claude-plugin/plugin.json` 作成（Claude Code plugin namespace 成立）
- [x] `skills/docs/SKILL.md` に frontmatter（`name: board:docs`）+ 本文を記述（`board docs` を呼び出す手順）
- [x] README.md / README_ja.md に `/board:docs` スキル利用案内セクション追加
- [x] `internal/docs/skill_test.go` で frontmatter / 本文 / bash コマンド smoke test を TDD（15 ケース Green）
- [x] 別セッションから `/board:docs` 呼び出し検証 → smoke test で機械的に代替（TestMain でバイナリビルド + exec 検証）
- 📄 詳細: plans/board-phase-m-m60-docs-skill.md
- 📝 M61 の v0.6.0 bump 時に `.claude-plugin/plugin.json` の version も `0.6.0` に更新すること

### M61: README / api-reference 拡充 + v0.6.0 リリース
- [ ] `docs/api-reference.md` にサンプル JSON / エラー応答例 / Ransack フィルタ完全表を追加
- [ ] `README.md` / `README_ja.md` に completion / docs / LLM 連携セクションを追加
- [ ] `CHANGELOG.md` に v0.6.0 エントリ
- [ ] `git tag v0.6.0 && git push origin v0.6.0`（GoReleaser が自動配信）
- [ ] `brew upgrade board` で新版取得確認
- 📄 詳細: 着手時に `plans/board-phase-m-m61-readme-release.md` を `/devflow:plan` で生成（遅延生成）
- 📝 依存: M58 / M59 / M60 全完了

## Blockers
なし

## Architecture Decisions

| # | 決定 | 理由 | 日付 |
|---|------|------|------|
| 1 | completion は固定列挙のみ | 動的補完はキャッシュ依存 + 遅延リスク、Phase M のスコープで扱わない | 2026-04-24 |
| 2 | `board docs` はミニマル版 + JSON 出力対応 | 既存 CLI が JSON 前提のため一貫性確保、LLM にも機械可読 | 2026-04-24 |
| 3 | `/board:docs` スキルは薄いラッパー | 情報の二重管理を避け、一次情報は board バイナリ埋め込みドキュメントに一元化 | 2026-04-24 |
| 4 | 埋め込み対象は `docs/` 配下のみ（`docs/specs/` は除外） | 設計書 44KB をバイナリに含めない、実用ドキュメントのみ | 2026-04-24 |
| 5 | M59-M61 の詳細ファイルは遅延生成 | 着手時まで概要のみ、途中で前提条件が変わっても柔軟に対応可能 | 2026-04-24 |

## Out of Scope（Phase M では対応しない）

- 動的補完（API コール / キャッシュ参照による ID 補完）
- `board docs` の fuzzy search / インタラクティブ TUI
- 英語版ドキュメントの自動翻訳機構
- find 層の再設計（Phase N で対応）

## Changelog

| 日時 | 種別 | 内容 |
|------|------|------|
| 2026-04-24 | 作成 | Phase M ロードマップ初版作成（M58-M61、v0.6.0 ターゲット） |

## Next Action

1. M58 詳細計画（`plans/board-phase-m-m58-completion.md`）を参照して実装開始
2. `/devflow:implement` で M58 を実行
3. M58 完了後、M59 詳細を `/devflow:plan` で生成してから着手
