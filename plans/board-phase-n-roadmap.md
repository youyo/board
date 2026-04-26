# Phase N ロードマップ: find 層必要性評価 → v0.7.0

## Meta
| 項目 | 値 |
|------|---|
| ゴール | `internal/service/find/`（12 Find メソッド、47 E2E テスト）の存在意義を api 層の成熟度を踏まえて再評価し、**廃止 / 再設計 / 一部残す** のいずれかを ADR で意思決定。決定内容に従って実装を刷新し v0.7.0 をリリース |
| 成功基準 | `docs/adr/ADR-001-find-layer.md` で意思決定記録 + 決定内容に基づく実装完了 + MCP 接続部も新方針に沿って刷新 + E2E テスト全 Green（実 API で感応するケースのみ） + v0.7.0 タグ配信 |
| 制約 | Phase M 完了（v0.6.0）後に着手、調査中は既存 find 層は現状維持（機能欠落期間を作らない）、実装方針は N01 の意思決定結果に依存 |
| 対象リポジトリ | /Users/youyo/src/github.com/youyo/board |
| 作成日 | 2026-04-24 |
| 最終更新 | 2026-04-25（N01 全 Step 完了、B 採択確定） |
| ステータス | N01 完了（B: ゼロベース再設計 採択）、N02 仕様策定へ移行 |
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
- **マイルストーン**: N07c（CLI 刷新: `board find <sub>` の各フラグ → ID 解決配線）— N07b 完走済
- **直近の完了**: N07b 完走（2026-04-27）。旧 `internal/service/find/` 全削除 + `find2/` → `find/` rename を 1 コミット完結で実施。**`cli/find_group.go` 削除**（ADR-001 Group 削除確定）。**MCP `find_groups` tool 削除**（12 → 11 tool、tools_test 期待値も更新）。**CLI Query 構築を `FindCommonOpts` 埋め込みに統一**、未対応フラグ（`--client-name` / `--project-name` / `--vendor-name` / `--purchase-order-id` / Document `--status`）は RunE で reject（N07c で配線予定）。MCP handler 側でも同じ args を error 返却。go test -race ./... pass / go vet pass / gofmt 差分なし
- **次のアクション**: N07c — CLI 刷新（`board find <sub>` の name → ID 解決を配線、N07b で reject していたフラグを動作させる）

## Progress

### N01: find 層必要性評価 + 設計方針案 + ADR
- [x] `plans/board-phase-n-m01-find-rationale.md` 雛形作成 + 本文（520 行、§1-§10 完成）
- [x] 観点 1-5 のデータ収集（grep / Read / 既存ドキュメント精読）
  - 観点 1: find 層の付加価値棚卸し（12 メソッド台帳）
  - 観点 2: api 層 Ransack での代替可能性（18 リソース × 主要フィルタ）
  - 観点 3: api 層の限界 5 件（find でしか困難な処理、§4 限界注記付き）
  - 観点 4: E2E SKIP 実態（70 件内訳 × 4 カテゴリ）
  - 観点 5: MCP 12 tool 現状 + 3 通り方針 (a)/(b)/(c) マトリクス
