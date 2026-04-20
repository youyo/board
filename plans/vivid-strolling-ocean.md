# Plan: BOARD API 準拠検証 & E2E 網羅ロードマップ作成

## Context
- **前提**: 22 リソースのうち boardapi E2E が通っているのは 10、service/find E2E は 5 パターンのみ。12 リソースが未カバー、さらに既検証リソースでも Search/Get を素通ししているものが多い。
- **リスク**: 直近の `271cba3` で UserEntity/ContactEntity/VendorContactEntity に 6 フィールドもの実 API 不整合が見つかっており、残り 19 Entity も同様のずれを抱えている可能性が極めて高い。E2E 未通過のうちに LLM/MCP 経由で使われると静かにデータが欠落する。
- **制約**: BOARD API は 3000 req/日・3 req/秒。本番業務には未使用のため上限の 50% 弱まで使えるが、手動 1 request 確認の粒度で進めるため**数日〜1 週間**を要する見込み。
- **目的**: 全 22 リソース × (List/Get/Search) × (boardapi/find) を網羅する E2E を、**中断耐性の高い細粒度マイルストーン**で整備し、全 Entity の **フィールド厳格突合** で準拠性を担保する。

## インタビュー確定事項
| 項目 | 決定 |
|------|------|
| スコープ | boardapi + service/find の E2E（MCP は対象外） |
| 開始順序 | マスタ系先行（低コスト・失敗リスク小） → コア → ベンダー → ドキュメント → find 層 |
| 検証厳格度 | **厳格: 全フィールド突合**（JSON 未マップキーをゼロに） |
| 中断方針 | **403/429 即停止 + 人間判断**（skip せず失敗として上げる） |
| API 日次予算 | **目安 1500 req/日**（上限の 50%、業務未使用のため余裕あり。必要時は後日調整可） |
| 生 JSON 保存先 | `tmp/e2e-artifacts/` （リポジトリ root 直下、.gitignore に追加、**絶対に commit しない**） |
| orders/deliveries/receipts | `projects` を response_group 付きで走査し documentID を発見 |

## 成果物（このプランが承認されたら作成）

### ロードマップファイル
- `plans/board-compliance-roadmap.md`（Layer 1：本ロードマップ）
- `plans/board-compliance-m01-{slug}.md`（Layer 2：M01 のみ先行詳細化。M02 以降は着手時に生成）

### コード変更は行わない（ロードマップ生成のみ）
実装は別途 `/devflow:implement` で 1 マイルストーンずつ進める。

## Rate Limit 対応方針（全 M 共通ルール）

各マイルストーン詳細計画に以下を記載する:

1. **事前 API コール数見積**: 実行前にテスト 1 本あたりの request 数を算出（pagination の要否を含む）。
2. **キャッシュ優先**: 最初の 1 リソースのみ `ForceRefresh: true`、以降は `ReadOptions{}` で cache 優先。
3. **1 test 1 endpoint**: `go test -run TestE2E_XXX -count=1` で個別実行し、実際の request 数を確認してから次へ。
4. **失敗時即停止**: 403/429/Entity field mismatch を検出したら当該 M を中断し、原因をロードマップ `Blockers` に記録して人間判断を仰ぐ。
5. **進捗を roadmap に毎 M 反映**: Current Focus と Progress のチェックボックスを commit 単位で更新。

## フィールド厳格突合の方式

1. テスト内で `json.Unmarshal(raw, &map[string]any{})` と `json.Unmarshal(raw, &Entity{})` を並行実行。
2. raw JSON のキー集合と Entity の `json:"..."` タグ集合を比較。
3. raw にあって Entity にないキー → **test Fail**（未マップ検知）。
4. 再マーシャル可能性を確認し、Entity 往復で値欠落がないことを保証。
5. 生 JSON を `tmp/e2e-artifacts/{resource}_{id}.json` に保存してレビュー可能にする。
6. この突合ロジックは共通ヘルパー `testhelper.StrictFieldDiff(t, raw, entity)` として M01 で提供。

## マイルストーン全体像（ドラフト：roadmap 作成時に最終化）

