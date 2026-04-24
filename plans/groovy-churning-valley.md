# Plan: Phase M + Phase N ロードマップ（CLI/Docs 充実化 → find 層再検討）

このファイルは ExitPlanMode 用の集約プラン。承認後、Implementer Agent 経由で
plans/board-phase-m-roadmap.md / plans/board-phase-n-roadmap.md
および各マイルストーン詳細ファイルに分割する。

## Context — なぜこの計画が必要か

Phase L（M49–M57、v0.5.0）で `internal/boardapi/` 全 22 リソースが BOARD API 準拠に到達し、
Ransack 形式のクエリと `ListResult[T]{Items, Meta, Headers}` による統一的レスポンスヘッダー伝達が完了した。
一方で **ユーザー体験側**には 4 つの未解決領域が残っている。

1. **CLI の補完がフラグ名止まり**: `--response-group`/`--status-eq` など固定列挙フラグでも値補完が効かない。
   `internal/cli/completion.go` は Cobra の `GenZshCompletion` / `GenBashCompletion` を呼ぶだけで、
   `RegisterFlagCompletionFunc` を一度も使っていない。
2. **LLM 向けの自己記述性が弱い**: 22 リソース × list/get × 全フラグの仕様が `docs/api-reference.md`（549 行）に
   集約されているが、バイナリ内から参照する手段がない（`go:embed` 未使用）。ecspresso v2.8 型の `docs` サブコマンドが欲しい。
3. **/board:docs スキルが存在しない**: `skills/` ディレクトリ自体が未作成。エージェントが board の使い方を
   都度探索している現状。
4. **find 層（`internal/service/find/`）の存在意義が再検証できていない**: Phase H（M25–M32）で 47 E2E テストまで
   作ったが、TODO(M25-M32) の enrichment 復元・Status post-filter 未実装・11,748 件 invoice での timeout・
   vendor/group の 0 件 SKIP が積み上がっている。ユーザーは「api 層を実装してみた結果、そもそも find 層が必要かから
   問い直したい」との判断。

Phase M で CLI/Docs を磨き v0.6.0、Phase N で find 層の必要性を評価したうえで廃止 or 再設計を決定し v0.7.0、
という 2 段階で進める。

## Goal / Success Criteria

### Phase M（v0.6.0）
- ✅ `board api ... --response-group <TAB>` で `small/large/estimate/...` が候補表示される（zsh/bash）
- ✅ `board docs` / `board docs --list` / `board docs --search <kw>` / `board docs <resource>` / `--format json` が動作する
- ✅ `skills/docs/SKILL.md` 経由で `/board:docs` が使える（薄いラッパー、一次情報は `board docs` 側）
- ✅ `docs/api-reference.md` にサンプル JSON / エラー応答例 / Ransack フィルタ完全表が追加される
- ✅ v0.6.0 リリース（GoReleaser + Homebrew tap 自動配信）

### Phase N（v0.7.0）
- ✅ `docs/adr/ADR-001-find-layer.md` で find 層の扱い（廃止 / 再設計 / 数本残す）を決定
- ✅ 決定に基づき `internal/service/find/` の実装更新 + E2E テストが新方針で全 Green
- ✅ MCP 接続部も新方針に沿って刷新
- ✅ v0.7.0 リリース

## Scope / Non-Goals

- **In scope**
  - completion は固定列挙のみ（IntSlice 系 `order-status-in` などもコード内マップで値+意味を補完）
  - `board docs` はテキスト出力 + `--format json` 対応、README + api-reference + guides を埋め込み
  - `/board:docs` スキルは薄いラッパー（実体は board バイナリ）
  - find 層の意思決定は ADR + 設計方針案 + 調査レポートの 3 点セット
- **Out of scope**
  - 動的補完（API コール/キャッシュ参照による ID 補完）は Phase M では対象外
  - Phase N の実装部分はロードマップ作成時点では未確定（N01 の意思決定結果依存）

## Phase M ロードマップ概要

