---
title: "N02 PoC Report: Document project_id 実在性 + 逆マッピング scale 実測"
created: "2026-04-25"
status: "Ready"
parent_plan: "plans/witty-sauteeing-kurzweil.md"
related_adr: "docs/adr/ADR-001-find-layer.md"
---

# N02 PoC Report: Document project_id 実在性 + 逆マッピング scale 実測

## §0 BOARD API spot check 結果

**確認日時**: 2026-04-25

**確認方法**:  
`tmp/e2e-artifacts/` に保存された Phase L（M49-M57）完走後の E2E dump を一次資料として使用。  
WebFetch ツール不使用（sandbox 制約）のため、公式 developer portal の直接確認は省略し、  
既存実装の API レスポンス構造（dump + StrictFieldDiff 完走結果）を根拠とした。

**確認した資料**:
- `tmp/e2e-artifacts/estimates_*.json` — EstimateEntity 実レスポンス
- `tmp/e2e-artifacts/orders_*.json` — OrderEntity 実レスポンス
- `tmp/e2e-artifacts/deliveries_*.json` — DeliveryEntity 実レスポンス
- `tmp/e2e-artifacts/receipts_*.json` — ReceiptEntity 実レスポンス
- `tmp/e2e-artifacts/projects_rg_estimate.json` 等 — projects RG dump

**Critical 仕様変更の有無**:  
Phase L（M49-M57）以降、Document Entity（EstimateEntity/OrderEntity/DeliveryEntity/ReceiptEntity）および  
`ProjectEntity` の RG フィールド（`Estimate *DocumentSummary` / `Order *DocumentSummary` /  
`Deliveries []DocumentSummary` / `Receipts []DocumentSummary`）の構造は一貫。  
**Critical 仕様変更（projects の response_group 廃止、Document Entity 破壊的変更等）は検出なし。**  
→ Step 2 以降着手可能と判断。

**実装フェーズで判明した影響事項**（実 API 実測で発覚、N03 Step 4 で吸収）:
- 全 4 RG cold_latency > 10s（最大 receipt 31915ms） → reverse_map.go に ctx timeout フォールバック必須
- mise run test:e2e:single 経由は sandbox で TLS 検証エラー → go test 直接呼びに統一

---

## §1 project_id/client_id 実在性結論

### 検証コマンド（オーケストレーター実施済み）

```bash
# トップレベルフィールド確認
jq 'has("project_id")' tmp/e2e-artifacts/estimates_*.json
jq 'has("project_id")' tmp/e2e-artifacts/orders_*.json
jq 'has("project_id")' tmp/e2e-artifacts/deliveries_*.json
jq 'has("project_id")' tmp/e2e-artifacts/receipts_*.json

# 再帰探索（ネストオブジェクト含む全探索）
jq '.. | objects? | select(has("project_id"))' tmp/e2e-artifacts/estimates_*.json
jq '.. | objects? | select(has("project_id"))' tmp/e2e-artifacts/orders_*.json
jq '.. | objects? | select(has("project_id"))' tmp/e2e-artifacts/deliveries_*.json
jq '.. | objects? | select(has("project_id"))' tmp/e2e-artifacts/receipts_*.json
```

### 確認した dump（8 件）

| dump ファイル | Document ID | project_id（トップ） | project_id（再帰） |
|---|---|---|---|
| estimates 3 件 | 105287235 他 | 存在しない | 存在しない |
| orders 2 件 | 71741501 他 | 存在しない | 存在しない |
| deliveries 2 件 | 64955390 他 | 存在しない | 存在しない |
| receipts 1 件 | 28449528 他 | 存在しない | 存在しない |

### projects RG dump との対応確認