| Phase | # | 目的 | 主要成果 | 見積 req |
|------|---|------|---------|---------|
| **A: 基盤** | M01 | 厳格突合ヘルパー & tmp/ 整備 | testhelper, .gitignore 更新 | 0 |
| **B: マスタ系（小）** | M02 | accounting_types 完走 | L/G/S + 厳格突合 | ~5 |
| | M03 | project_types 完走 | 同上 | ~5 |
| | M04 | payment_terms 完走 | 同上 | ~5 |
| | M05 | document_send_channels 完走 | 同上 | ~5 |
| | M06 | purchase_types Search/Get 追加 | 既存 List + 追補 | ~3 |
| | M07 | groups Get 検証 + 厳格突合 | 既存 List + Get | ~3 |
| **C: マスタ系（中）** | M08 | users Get/Search 厳格突合 | 既存 List/Get + S | ~5 |
| **D: コア業務（未カバー）** | M09 | client_branches 完走 | L/G/S + 厳格突合 | ~8 |
| | M10 | contacts 完走（19 フィールド） | 同上 | ~10 |
| | M11 | project_costs 完走 | 同上 | ~8 |
| **E: コア業務（再検証）** | M12 | clients 厳格突合（既存 L/G/S 再走査） | 既存 + 厳格 | ~5 |
| | M13 | projects 厳格突合 + GetWithGroup 全 response_group | 既存 + 拡張 | ~15 |
| **F: ベンダー系** | M14 | vendor_branches 完走（payee_branches 実パス確認） | L/G/S + 厳格 | ~8 |
| | M15 | vendor_contacts 完走（payee_contacts 実パス確認） | 同上 | ~8 |
| | M16 | vendors Get/Search 追加 | 既存 List + G/S | ~5 |
| **G: ドキュメント系（documentID 発見フロー確立）** | M17 | documentID discovery helper（estimates で再利用） | helper fn | ~5 |
| | M18 | estimates Get 厳格突合（既存を強化） | 既存 + 厳格 | ~5 |
| | M19 | orders Get（documentID 発見→GET）+ 厳格 | 新 E2E | ~10 |
| | M20 | deliveries Get 同上 | 新 E2E | ~10 |
| | M21 | receipts Get 同上 | 新 E2E | ~10 |
| | M22 | invoices Get/Search 厳格突合（既存 List + 強化） | 既存 + G/S | ~20 |
| | M23 | purchase_orders Get/Search 追加 | 既存 List + G/S | ~10 |
| | M24 | payments Get/Search 追加 | 既存 List + G/S | ~10 |
| **H: service/find 層** | M25 | FindClient 厳格化 + 全 ClientResult フィールド確認 | 既存 + 強化 | ~5 |
| | M26 | FindProject ClientID/WithEstimate 全パス | 既存 + 拡張 | ~10 |
| | M27 | FindOrder 新規 E2E（FindEstimate と対比） | 新 E2E | ~10 |
| | M28 | FindDelivery 新規 E2E | 新 E2E | ~10 |
| | M29 | FindReceipt 新規 E2E | 新 E2E | ~10 |
| | M30 | FindVendor/FindPurchaseOrder/FindPayment 新規 E2E | 新 E2E | ~15 |
| | M31 | FindUser/FindGroup 厳格突合 | 既存 + 厳格 | ~5 |
| | M32 | FindInvoice E2E（キャッシュ前提、軽量版） | 新 E2E | ~5 |
| **I: 仕上げ** | M33 | 全 E2E 通しスモーク（キャッシュ有効前提） | CI 相当確認 | ~50 |
| | M34 | docs/specs 反映・CLAUDE.md 更新・memory 反映 | ドキュメント | 0 |

**合計見積**: ~280 req（キャッシュ効く想定）／バッファ込み ~500 req／失敗再試行込みでも 1500 req 以下。
**所要日数**: 1 日 2〜4 M 消化で **8〜14 日**。中断しても M 単位で独立再開可能。

## M01 詳細計画（先行生成予定の内容）

### Overview
- 対象ファイル:
  - `.gitignore`（`tmp/` 行追加）
  - `internal/testhelper/strict_field_diff.go`（新規）
  - `internal/testhelper/strict_field_diff_test.go`（新規、unit）
  - `internal/boardapi/e2e_helpers_test.go`（`dumpJSON`, `documentIDFinder` 追加）
  - `mise.toml`（`test:e2e:single` タスク）
- 依存: なし（後続全 M の前提）
- 目標 req 数: **0**（全て unit + fixture）

### TDD 設計
| # | テスト | 入力 | 期待 |
|---|--------|------|------|
| 1 | StrictFieldDiff は未マップキーを列挙 | JSON `{a,b,c}` vs struct {a,b} | `[c]` |
| 2 | 同値なら空リスト | struct 完全一致 | `[]` |
| 3 | ネスト struct も再帰 | nested に未マップ | nested.x 検知 |
| 4 | `json:"-"` は無視 | omit フィールド | 無視 |
| 5 | 配列要素の各 element 突合 | `[{a,b},{a,c}]` | c 検知 |

### Steps
- [ ] `.gitignore` に `tmp/` 追加
- [ ] `StrictFieldDiff` 関数 & unit test（Red → Green）
- [ ] `dumpJSON(t, resource, id, raw)` を e2e helper に追加（`tmp/e2e-artifacts/{resource}_{id}.json` へ書き出し）
- [ ] `mise test:e2e:single <pattern>` タスク定義
- [ ] M02 の雛形コメントを PR に含める（次 M の準備）

### Risks
| リスク | 影響 | 対策 |
|--------|------|------|
| reflect ベース突合で `interface{}` ネストを取りこぼす | 中 | unit テストで配列・map・ポインタをカバー |
| `tmp/` にシークレット（顧客名等）が残る | 高 | `.gitignore` 追加 + README に注意書き |

## 本プラン承認後の流れ

1. `plans/board-compliance-roadmap.md` を上記 34 M で生成。
2. `plans/board-compliance-m01-foundation.md` を先行詳細化。
3. `plans/board-roadmap.md` の Current Focus に「別軸の準拠検証ロードマップを併走」と追記（既存ロードマップは凍結しない）。
4. ユーザーへ `/devflow:implement M01` の実行を案内。
5. 各 M 完了時に roadmap の Progress 行をチェック、Changelog に「なぜその順序で進めたか」を記録。

## 検証方法（このプラン自体の）
- `plans/vivid-strolling-ocean.md`（本ファイル）をレビュー。
- 粒度・順序・予算に対する修正希望があれば **ファイル内に直接インライン注釈** してもらう（Annotation Cycle）。
- 注釈反映後に roadmap ファイル群を生成して `/devflow:implement` フェーズへ。