| M | 名称 | ゴール | 状態 |
|---|------|--------|------|
| **M58** | completion: 固定列挙の値補完 | `--response-group`/`--status-eq`/`*-in` 系が `<TAB>` で候補表示される | 未着手 |
| **M59** | docs サブコマンド + JSON 出力 | `board docs [--list|--search|--format json] [<resource>]` が動作 | 未着手 |
| **M60** | /board:docs スキル作成 | `skills/docs/SKILL.md` + プラグインメタ。エージェントが `board docs` を呼び出す | 未着手 |
| **M61** | README/api-reference 拡充 + v0.6.0 リリース | サンプル JSON・エラー応答・Ransack 表追加、CHANGELOG、タグ付け、GoReleaser | 未着手 |

## Phase N ロードマップ概要（N01 のみ詳細、N02 以降は概要）

| M | 名称 | ゴール | 状態 |
|---|------|--------|------|
| **N01** | find 層必要性評価 + 設計方針案 + ADR | 利用実績ベースで find 層を評価し「廃止 / 再設計 / 一部残す」の意思決定 | 未着手 |
| **N02+** | 意思決定に基づく実装 | N01 結果依存（廃止なら削除 + MCP を api 層直呼び、再設計なら新仕様の実装） | 未定 |
| **N終盤** | v0.7.0 リリース | CHANGELOG + タグ付け | 未定 |

**N01 開始時点では `internal/service/find/` / `internal/cli/find_*.go` / MCP find_* tool は現状維持**。
意思決定後に一括で廃止 or 書き換え。

---

## M58 詳細計画: completion の固定列挙値補完

### Overview
| 項目 | 値 |
|------|---|
| ステータス | 未着手 |
| 依存 | なし |
| 対象ファイル | `internal/cli/api_*.go`（22 リソース）、`internal/cli/completion_values.go`（新規）、`internal/cli/completion.go`（既存・テスト追加） |
| 想定工数 | 半日〜1日 |

### Goal
以下のフラグを `RegisterFlagCompletionFunc` で補完対象にする。
**固定列挙のみ。** 動的補完（API/キャッシュ参照）は Phase M では対象外。

### 対象フラグと補完候補（現状のソース調査結果）

| フラグ | 対象コマンド | 補完候補 |
|--------|------------|----------|
| `--response-group` | `api clients list`, `api invoices list`, `api payments list`, `api purchase_orders list` | `small`, `large` |
| `--response-group` | `api projects list` | `small`, `large`, `estimate`, `order`, `delivery`, `invoice`, `receipt`, `all` |
| `--response-group` | `api projects get` | `estimate`, `order`, `delivery`, `invoice`, `receipt`, `all` |
| `--status-eq` | `api invoices list` | `draft`, `sent`, `paid` |
| `--status-eq` | `api purchase_orders list` | `draft`, `approved`, `sent` |
| `--status-eq` | `api payments list` | `pending`, `paid` |
| `--order-status-in` | `api projects list` | 整数: `1`(見積中) / `2`(見積送付済) / `5`(受注済) / `8`(失注) 等（コード内マップから `1\t見積中` の description 形式で補完） |
| `--delivery-status-in` | `api projects list` | `1`(未着手) / `3`(納品済) / `4`(検収済) 等 |
| `--invoice-timing-kbn-in` | `api projects list` | 0-5 の区分値（既存ソースから抽出） |

### Implementation Steps

- [ ] **Step 1**: `internal/cli/completion_values.go` を新設
  - `responseGroupCommon` = `[]string{"small", "large"}`
  - `responseGroupProjectsList` = `[]string{"small","large","estimate","order","delivery","invoice","receipt","all"}`
  - `responseGroupProjectsGet` = `[]string{"estimate","order","delivery","invoice","receipt","all"}`
  - `statusEqInvoices`, `statusEqPurchaseOrders`, `statusEqPayments`
  - `orderStatusMap` / `deliveryStatusMap` / `invoiceTimingKbnMap`: `map[int]string`
  - ヘルパ: `staticCompletion([]string) cobra.CompletionFunc`, `intMapCompletion(map[int]string) cobra.CompletionFunc`