- [x] 「12 Find メソッド × 4 選択肢」の 48 セル大表作成（§8）
- [x] 選択肢 A/B/C/**D** の trade-off 分析（6 評価軸 × 4 列、弁証法レビューで D 追加）
  - A: 全廃止（MCP を api 層直接呼び出しに）
  - B: ゼロベース再設計（5 件特化層）
  - C: 数本に絞る（api 層で代替できないものだけ残す、AND 3 条件で選定）
  - D: 現状維持 + 負債解消（TODO(M25-M32) 8 箇所解消 + E2E SKIP 仕分け）
- [x] Step 7a: ユーザー事実確認レビュー（事実に誤りなし確認済み、2026-04-25）
- [x] Step 7b: 評価軸重み付け + A/B/C/D 選択 → **B 採択**（2026-04-25）
- [x] `docs/adr/ADR-001-find-layer.md` Placeholder 作成（MADR 3.x、Status: Proposed）
- [x] Step 8: ADR-001 Decision Outcome 確定（B 採択・Status: Accepted、2026-04-25）
- [x] `docs/specs/board_cli_mcp_ultra_detailed_design_ja.md` §7.9/§8.5/§22 に全選択肢共通 Placeholder 注記
- [x] Step 9: 仕様書最終化（B 採択に従って §7.9/§8.5/§22 の Placeholder 注記を更新、2026-04-25）
- 📄 詳細計画: `plans/pure-twirling-coral.md`（弁証法レビュー反映改訂版、Step 0-11 定義）
- 📝 groovy-churning-valley.md の N01 詳細セクションを参照

### N02+（N01 意思決定結果による分岐、概要のみ）

#### 選択肢 A（全廃止）になった場合
- N02: `internal/service/find/` 削除
- N03: `internal/cli/find_*.go` 削除
- N04: MCP tools（`internal/mcpserver/tools.go`）を api 層直接呼び出しに書き換え
- N05: v0.7.0 リリース（Breaking Change として CHANGELOG 強調）

#### 選択肢 B（ゼロベース再設計）採択（2026-04-25 確定）

詳細計画: `plans/wondrous-skipping-snowglobe.md`（N02 詳細設計書、弁証法レビュー反映済）

**N02 完了チェックリスト**:
- [x] 設計書初版作成（649 行、11 メソッド / 5 件特化 / N03-N10 マイルストーン定義）
- [x] Document Entity 対応（案 A + C: Entity 変更なし、Result 構造体に ProjectID 保持）
- [x] TDD テスト設計（Query→ListOptions 変換テスト例 / 正常・境界・異常系 / 逆マッピング unit test / Refactor 対象明示）
- [x] E2E 再構築方針（193 ケース削除 → 30-41 代表ケース、SKIP 統一テンプレート 4 種）
- [x] シーケンス図 2 種（FindProject 逆引き + enrichment 並列 / FindEstimate 逆マッピング）
- [x] 弁証法レビュー（devils-advocate 13 指摘 → advocate Must 4 / Should 6 採用）
- [x] Must 4 件反映（C1: N07 三分割 / C2: 逆マッピング scale 実測 / C3: ADR 再評価トリガ監視 / H6: Statuses[] 明文化）
- [x] 設計書ステータス → **Ready for Review**（2026-04-25）

- **N02**: 新仕様策定（Query / Result 型、FindXxx API 設計、PoC 含む）✅
- **N03**: Document PoC + `find2/` パッケージ骨格 + 共通ヘルパー — ✅ 完了（2026-04-25）
  - [x] Step 1: PoC test + retry instrumentation + PoC レポート（案 A 確定、4 種 retry=0、cold_latency >10s で ctx timeout フォールバック追加要件確定）
  - [x] Step 2: go.mod / go.sum に `golang.org/x/sync v0.20.0`
  - [x] Step 3: find2/service.go + types.go（11 Query+Result、Document 4 種は ProjectID/ClientID 追加、15 repo interface、Groups 除外）
  - [x] Step 4: text_match.go + filter.go + resolver.go + reverse_map.go（**ctx timeout 10s フォールバック実装済**、`context.DeadlineExceeded`/`Canceled` 検知時 `(0, false, nil)` + `slog.Warn`）
  - [x] Step 5: helpers_test + 各 helper unit test（T01-T24 + timeout フォールバック test、35 関数、`-race` pass）
  - [x] Step 6: app.go に `FindService2()` 暫定追加（15 repo 配線、Groups 除外）
  - [x] Step 7: mise.toml に `[tasks."test:race"]` タスク追加
  - [x] Step 8: N07b rename drill（実コード参照: find2.=2/import=1/FindService2=1、極めて低リスク確認、PoC レポート §6 追記）
- **N04**: FindClient + FindVendor 実装 — ✅ 完了（2026-04-25, commit 07e6643）。non-fatal enrichment / `validateQuery` 規約 / PayeeIDEq / handshake 並列検証を確立。code-reviewer APPROVED（Major 4 件は N05/N07c で吸収）
- **N05**: FindProject 実装 — ✅ 完了（2026-04-26, commits 6762420 / bd3218c / 7411161）。Status/Statuses-only クエリの validation reject 導入（API delegation 不可のため narrowing 必須）、OrderStatusName/DeliveryStatusName の OR 評価 post-filter、recordingHandler 追加。code-reviewer APPROVED
- **N06**: Document 4 種実装（Estimate/Order/Delivery/Receipt）+ ADR-001 再評価トリガチェックポイント — ✅ 完了（2026-04-26）。reverseMapper 初実用（ID branch 3 hop、cold timeout フォールバック）、二重 fetch 回避（lookupClient helper + p 再利用）、Delivery/Receipt 配列全件ループ。+58 unit tests、ADR トリガは N08+2 週間後に deferred
- **N07a**: FindInvoice/PurchaseOrder/Payment/User 実装 — ✅ 完了（2026-04-27）。4 find_*.go + 4 find_*_test.go 新規（49 unit test 関数、find2 累計 192 PASS）。**Status (single) は API delegation で allow / Statuses[] (multi) は narrowing 必須 reject**（D2、N05 と判別軸（API delegation 可否）が同じ非対称適用）、**`filterByStatuses[T]` ジェネリック初実用**、**Payment.Project = nil 固定**（D1、PaymentEntity に ProjectID 不在 + E2E dump 0 件）。code-reviewer は self-execute + advisor() で代替（N06 と同様の環境制約）
- **N07b**: 旧 `internal/service/find/` 削除 + `find2/` → `find/` rename（独立 revertable 境界）— ✅ 完了（2026-04-27）。47 E2E + 旧 unit 削除、find2 1728 LOC を find/ にリネーム。`cli/find_group.go` + MCP `find_groups` tool 削除（ADR-001 forced）、Query 構築を `FindCommonOpts` 埋め込みに統一。CLI/MCP の未対応フラグ（name → ID 解決、document `--status`）は N07b では reject、N07c で配線。go test -race ./... pass
- **N07c**: CLI 刷新（`board find_*` → `board find <sub>`）
- **N08**: MCP tools 刷新（12 → 11 tool、find_groups 削除）
- **N09**: E2E テスト再構築（33-41 代表ケース、実 API 感応のみ、SKIP 理由は明示ログ）
- **N10**: v0.7.0 リリース準備

**合計工数**: 25-34 日 ≒ 5-7 週（ADR-001 当初 2-3 週見積は下振れシナリオ、devils-advocate/advocate 修正後の実測寄り見積）

#### 選択肢 B 以外の参考（採択済みのため非活性）
~~A 全廃止 / C 数本残す / D 現状維持~~

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
| 2026-04-24 | 更新 | N01 Step 1-6 完了：調査レポート 520 行 / ADR-001 Placeholder / 仕様書 3 節 Placeholder。Step 7a（事実確認）待ちに遷移、選択肢に D（現状維持 + 負債解消）を追加して A/B/C/D 4 選択肢フラット評価に変更 |
| 2026-04-25 | 完了 | N01 完走（B 採択・ADR-001 Accepted）。N02 設計書（wondrous-skipping-snowglobe.md）作成 + 弁証法レビュー（devils-advocate → advocate）反映・Ready for Review |
| 2026-04-25 | 進行 | N03 計画書（witty-sauteeing-kurzweil.md）Ready for Review。Step 1 完了（PoC 4 種 retry=0、案 A 確定、PoC レポート Ready）+ Step 2 完了（go.mod に golang.org/x/sync v0.20.0）。Step 3-8 は次セッションへ |
| 2026-04-25 | 完了 | N03 全 8 Step 完走。find2/ パッケージ 12 ファイル新規作成、ctx timeout 10s フォールバック実装、35 unit test 関数（race pass）。app.go に `FindService2()` 暫定追加、mise.toml に `test:race` タスク追加、PoC レポート §6 に rename drill 結果記録（実コード参照: find2.=2/import=1/FindService2=1、低リスク）。code-reviewer 独立レビュー APPROVED（Major M1: T20/T21 ctx 伝播テストの空回り → ctx-aware stub に修正済）。次は N04（FindClient + FindVendor 具象実装） |
| 2026-04-25 | 完了 | N04 完走（commit 07e6643）。`FindClient` + `FindVendor` 具象実装、find2/ に 5 ファイル新規/更新（+983 LOC、29 unit test 関数追加）。**non-fatal enrichment ポリシー / `validateQuery(q.FindCommonOpts, q)` 規約 / PayeeIDEq による Vendor enrichment / handshake チャネル方式での並列検証** を確立（advisor 反映の N04 計画書に基づく）。`go test -race` pass、code-reviewer APPROVED（Major 4 件はいずれも N04 動作に非影響、N05/N07c で吸収予定）。次は N05（FindProject 実装） |
| 2026-04-26 | 完了 | N05 完走（commits 6762420 / bd3218c / 7411161）。`FindProject` 具象実装、+627 LOC / 22 unit test 関数追加。**Status/Statuses-only クエリの validation reject**（advisor R3 (a)、API delegation 不可のため narrowing 必須）、**`filterProjectsByStatuses`**（OrderStatusName/DeliveryStatusName OR 評価）、**`resolveProjectClient`**（単一 enrichment 逐次）、**`recordingHandler`**（slog.Warn 観測、N06+ で reuse）を確立。code-reviewer APPROVED（Critical/Major 0、Minor 4 件は将来注意点）。次は N06（Document 4 種、reverseMapper 初実用） |
| 2026-04-26 | 完了 | N06 完走（self-execute、Agent tool 環境制約）。`FindEstimate` / `FindOrder` / `FindDelivery` / `FindReceipt` 具象実装、+58 unit test 関数追加（find2 累計 143 関数）。**reverseMapper 初実用**（ID branch documentID → projectID 逆引き、cold > 10s フォールバック既実装）。**二重 fetch 回避**: `lookupClient` 新規 + 既存 `p` 再利用（advisor R1）、ClientName branch outer client 再利用（advisor R2）。**Delivery/Receipt 配列全件ループ**（N02 §4.4、旧 [0] 廃止）。Document Entity は Status/Statuses 不所持のため filter/validation reject なし。go test -race pass、go vet pass、gofmt 差分なし。ADR-001 N06 再評価トリガは MCP 未配線のため N08+2 週間後に deferred（plans/board-phase-n-m06-adr-trigger-review.md）。次は N07a（FindInvoice/PO/Payment/User） |
| 2026-04-27 | 完了 | N07a 完走（self-execute、Agent tool 環境制約）。`FindInvoice` / `FindPurchaseOrder` / `FindPayment` / `FindUser` 具象実装、4 find_*.go + 4 find_*_test.go 新規（49 unit test 関数追加、find2 累計 192 PASS）。**Status/Statuses 非対称 validation reject** を確立（D2、advisor 確認済）: `Status` 単独は `StatusEq` で API delegation 可のため allow、`Statuses[]` 単独は `StatusIn[]` 不在で full-scan 必須のため narrowing 強制 reject。判別軸（API delegation 可否）は N05 と同じだが reject 範囲が非対称。**`filterByStatuses[T]` ジェネリック初実用**（N06 までは OR 評価専用 `filterProjectsByStatuses` のみ）。**Payment.Project = nil 固定方針**（D1、PaymentEntity トップレベルに ProjectID 不在 + E2E dump 0 件で 3-hop 安全性検証不可、N09 E2E 再構築時に再検討）。**helpers_test.go stub 4 種拡張**（searchFunc/getFunc/searchCount/getCount を Invoice/PO/Payment/User に追加、N04 stubVendorRepo 同等）。go test -race ./... pass（全 14 パッケージ）、go vet pass、gofmt 差分なし。次は N07b（旧 find/ 削除 + find2/ → find/ rename） |
| 2026-04-27 | 完了 | N07b 完走（self-execute）。旧 `internal/service/find/` 全削除（47 E2E + 旧 unit）+ `find2/` → `find/` rename（package decl + slog warn 文字列 + 全テスト assertion）。`app.go` の `FindService2()` を `FindService()` に rename。**`cli/find_group.go` 削除** + **MCP `find_groups` tool 削除**（12 → 11 tool、ADR-001 Group 削除確定 forced）。**CLI Query 構築を `FindCommonOpts` 埋め込みに統一**、未対応フラグ（`--client-name`/`--project-name`/`--vendor-name`/`--purchase-order-id` および Document の `--status`）は RunE で reject。MCP tools.go handler でも同等 args reject。1 コミット完結 revertable。go test -race ./... pass（全パッケージ、find/ 191 unit、mcpserver/ 含む）、go vet pass。次は N07c（CLI name → ID 解決配線） |

## Next Action

1. **N07b 詳細計画作成**（`/devflow:plan` または `/devflow:cycle`）
   - 対象: 旧 `internal/service/find/` 削除 + `find2/` → `find/` rename（独立 revertable 境界）
   - 既知の影響範囲: N03 Step 8 rename drill 結果（実コード参照: `find2.`=2 / import=1 / `FindService2`=1）が極めて低リスクと既に確認済（PoC レポート §6 記録）
   - 既存 find/ パッケージは Phase H 産物で 12 メソッド（N02 で 11 メソッドに整理、Group 削除確定）。CLI/MCP は依然旧 find/ を経由 → N07b では service 層のみスイッチし、CLI/MCP は N07c/N08 で順次切替（1 コミット完結 revertable 維持）
   - app.go の `FindService()` を新サービスに差し替え、`FindService2()` 削除
2. **N07c 計画作成時の追加タスク**（N04 code-reviewer Major M1 + N07a D2 引継）
   - enrichment 失敗時の non-fatal セマンティクス変更を CLI/MCP/CHANGELOG/docs/api-reference に breaking change として告知
   - N07a の Statuses-only narrowing 必須エラーメッセージを CLI flag 説明に反映
3. ADR-001 N06 再評価トリガは N08 完了 + 2 週間後に実施（`plans/board-phase-n-m06-adr-trigger-review.md` に記録）
4. N07b → N07c → N08 → N09 → N10 を順次実装