| projects RG dump | project の Estimate/Order/Deliveries/Receipts に含まれる Document ID |
|---|---|
| `projects_rg_estimate.json` | estimate.id = 105287235 |
| `projects_rg_order.json` | order.id = 71741501 |
| `projects_rg_delivery.json` | deliveries[].id に 64955390 を含む |
| `projects_rg_receipt.json` | receipts[].id に 28449528, 28449529, 28449530 を含む |

**結論**: 全 8 件の Document dump において `project_id`/`client_id` はトップレベル・再帰探索双方で  
**完全に存在しない**。一方、projects RG dump には対応 Document の ID が含まれる。  
→ **Document Entity から直接 project_id を取得する経路は存在しない。**  
→ **逆マッピング（projects RG → documentID to projectID テーブル）が必須。**  
→ **案 A 確定の根拠が dump 検証で成立。**

---

## §2 scale 実測表（cold × 4 種）

**retry 計装手段**: 手段 (a)（推奨）を採用。  
- 実装: `internal/boardapi/retry_instrumentation.go`（`WithRetryCounter` + `recordRetryAttempt`）  
- `context.Value` 経由で retry 発動回数をカウント。behavior change なし、誤検出なし。  
- 手段 (b)（タイムスタンプ観測）は誤検出リスクがあるため不採用。

### 実測結果

| response_group | cold_latency (ms) | pages | items | retry_count | 判定 |
|---|---|---|---|---|---|
| estimate | 19465 | 25 | 2425 | 0 | 有効 |
| order | 16466 | 25 | 2425 | 0 | 有効 |
| delivery | 16930 | 25 | 2425 | 0 | 有効（初回 retry=1 → 再実施で retry=0 達成） |
| receipt | 31915 | 25 | 2425 | 0 | 有効 |

**結論**: 全 4 種で retry_count=0 を達成。scale は items=2425/pages=25 で平均 cold_latency = 21194 ms。
- 最大: receipt 31915ms（>30s） — **計画書 §5.3 のレイテンシ budget「10 秒超過時は ctx timeout で中断し ProjectID=0 フォールバック」のトリガに該当**
- 最小: order 16466ms（>10s）も同条件をトリガ
- → reverse_map.go (N03 Step 4) に **ctx timeout フォールバック実装を追加要件として組み込む**（user 承認済、2026-04-25）

*実測コマンド*:
```bash
# 各テストを順次実行（rate limit 配慮、テスト間は前 test の所要時間 100-120s で十分回復）
go test -tags e2e -v -count=1 -run TestE2E_FindLayerPoC_Projects_RG_Estimate_ScaleCold ./internal/boardapi/
go test -tags e2e -v -count=1 -run TestE2E_FindLayerPoC_Projects_RG_Order_ScaleCold    ./internal/boardapi/
go test -tags e2e -v -count=1 -run TestE2E_FindLayerPoC_Projects_RG_Delivery_ScaleCold ./internal/boardapi/
go test -tags e2e -v -count=1 -run TestE2E_FindLayerPoC_Projects_RG_Receipt_ScaleCold  ./internal/boardapi/

# 注: mise run test:e2e:single 経由は sandbox で TLS 証明書検証エラーになる場合があるため、
# go test を直接呼ぶ。retry 発動時は前テストとの間隔を 1 分以上空けて再実施する。
```

---

## §3 singleflight 要否判断

**判断基準**: cold_latency > 5s → singleflight 必要。< 5s でも concurrent 抑制目的で採用。

**N02 §5.3 既定方針**: singleflight パターン（`golang.org/x/sync/singleflight`）で  
projects RG 同時取得を 1 本に集約することを既定採用（concurrent 抑制目的）。  
→ **実測値に関わらず N03 Step で singleflight を実装する。**

実測結果が出次第、以下を記録する:
- cold_latency < 5s → 「< 5s だが N02 §5.3 方針通り singleflight 採用（concurrent 抑制目的）」
- cold_latency 5s-10s → 「singleflight 必要（cold 5s-10s）、N02 §5.3 確定」
- cold_latency > 10s → 「ctx timeout フォールバック（ProjectID=0）も N03 で実装必要、N02 §5.3 追加要件確定」