- [ ] **Step 2**: 各 `api_*.go` の `newXxxListCmd()` / `newXxxGetCmd()` で該当フラグに `cmd.RegisterFlagCompletionFunc(...)` を呼ぶ
- [ ] **Step 3**: ユニットテスト追加 (`internal/cli/completion_values_test.go`)
  - 各補完関数が期待値を返すことを検証
  - `cobra.ShellCompDirectiveNoFileComp` の指定確認
- [ ] **Step 4**: 実動作確認
  - `go build && ./board completion zsh > _board` で補完ファイル生成
  - `zsh` シェル上で `board api projects list --response-group <TAB>` を手動検証
- [ ] **Step 5**: README の該当セクションに「補完対応フラグ一覧」の小セクションを追加（M61 とまとめるか個別か検討）

### Risks
| リスク | 影響 | 対策 |
|--------|------|------|
| Cobra のバージョンによって API 差異あり | 中 | `go.mod` で `spf13/cobra` 確認、`RegisterFlagCompletionFunc` が存在するバージョンを確認 |
| IntSlice 補完のフォーマット | 小 | cobra は `tab\tdescription` 形式をサポート。`"1\t見積中"` のような形式で返す |
| fish/pwsh 対応 | 小 | Phase M では zsh/bash のみ対象。fish は Cobra の `GenFishCompletion` で自動対応する |

### Verification
```bash
mise run build
./board completion zsh > /tmp/_board && fpath=(/tmp $fpath) && autoload -U compinit && compinit
# 手動で ./board api projects list --response-group <TAB> を叩く
go test ./internal/cli/... -run TestCompletion -count=1
```

---

## M59 詳細計画: board docs サブコマンド

### Overview
| 項目 | 値 |
|------|---|
| ステータス | 未着手 |
| 依存 | なし（M58 と並行可） |
| 対象ファイル | `internal/embed/docs.go`（新規、go:embed）、`internal/cli/docs.go`（新規）、`cmd/board/main.go` または `internal/cli/root.go` |
| 想定工数 | 1日 |

### Goal
ecspresso v2.8 風 `docs` サブコマンド。**ミニマル版 + JSON 出力**。

```
board docs                          # README を pager で表示
board docs --list                   # 埋め込みドキュメント一覧
board docs --search <keyword>       # 全文検索（マッチ行を出力）
board docs <resource>               # api-reference.md から該当リソース節を抽出
board docs --format json [...]      # すべてのモードで JSON 出力対応
```

### Design
- `internal/embed/docs.go`
  ```go
  package embed
  import "embed"
  //go:embed docs/README.md docs/api-reference.md docs/installation*.md docs/guides/*.md
  var Docs embed.FS
  ```
  ※ 埋め込み対象は `docs/` 配下のみ。`docs/specs/` の超詳細設計書（44KB）は埋め込まない
- `internal/cli/docs.go` に `newDocsCmd()`。`spf13/cobra` で実装
- リソース抽出は `api-reference.md` 内の `#### {resource} —` ヘッダーをアンカーに次の `---` までを切り出す
- 検索はシンプルな `strings.Contains`（大文字小文字無視）。ヒット行周辺 ±2 行を出力
- JSON 出力形式:
  ```json
  {
    "mode": "resource" | "list" | "search" | "readme",
    "query": "<keyword if search>",
    "results": [ {"file":"docs/api-reference.md","section":"clients","content":"..."} ]
  }
  ```

### Implementation Steps

- [ ] **Step 1**: `internal/embed/` パッケージ新設 + `go:embed` で README/api-reference/installation/guides を取り込む
- [ ] **Step 2**: `internal/cli/docs.go` に `newDocsCmd()` + サブフラグ `--list`, `--search`, `--format`（`text|json`）
- [ ] **Step 3**: リソース抽出関数: `ExtractSection(md, resource string) (string, error)` — 見出しベース
- [ ] **Step 4**: 検索関数: `Search(md, keyword string) []Match` — 行単位マッチ + 前後コンテキスト
- [ ] **Step 5**: ユニットテスト
  - `docs embed` が非ゼロサイズ
  - `ExtractSection("clients")` が「clients — 顧客」節を返す
  - `Search("Ransack")` が 5 件以上ヒット
  - `--format json` で JSON 妥当性（`encoding/json` でアンマーシャル）
