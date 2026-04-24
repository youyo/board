---
title: マイルストーン M61 - README / api-reference 拡充 + v0.6.0 リリース準備
project: board-phase-m
author: Milestone Executor (M61)
created: 2026-04-24
status: Draft / Ready for Review
complexity: M
---

# M61: README / api-reference 拡充 + v0.6.0 リリース準備

## Overview
| 項目 | 値 |
|------|---|
| ステータス | 未着手 |
| 依存 | M58 / M59 / M60 完了済 |
| 対象ファイル | `docs/api-reference.md`（拡充）、`README.md`（最小整理）、`README_ja.md`（最小整理）、`CHANGELOG.md`（v0.6.0 エントリ追加）、`.claude-plugin/plugin.json`（version bump 0.5.0 → 0.6.0）、`.github/workflows/ci.yml`（check-docs-sync 追加）、`internal/docs/assets/*`（sync-docs で再生成） |
| 想定工数 | 半日 |
| 親ロードマップ | plans/board-phase-m-roadmap.md |

## スコープ分離（重要）

### 本マイルストーンで実施
- `docs/api-reference.md` 拡充（サンプル JSON / エラー応答例 / Ransack フィルタ完全表 / 補完値一覧）
- `README.md` / `README_ja.md` の Agent / LLM 連携セクション整理（M60 で既に追加済。重複回避しつつ completion / docs への言及を補強）
- `CHANGELOG.md` に v0.6.0 エントリ追加（M58 + M59 + M60 まとめ、バイナリサイズ +138KB 言及）
- `.claude-plugin/plugin.json` version 0.5.0 → 0.6.0
- `mise run sync-docs` 実行（api-reference.md 拡充後の assets 同期）
- GitHub Actions `ci.yml` に `mise run check-docs-sync` 相当を組み込み（mise セットアップまたは shell 直書きで drift 検知）
- 単一コミットで全変更をまとめる（Conventional Commits 日本語）

### 本マイルストーンでは実施しない（ユーザーが別途実施）
- `git tag v0.6.0`
- `git push origin v0.6.0`
- GoReleaser の動作確認
- `brew upgrade board`

## Goal

Phase M の締めくくりとして、v0.6.0 を **「リリースタグを打つだけの状態」** にする。
ユーザードキュメントを `board` バイナリの実体に沿って揃え、CI に drift 検知を組み込み、
CHANGELOG と plugin manifest のバージョンを揃える。

## 設計（Design）

### 1. `docs/api-reference.md` 拡充方針

既存 549 行の構造（グローバルフラグ / 共通 list フラグ / リソース一覧 / v0.5.0 破壊的変更 / find 使い分け）を保ったまま、**後半に 4 つの新節を追加**する。

#### 1.1 新設節「サンプル JSON」
list / get の代表的な JSON 形状をサンプル提示。抜粋のため実 API dump（`tmp/e2e-artifacts/`）をそのまま貼らず、必須フィールドだけのコンパクト形に整える。

- `clients list` の `{items:[], _meta:{}}` サンプル（--show-meta 有無 2 パターン）
- `projects get` の `{id, name, ...}` サンプル（ネストされた `client`, `estimates` 等）
- `invoices list` の基本形

#### 1.2 新設節「エラー応答例」
BOARD API が返す 4xx/5xx の代表的な形と、`board` CLI が stderr に出力する JSON エラー形式を示す。

- 401 Unauthorized（api_key 不正時）
- 404 Not Found（存在しない ID）
- 429 Too Many Requests（rate limit）
- `board` CLI 側の `{error:true, message:"..."}` 形式（`board docs foobar` / 不正フラグ等）

#### 1.3 新設節「Ransack フィルタ完全表」
既存のリソース別 list フラグ表は散在しているため、**横断的にまとまった一覧表**を末尾に追加。
- 22 リソース × 使用可能 Ransack オペレータを 1 つの表にまとめる
- リソース別詳細は既存の各セクションに任せ、この節は「どのオペレータが使えるか」の俯瞰表に徹する
- 対象オペレータ: `_eq` / `_cont` / `_in` / `_gteq` / `_lteq`

#### 1.4 新設節「CLI 補完値一覧（M58 掲載）」
`board completion zsh` 経由で補完される固定列挙値を、ドキュメントにも明示する。

| フラグ | コマンド | 補完候補 |
|--------|---------|---------|
| `--response-group` | `api clients/invoices/payments/purchase_orders list` | `small`, `large` |
| `--response-group` | `api projects list` | 8 値（small/large/estimate/order/delivery/invoice/receipt/all） |
| `--response-group` | `api projects get` | 6 値（estimate/order/delivery/invoice/receipt/all） |
| `--order-status-in` | `api projects list` | 1=見積中(高) / 2=見積中(中) / 3=見積中(低) / 4=受注確定 / 5=受注済 / 8=見積中(除) |
| `--delivery-status-in` | `api projects list` | 1=未着手 / 2=着手中 / 3=納品済 / 4=検収済 |
| `--invoice-timing-kbn-in` | `api projects list` | 1=一括請求 / 2=定期請求 |