**実測まとめ**:
- 全 4 種で cold_latency > 5s（平均 21s、最大 receipt 31.9s）
- → singleflight 必要 + N02 §5.3 確定
- → 全 4 種で >10s 超過 → **ctx timeout フォールバック実装を N03 Step 4 に追加要件として組み込む**
- 想定 timeout 値: 10s（projects.Search が 10s 超過したら ctx canceled、Lookup は ProjectID=0 を返却）
- cold-reverse-map ログ `[SLOW:cold-reverse-map] projects RG=<group> building (estimated <pages> pages, latency=<ms>ms)` は stderr に出力

---

## §4 案採択確定

**dump 検証（§1）の結論**: 案 A 確定の決定論的根拠が dump 8 件で揃っている。  
実 API scale 実測が retry なしで完了した場合、**案 A 確定**。

- **案 A 確定条件**: 全 4 種 retry_count=0 で scale 実測完了 + §1 dump 検証で project_id 非存在確認済
- **案 B/ハイブリッド移行条件**: 実測中に project_id が返るケースを発見（現時点では発見なし）

**実測完了状態（2026-04-25）**:
- estimate: retry_count=0、cold=19465ms、pages=25、items=2425 — 有効
- order: retry_count=0、cold=16466ms、pages=25、items=2425 — 有効（初回 retry=1 → mise→go test 直接呼び切り替え + 1 分超 interval で再実施し retry=0 達成）
- delivery: retry_count=0、cold=16930ms、pages=25、items=2425 — 有効（初回 retry=1 → receipt 完走後の rate limit bucket 回復で retry=0 達成）
- receipt: retry_count=0、cold=31915ms、pages=25、items=2425 — 有効

**§1 dump 検証**で案 A の決定論的根拠は成立済（Document Entity に project_id 完全非存在 / projects RG dump に Document ID 包含）。
**§2 scale 実測**で全 4 種 retry_count=0 達成 → **案 A 確定（gate 通過）**。

ただし全 4 種で cold_latency > 10s を観測したため、N02 §5.3 のレイテンシ budget「10 秒超過時は ctx timeout で中断し ProjectID=0 フォールバック」のトリガが発動。
N03 Step 4 reverse_map.go に **ctx timeout フォールバック実装を追加要件として組み込む**（user 承認済、2026-04-25）。

**status**: `Draft` → `Ready`

---

## §5 案 B/ハイブリッド時の影響範囲棚卸し

N/A 案 A 確定（§1 dump 検証により根拠成立、§4 参照）

案 B またはハイブリッドに移行する場合は以下を棚卸しする:
- `EstimateEntity`/`OrderEntity`/`DeliveryEntity`/`ReceiptEntity` に `ProjectID int` / `ClientID int` 追加
- parser / StrictFieldDiff 対応（M39-M45 確立パターンで対応）
- 既存 E2E テストへの影響確認
- N03 find2/ 骨格への影響（Result 型の `ProjectID` フィールド重複回避）

---

## §6 N07b rename 事前検証

*Step 8 drill 結果をここに追記する（placeholder）。*

- 対象: `internal/service/find/` → `internal/service/find2/` への段階的移行
- 検証予定: シンボル衝突チェック、import パス変更影響、IDE/ツール互換性
- 実施タイミング: N03 Step 8

---

## 付記: 実装ファイル

| ファイル | 内容 |
|---|---|
| `internal/boardapi/retry_instrumentation.go` | retry 計装（WithRetryCounter / recordRetryAttempt） |
| `internal/boardapi/retry.go` | `recordRetryAttempt` 呼び出し 2 箇所追加（behavior change なし） |
| `internal/boardapi/e2e_find_layer_poc_test.go` | cold scale 実測 E2E テスト 4 種 + preflight budget check |