- [ ] **Step 6**: ルート登録（`internal/cli/root.go` または `cmd/board/main.go`）
- [ ] **Step 7**: バイナリサイズ計測（許容: +50KB 以内を目標）

### Risks
| リスク | 影響 | 対策 |
|--------|------|------|
| `go:embed` パス指定の相対性 | 中 | go:embed は Go ソースファイルからの相対パス。`internal/embed/docs.go` からは `../../docs/...` が必要 → 方針 B: リポジトリルートに `docs.go` を置くか、`internal/embed/docs.go` をルートに置く |
| api-reference.md のセクション区切り規則が変わると抽出が壊れる | 中 | 抽出ロジックは正規表現ベース、変更時はテスト側で検知 |
| docs/ 内のリンクが埋め込みテキストだと壊れる | 小 | 出力時にそのまま表示でよい（コンソール上ではハイパーリンク非対応でも可） |

### Verification
```bash
mise run build
./board docs
./board docs --list
./board docs --search "Ransack"
./board docs clients
./board docs clients --format json | jq .
go test ./internal/cli/... -run TestDocs -count=1
go test ./internal/embed/... -count=1
```

---

## M60 詳細計画: /board:docs スキル作成

### Overview
| 項目 | 値 |
|------|---|
| ステータス | 未着手 |
| 依存 | **M59 完了後**（board docs が先に動作する必要あり） |
| 対象ファイル | `skills/docs/SKILL.md`（新規）、`skills/README.md`（新規、任意） |
| 想定工数 | 半日 |

### Goal
Claude Code / devflow skill からロードできる薄いラッパー。`board docs` を呼び出す手順を LLM に伝える。

### Design
- `skills/docs/SKILL.md`
  - frontmatter: `name: docs`, `description: board コマンドの使い方を参照する...`, `allowed-tools: [Bash]`
  - 本文:
    - 「まず `board docs --list` で目次を取得」
    - 「具体的なリソースなら `board docs <resource> --format json`」
    - 「横断キーワード検索は `board docs --search <keyword>`」
    - 「バイナリに埋め込み済みなのでオフラインで動作」
  - トラブルシュート: board バイナリ未インストール時の案内（`mise run build` で自前ビルド）

### Implementation Steps

- [ ] **Step 1**: `skills/` ディレクトリ作成 (git で管理)
- [ ] **Step 2**: `skills/docs/SKILL.md` を作成
- [ ] **Step 3**: README に「`/board:docs` スキルは Claude Code プラグイン経由で使える」旨を記載
- [ ] **Step 4**: 実際に別セッションの Claude Code から `/board:docs` 呼び出しが成立することを確認

### Risks
| リスク | 影響 | 対策 |
|--------|------|------|
| skills/ のファイル構造が devflow プラグイン仕様と合わない | 中 | 既存の `.claude/skills/` 等の slash 定義を確認してから書く |
| board バイナリが LLM 環境に未インストール | 中 | SKILL.md の冒頭で前提条件を明示 + `go run ./cmd/board docs ...` でも動く旨を併記 |

### Verification
- 別セッションで `/board:docs` を呼び出し、期待通り `board docs --list` が実行されるかを確認

---

## M61 詳細計画: README/api-reference 拡充 + v0.6.0 リリース

### Overview
| 項目 | 値 |
|------|---|
| ステータス | 未着手 |
| 依存 | M58, M59, M60 完了後 |
| 対象ファイル | `docs/api-reference.md`、`README.md`, `README_ja.md`、`CHANGELOG.md`、`.goreleaser.yaml`（確認のみ） |
| 想定工数 | 1日 |

### Goal
- 既存 549 行の api-reference にサンプル JSON + エラー応答 + Ransack 完全表を追加
- README に completion 節 + docs 節を追加
- CHANGELOG に M58-M61 をまとめる
- v0.6.0 タグを push（GoReleaser が自動リリース）

