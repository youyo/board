---
title: "N06: ADR-001 再評価トリガチェックポイント記録（Deferred）"
project: "board"
phase: "Phase N"
milestone: "N06"
created: "2026-04-26"
status: "Deferred to N08+"
related_adr: "docs/adr/ADR-001-find-layer.md"
parent_plan: "plans/board-phase-n-m06-documents-reversemapper.md"
---

# N06: ADR-001 再評価トリガチェックポイント記録（Deferred）

## Context

ADR-001 §9.1 で定義された再評価トリガ (i)「実装着手から 3 マイルストーン完了時点で
find 層呼び出し実績が想定の 50% 以下」の N06 完走時チェックポイントである。

N02 設計書 (`plans/wondrous-skipping-snowglobe.md` §9.1)「N06 完了時点のチェックポイント
（C3: ADR-001 再評価トリガ監視）」より:

- [ ] MCP server の tool_call ログ集計（前 2 週間分、find_* / api_* 呼び出し比率）
- [ ] 想定比率（find_*:api_* = 60:40 目安）との乖離確認
- [ ] 50% 以下なら **ADR-002 起票を検討**（本計画を継続するか再評価）

## Status: Deferred to N08 + 2 weeks

**理由**:
- 本 N06 時点で `internal/service/find2/` は CLI/MCP に未配線（N07c で CLI、N08 で MCP 切替予定）
- 旧 `internal/service/find/` 経由の tool_call ログは新サービス（find2）の利用実績ではないため、
  ADR-001 トリガの意図する「新 find 層への需要」を測ることができない
- 集計可能になるのは N08（MCP tools 刷新）完了後 + 2 週間の運用ログ蓄積後

## 集計手順（N08 完了 2 週間後に実施）

1. MCP server 稼働ログ（または `[BOARD] tool_call` 形式の slog 出力）を前 2 週間分取得
2. tool 名で grouping 集計:
   - `find_*` カテゴリ: find_client / find_project / find_estimate / find_order / find_delivery / find_receipt /
     find_invoice / find_vendor / find_purchase_order / find_payment / find_user（11 tool）
   - `api_*` カテゴリ: api_clients_list / api_projects_get 等（22 リソース × 操作）
3. 想定比率との乖離計算:
   - 想定: find_*:api_* = 60:40
   - 実測: 計測値
   - 乖離: |実測 find_* 比率 - 60%|
4. 判定:
   - 乖離 ≤ 10% → 想定通り → 継続
   - 乖離 10-30% → 注意（ログ・要因分析）
   - 乖離 > 30% (find_* < 30% 等) → **ADR-002 起票検討**（本計画を継続するか再評価）

## 実施タイミング

- N08 完了直後: 2026年 N08 リリース日
- 2 週間後: 計測実施（最低 2 週間の運用ログが必要）
- 結果記録: 本ファイルに追記、または別 plan ファイルに昇格

## 関連 ADR / 計画

- ADR: `docs/adr/ADR-001-find-layer.md`
- 親計画: `plans/board-phase-n-m06-documents-reversemapper.md`
- N02 設計書: `plans/wondrous-skipping-snowglobe.md` §9.1
- 後続: N08（MCP tools 刷新）

## Changelog

| 日時 | 内容 |
|---|---|
| 2026-04-26 | N06 完走時に作成（Deferred 判断、N08+2 週間後に再開予定） |