#### 1.5 新設節「board docs サブコマンド（M59 要約）」
`board docs` / `board docs --list` / `board docs <resource> --format json` / `board docs --search <kw>` の使い方を 20 行程度で要約。README との重複を避け、**JSON スキーマ（mode / query / results[]）の詳細**だけ記載する（README はクイック参照、api-reference は仕様書の位置付け）。

### 2. `README.md` / `README_ja.md` の整理方針

M60 で既に「Agent / LLM integration」セクションが追加済。**重複を避けつつ以下を補強**する。

- `board completion` の説明に「固定列挙値補完（M58）」の 1 文追加
- 「Agent / LLM integration」セクションの `board docs` サブセクションに「詳細仕様は `docs/api-reference.md` 参照」のリンク追加
- 新規セクションは作らない（既存セクションへの追記のみ、肥大化回避）

### 3. `CHANGELOG.md` v0.6.0 エントリ

```markdown
## [0.6.0] - 2026-04-24

### New Features

- **`board docs` サブコマンド追加（M59）**: バイナリに埋め込まれた README / api-reference / installation / guides を CLI から参照可能。`--list`, `--search`, `--format json` をサポート
- **`/board:docs` Claude Code スキル追加（M60）**: `.claude-plugin/plugin.json` + `skills/docs/SKILL.md` により AI エージェントから BOARD CLI の使い方をオフライン参照可能
- **shell completion の値補完対応（M58）**: `--response-group` / `--order-status-in` / `--delivery-status-in` / `--invoice-timing-kbn-in` の固定列挙値を zsh/bash/fish で TAB 補完可能
- **api-reference.md 拡充（M61）**: サンプル JSON / エラー応答例 / Ransack フィルタ完全表 / CLI 補完値一覧を追加

### Internal

- `internal/docs/` + `go:embed` でドキュメントをバイナリに取り込み（README / docs/ 配下、specs/ 除外）
- `mise run sync-docs` / `mise run check-docs-sync` タスク追加、CI に drift 検知組み込み
- `.claude-plugin/plugin.json` version を `0.5.0` → `0.6.0` に bump

### Notes

- **バイナリサイズ**: 22.07 MB → 22.21 MB（+138 KB、+0.64%）。埋め込みドキュメント 52 KB + `go:embed` FS メタデータ + `encoding/json` / `io/fs` リンク分
```

### 4. `.claude-plugin/plugin.json` version bump

```diff
-  "version": "0.5.0",
+  "version": "0.6.0",
```

### 5. GitHub Actions `ci.yml` への check-docs-sync 組み込み

既存ジョブ `test` の `Run tests` ステップ前に mise セットアップ + check-docs-sync を追加。mise 導入は既存 CI で未使用なので、GHA 上は shell 直書き（`rsync` + `diff -r`）で drift 検知する方針（依存ツール最小化）。

```yaml
      - name: Check docs sync
        run: |
          tmp=$(mktemp -d)
          cp README.md "$tmp/README.md"
          rsync -a --exclude=specs docs/ "$tmp/"
          diff -r "$tmp" internal/docs/assets
          rm -rf "$tmp"
```

これを `Run go vet` の前に挟む（drift を最初に検知）。

### 6. sync-docs の実行順序

1. api-reference.md を拡充（docs/api-reference.md 編集）
2. **`mise run sync-docs` 実行** → `internal/docs/assets/api-reference.md` 更新
3. `go test ./...` 実行（docs_test.go の TestSearch_Ransack / TestList など、新しいサイズで再検証）
4. `mise run build` でバイナリ再構築 → `skill_test.go` の smoke test が通ることを確認

## TDD テスト設計

本 M61 は **ロジック変更なし / ドキュメントと設定のみ変更** のため、新規の単体テストは書かない。
ただし以下の既存テストが api-reference.md 拡充後も Green であることを確認する:

| テスト | 期待 |
|--------|------|
| `internal/docs/docs_test.go::TestList` | `results` 長が 6 のまま（新規ファイル追加なし） |
| `internal/docs/docs_test.go::TestExtractSection_Clients` | api-reference.md 構造変更で既存セクションの切り出しが壊れていない |
| `internal/docs/docs_test.go::TestSearch_Ransack` | 拡充後に Ransack マッチ件数が増えても Green（実測 assertion: `if len(matches) < 3`、増加方向は許容） |
| `internal/docs/docs_test.go::TestExtractSection_StopAtUpperHeading` | `document_send_channels` が `## v0.5.0 破壊的変更` で切れる — 新設節追加で壊れないか要注意 |
| `internal/docs/skill_test.go::*` | plugin.json の version 変更は `TestPluginManifest_Valid` の `name == "board"` 検証だけなので影響なし |
| `internal/cli/docs_test.go::TestDocs_Resource` | 既存セクション抽出が壊れていない |