### Implementation Steps

- [ ] **Step 1**: `docs/api-reference.md` 拡充
  - 各リソースに「サンプル出力」サブセクション（`board api <r> list --limit 1` の実結果の抜粋）
  - 「エラーレスポンス」節を新設（UNAUTHORIZED / RATE_LIMIT / NOT_FOUND 等 7 種の JSON 例）
  - 「Ransack フィルタ一覧」節を新設（全 22 リソース × 全 Ransack フラグの表）
- [ ] **Step 2**: `README.md` / `README_ja.md` の拡充
  - 「補完」セクション（zsh/bash への設定手順）
  - 「docs」セクション（`board docs` の使い方）
  - 「LLM 連携」セクション（`/board:docs` と MCP の使い分け）
- [ ] **Step 3**: `CHANGELOG.md` に M58-M61 を記載
- [ ] **Step 4**: v0.6.0 タグを push
  - `git tag v0.6.0 && git push origin v0.6.0`
  - GoReleaser の自動配信と Homebrew tap 更新を監視
- [ ] **Step 5**: リリース確認
  - `brew upgrade board` で新バージョンが取れる
  - `board --version` が `0.6.0` を返す
  - `board completion zsh` が M58 の補完を含む
  - `board docs --list` が動作

### Risks
| リスク | 影響 | 対策 |
|--------|------|------|
| v0.5.0 からの Breaking Change 有無 | 中 | completion/docs は純粋追加なので Breaking なし。CHANGELOG に Non-Breaking を明記 |

### Verification
```bash
brew upgrade board || brew install youyo/tap/board
board --version
board completion zsh > /tmp/_board
board docs --list
```

---

## N01 詳細計画: find 層必要性評価 + ADR

### Overview
| 項目 | 値 |
|------|---|
| ステータス | 未着手 |
| 依存 | Phase M 完了後（v0.6.0 リリース後に開始） |
| 対象ファイル | `plans/board-phase-n-m01-find-rationale.md`（調査レポート）、`docs/adr/ADR-001-find-layer.md`（新規）、`docs/specs/board_cli_mcp_ultra_detailed_design_ja.md`（関連節の補記） |
| 想定工数 | 2-3日（実装なしの純調査） |

### Goal
**find 層を廃止 / 再設計 / 数本に絞るかを意思決定する。**
そのために以下を成果物として出す。

1. **調査レポート**: `plans/board-phase-n-m01-find-rationale.md`
2. **設計方針案**: 上記レポートに 3 つの選択肢と trade-off 分析
3. **ADR**: `docs/adr/ADR-001-find-layer.md` に最終決定を記録

### 調査観点

- [ ] **観点 1: 現状の find 層が提供している「付加価値」の棚卸し**
  - 12 Find メソッド（FindClient, FindProject, FindEstimate, FindOrder, FindDelivery, FindReceipt, FindInvoice, FindVendor, FindPurchaseOrder, FindPayment, FindUser, FindGroup）
  - それぞれが api 層の何を補完しているか（例: enrichment, 名前→ID 解決, 複数モード対応, post-filter）
  - 各メソッドを api 層単体で代替する際の複雑さを記述

- [ ] **観点 2: api 層の Ransack フィルタで代替できるか**
  - Phase L で追加された `--name-cont` / `--client-name-cont` / `--*-eq` / `--*-in[]` 等を使うと、
    find 層のどのメソッドが不要になるか
  - 例: `FindProject(ClientName=xxx)` → `api projects list --client-name-cont xxx` で完全代替可能か？

- [ ] **観点 3: BOARD サービスサイトの機能とのギャップ**
  - BOARD 本体サイトの UI で「どんな検索ができるか」（柔軟検索、統合ビュー）
  - api 層だけで同等の体験が出せるか
  - find 層が本当に必要な「柔軟検索」「統合データ」はどのユースケースか

- [ ] **観点 4: 既存の利用実績**
  - `internal/cli/find_*.go` の CLI コマンド利用履歴（可能なら git blame / PR で確認）
  - `internal/mcpserver/tools.go` の 12 MCP tool のうち、実使用例があるもの
  - 未使用 / 過剰実装のメソッドを特定