**特に注意**: 拡充で追加する「サンプル JSON」「エラー応答例」「Ransack フィルタ完全表」「CLI 補完値一覧」は **`## ` （H2 見出し）** で追加し、`### ` / `#### ` を使わないことで、`ExtractSection` の境界検出ロジック（`#### xxx —` / `### ` / `## ` で終了判定）に矛盾しないようにする。

ただし既存の `document_send_channels` セクションが `## v0.5.0 破壊的変更` で終了している境界の直前に何か入るわけではなく、新設節は `## find コマンドとの使い分け` の前後に配置する。順序:

1. 既存: リソース一覧
2. 既存: `## v0.5.0 破壊的変更`
3. **新設: `## サンプル JSON`**
4. **新設: `## エラー応答例`**
5. **新設: `## Ransack フィルタ完全表`**
6. **新設: `## CLI 補完値一覧`**
7. **新設: `## board docs サブコマンド`**
8. 既存: `## find コマンドとの使い分け`
9. 既存: `## 関連ドキュメント`

## 実装手順

### Step 1: `docs/api-reference.md` 拡充
- 末尾 `## find コマンドとの使い分け` の直前に新設節 5 本を挿入
- 既存のリソース別セクションは変更しない（`ExtractSection` テスト保護のため）

### Step 2: `mise run sync-docs` 実行
- macOS sandbox 下では mise タスクの mktemp が失敗するため、直接 rsync で実行:
  ```bash
  rm -rf internal/docs/assets
  mkdir -p internal/docs/assets
  cp README.md internal/docs/assets/README.md
  rsync -a --exclude=specs docs/ internal/docs/assets/
  ```
- もしくは `mise run sync-docs`（Linux/CI では問題なし）

### Step 3: `README.md` / `README_ja.md` 最小整理
- `board completion` 節に固定列挙値補完の 1 文追加
- 「Agent / LLM integration」 → `board docs` サブセクションに `docs/api-reference.md` へのリンク追加

### Step 4: `CHANGELOG.md` v0.6.0 エントリ追加
- `[Unreleased]` と `[0.5.0]` の間に `[0.6.0] - 2026-04-24` ブロックを挿入

### Step 5: `.claude-plugin/plugin.json` version bump
- `"version": "0.5.0"` → `"version": "0.6.0"`

### Step 6: GHA `ci.yml` に docs sync 検証追加
- `Run go vet` の前に `Check docs sync` ステップ挿入（shell 直書き、依存ツールは rsync + diff のみ）

### Step 7: sync-docs 再実行（README.md 更新反映）
- README.md 編集後に再度 `internal/docs/assets/README.md` を同期

### Step 8: 全体検証
- `go test ./... -count=1`
- `go vet ./...`
- `gofmt -s -w .`（Go ファイル変更ないはずだが念のため）
- `mise run build` でバイナリ再構築
- `./board docs --list` / `./board docs clients` / `./board docs --search Ransack` が期待通り動作

### Step 9: コミット（明示 `git add` で対象ファイルのみステージ）

**advisor 指摘（blocking）対応**: ワーキングディレクトリに M61 無関係の modified / untracked ファイルが残っているため、`git add -A` / `git add .` / `git commit -a` は **禁止**。対象ファイルのみ明示指定する:

```bash
git add docs/api-reference.md
git add README.md README_ja.md
git add CHANGELOG.md
git add .claude-plugin/plugin.json
git add .github/workflows/ci.yml
git add internal/docs/assets/
git add plans/board-phase-m-m61-readme-release.md
git add plans/board-phase-m-roadmap.md  # M61 完了マーク更新分
```

**M61 スコープ外（今回はコミットしない）**:
- `internal/boardapi/e2e_deliveries_m53_test.go` / `e2e_orders_m53_test.go` / `e2e_receipts_m53_test.go`（modified、E2E テストの未コミット変更、M53 関連で別途対応）
- `plans/board-phase-n-roadmap.md`（Phase N 先行計画、別コミット対象）
- `plans/groovy-churning-valley.md`（親プラン、別コミット対象）

commit:
- Conventional Commits 日本語、単一コミット
- メッセージ: `chore(release): M61 v0.6.0 リリース準備 — api-reference 拡充 / README / CHANGELOG / plugin.json bump / GHA drift 検知`
- push はしない（ユーザーが別途実施）