- [ ] **観点 5: 現状の技術的負債**
  - TODO(M25-M32) の 4 箇所（find_estimate/order/delivery/receipt の enrichment 復元 + Status post-filter）
  - 47 E2E テスト中 15+ が SKIP（vendor 0 件 / group 0 件 / cache-warm 必須）
  - 11,748 件 invoices での timeout 問題
  - これらを真正面から解決するコスト vs 廃止するコストの比較

### 意思決定の選択肢テンプレート

レポートには以下 3 つを提示する。

| 選択肢 | 概要 | Pros | Cons |
|--------|------|------|------|
| **A: 全廃止** | `internal/service/find/` と `internal/cli/find_*.go`、MCP の find_* tools を削除。MCP は api 層を直接呼ぶ構造に変更 | シンプル。api 層が成熟した今なら付加価値が薄い可能性 | 一部の「横断検索」「統合ビュー」が失われる |
| **B: ゼロベース再設計** | 既存を全廃棄 → BOARD サイトのUX に寄せた柔軟検索 API を新設計 | UXが大きく改善 | 実装コスト最大、v0.7.0 リリースが遠のく |
| **C: 数本に絞る** | 本当に api 層で代替できない find メソッド（例: ClientName→Project 逆引き等）だけを残し、他は廃止 | 現実的な落としどころ | 中途半端な複雑さが残る可能性 |

### Implementation Steps

- [ ] **Step 1**: `plans/board-phase-n-m01-find-rationale.md` の雛形を作成
- [ ] **Step 2**: 観点 1-5 のデータ収集（grep / Read / 既存ドキュメント精読）
- [ ] **Step 3**: 「12 Find メソッド × api 層代替可否」の大表を作成
- [ ] **Step 4**: 選択肢 A/B/C の trade-off 分析
- [ ] **Step 5**: ユーザーにレビュー依頼 → 意思決定
- [ ] **Step 6**: 決定内容を `docs/adr/ADR-001-find-layer.md` に記録
- [ ] **Step 7**: `docs/specs/board_cli_mcp_ultra_detailed_design_ja.md` の関連節に補記（`find` 層の章）

### Risks
| リスク | 影響 | 対策 |
|--------|------|------|
| 判断材料が主観的になる | 中 | 観点 2（api 層で代替可否）は実コード検証（PoC script）で客観化 |
| 意思決定が遅延する | 中 | 選択肢を A/B/C に絞り、ユーザー判断を 1 回で取る前提で設計 |

### Verification
- レポートを別 LLM またはアドバイザに読ませて「3 つの選択肢が trade-off を網羅しているか」を評価
- ADR が将来の意思決定トレーサビリティに耐える粒度か（他の ADR ベストプラクティスと比較）

---

## Phase N 後続（N02 以降）の概要

N01 の意思決定結果により分岐するため、この時点では概要のみ。

### もし選択肢 A（全廃止）になった場合
- N02: `internal/service/find/` 削除
- N03: `internal/cli/find_*.go` 削除
- N04: MCP tools を api 層直接呼び出しに書き換え
- N05: v0.7.0 リリース（Breaking Change として CHANGELOG 強調）

### もし選択肢 B（ゼロベース再設計）になった場合
- N02: 新仕様策定（Query/Result 型、FindXxx API 設計）
- N03-N07: リソース別に実装（client / project / document / vendor / master）
- N08: MCP tools 刷新
- N09: E2E テスト再構築
- N10: v0.7.0 リリース

### もし選択肢 C（数本に絞る）になった場合
- N02: 残すメソッドを確定
- N03-N04: 残すメソッドの再実装（enrichment / post-filter 含む）
- N05: 不要メソッド削除
- N06: MCP tools 整理
- N07: v0.7.0 リリース

## Architecture Decisions

| # | 決定 | 理由 | 日付 |
|---|------|------|------|
| 1 | completion は固定列挙のみ | 動的補完はキャッシュ依存 + 遅延リスク、Phase M のスコープで扱わない | 2026-04-24 |
| 2 | `board docs` はミニマル版 + JSON 出力 | 既存 CLI が JSON 前提のため一貫性確保、LLM にも機械可読 | 2026-04-24 |
| 3 | `/board:docs` スキルは薄いラッパー | 情報の二重管理を避け、一次情報は board バイナリ埋め込みドキュメントに一元化 | 2026-04-24 |
| 4 | find 層は N01 の調査結果で意思決定 | 「api 層実装後に改めて必要性を見直したい」とのユーザー判断 | 2026-04-24 |
| 5 | Phase N 調査中は既存 find 層は現状維持 | 機能欠落期間を作らない | 2026-04-24 |
| 6 | Phase M → Phase N の順で進める | UX 改善（completion/docs）を先行リリースしユーザー価値を早く届ける | 2026-04-24 |

## Critical Files（変更予定）

### Phase M
- `internal/cli/completion_values.go` **(新規)** — 固定列挙マップ
- `internal/cli/api_*.go` **(22 ファイル編集)** — `RegisterFlagCompletionFunc` 追加
- `internal/embed/docs.go` **(新規)** — `go:embed` 埋め込み
- `internal/cli/docs.go` **(新規)** — `board docs` サブコマンド
- `internal/cli/root.go` または `cmd/board/main.go` **(編集)** — `docs` コマンド登録
- `skills/docs/SKILL.md` **(新規)** — `/board:docs` スキル
- `docs/api-reference.md` **(編集)** — サンプル JSON / エラー / Ransack 表
- `README.md` / `README_ja.md` **(編集)** — 補完 / docs / LLM 連携セクション
- `CHANGELOG.md` **(編集)** — v0.6.0 エントリ

### Phase N
- `plans/board-phase-n-m01-find-rationale.md` **(新規)** — 調査レポート
- `docs/adr/ADR-001-find-layer.md` **(新規)** — 意思決定記録
- `docs/specs/board_cli_mcp_ultra_detailed_design_ja.md` **(編集)** — 関連節補記
- `internal/service/find/*` **(N02 以降で変更)**
- `internal/cli/find_*.go` **(N02 以降で変更)**
- `internal/mcpserver/tools.go` **(N02 以降で変更)**

## 検証方法（Verification）

### 承認後の即時検証
- 本プランの各 M で定義した Verification を逐次実行
- Phase M 完了時に `go test ./... -count=1 && go vet ./...` + 手動 completion 動作確認

### リリース検証
- v0.6.0: `brew upgrade board` で入手でき、`board --version` / `board completion zsh` / `board docs --list` が全部動く
- v0.7.0: ADR に従った実装が E2E テスト全 Green（vendor/group が 0 件の環境でも SKIP の理由がログされる）

## Next Action

1. **本プランのレビュー + 承認**（ExitPlanMode で提示）
2. 承認後、以下を Implementer Agent に委譲:
   - `plans/board-phase-m-roadmap.md` を本ファイルの Phase M セクションから作成
   - `plans/board-phase-n-roadmap.md` を本ファイルの Phase N セクションから作成
   - `plans/board-phase-m-m58-completion.md` を M58 詳細セクションから作成
   - `plans/board-phase-m-m59-docs-command.md` を M59 詳細セクションから作成
   - `plans/board-phase-m-m60-docs-skill.md` を M60 詳細セクションから作成
   - `plans/board-phase-m-m61-readme-release.md` を M61 詳細セクションから作成
   - `plans/board-phase-n-m01-find-rationale.md` を N01 詳細セクションから作成
3. ドキュメント分割完了後、`/devflow:implement` で M58 から順次実装開始

---

**親計画**: なし（Phase L に続く次フェーズ）
**先行フェーズ**: `plans/board-phase-l-roadmap.md`（Phase L: api 層 BOARD API 完全準拠、v0.5.0 完走）
**作成日**: 2026-04-24
**最終更新**: 2026-04-24
**ステータス**: **計画中（ExitPlanMode で承認待ち）**