### Step 10: ロードマップ更新
- `plans/board-phase-m-roadmap.md` の M61 チェックボックスを `[x]` に更新
- ステータスを「完走（v0.6.0 リリース準備完了）」に

## リスク評価

| # | リスク | 重大度 | 対策 |
|---|--------|--------|------|
| R1 | api-reference.md 拡充で `ExtractSection` テストが壊れる | **高** | 新設節は全て `## ` で追加、既存リソース別セクションは一切変更しない。`TestExtractSection_StopAtUpperHeading` の境界は新設節の配置順で保持 |
| R2 | sync-docs を忘れて drift が残ったまま commit | **高** | Step 2 / Step 7 で明示実行。さらに GHA に drift 検知を追加（Step 6）して今後のリグレッション防止 |
| R3 | `TestSearch_Ransack` が件数増加で壊れる | 低 | 現状 `>= 5` の閾値。新設節に Ransack の記載を追加するので件数は増える方向 |
| R4 | README 編集で `TestDocs_Readme` の「# board」検出が壊れる | 低 | README 先頭の H1 は変更しない |
| R5 | `.claude-plugin/plugin.json` version bump を忘れる | 中 | Step 5 で明示。M60 の申し送りとしても残っている |
| R6 | GHA ci.yml の docs sync ステップが既存ジョブを壊す | 中 | 既存ステップ（vet / golangci-lint / test）の前に追加するだけ。drift がなければ exit 0。初回 push 時に drift があると fail するため、本コミット前に sync-docs 実行を必須化 |
| R7 | CHANGELOG v0.6.0 の日付が実際のリリース日と異なる | 低 | 「リリース準備完了日」として 2026-04-24 を記録。実際のタグ push 日と異なっても CHANGELOG の慣習上問題なし |
| R8 | Ransack フィルタ完全表で 22 リソース × 5 オペレータを手書き管理する乖離リスク | 中 | 全オペレータを全リソースに書くのではなく、「リソース別詳細は上記セクション参照」と注記。横断表は「どのオペレータが使えるか」の俯瞰に絞る |
| R9 | GHA で macOS 用 shell 構文を書くと Ubuntu で動かない | 低 | `mktemp -d` / `rsync` / `diff -r` は全て GNU coreutils で動作 |
| R10 | 単一コミット方針で commit 差分が大きくなりレビュー困難 | 低 | 純粋ドキュメント + 設定 bump のみ。コード変更はない。レビューコストは内容把握のみ |

## 検証項目

### ローカル検証
- [ ] `go test ./... -count=1` 全 Green
- [ ] `go vet ./...` warning なし
- [ ] `gofmt -s -w .` 差分なし
- [ ] `./board docs --list` 6 件表示
- [ ] `./board docs clients` で `clients — 顧客マスタ` セクション表示
- [ ] `./board docs --search Ransack --format json | jq '.results | length'` >= 5
- [ ] docs sync 手動検証: `rsync -a --exclude=specs docs/ tmp/` + `cp README.md tmp/README.md` + `diff -r tmp internal/docs/assets` が exit 0

### リリース後確認項目（ユーザー側、本 M61 スコープ外）
- [ ] `git tag v0.6.0 && git push origin v0.6.0`
- [ ] GoReleaser が `release` job で自動配信成功
- [ ] `brew upgrade board` で v0.6.0 取得成功
- [ ] `board --version` で `0.6.0` 表示

## 受入基準（Success Criteria）

- [ ] `docs/api-reference.md` に新設節 5 本が追加されている
- [ ] `README.md` / `README_ja.md` の Agent/LLM セクションが api-reference への導線を持つ
- [ ] `CHANGELOG.md` に v0.6.0 エントリが追加されている
- [ ] `.claude-plugin/plugin.json` の version が `0.6.0` になっている
- [ ] `.github/workflows/ci.yml` に docs sync drift 検知ステップが追加されている
- [ ] `internal/docs/assets/` が最新 docs と同期済み
- [ ] `go test ./... -count=1` 全 Green
- [ ] 単一コミットで全変更がまとまっている（Conventional Commits 日本語）

## Next Action（完了後、ユーザー側）

1. PR を作成（`feature-m60-docs-skill` ブランチを main にマージ）または直接 main にマージ
2. `git tag v0.6.0` + `git push origin v0.6.0`
3. GoReleaser の release job をモニター
4. リリース後 `brew upgrade board` で取得確認

---

**親ロードマップ**: plans/board-phase-m-roadmap.md
**先行マイルストーン**: M58 (completion)、M59 (docs command)、M60 (docs skill)
**後続フェーズ**: Phase N（find 層必要性評価 → v0.7.0）
**作成日**: 2026-04-24
