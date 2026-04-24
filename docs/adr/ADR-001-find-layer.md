---
status: "Proposed"
date: 2026-04-24
deciders: [youyo]
consulted: ["調査レポート plans/board-phase-n-m01-find-rationale.md"]
informed: []
---

# ADR-001: find 層の扱い方針

## Context and Problem Statement

- Phase L（v0.5.0）で api 層 22 リソース Ransack 準拠完了。BOARD API のほぼ全フィルタを直接表現できる能力を獲得した。
- find 高レベル層（`internal/service/find/`、12 メソッド、内部実装約 3,945 行、47 E2E セット 193 ケース、SKIP 70 件）は Phase H 期の設計のまま未更新。
- TODO(M25-M32) 由来の enrichment / post-filter 未復元が 4 ドキュメント系（Estimate/Order/Delivery/Receipt）に 8 箇所残存。
- `internal/mcpserver/tools.go` の 12 tool が全て find_* 経由。api 層を直接呼ぶ MCP tool は現状ゼロ。
- ユーザー判断: 「全廃棄+ゼロベース前提で問い直したい」+ フラット評価要求 → D（現状維持 + 負債解消）を対抗馬として追加。
- **判断材料の限界**: 本 ADR は Explore 段階の一次情報（コードベース調査）+ 仕様ベース分析に基づく。
  実 API 検証（PoC）は未実施。最終確定は N02 冒頭の「実装前レビュー」で再確認する段階的合意とする。

## Decision Drivers

1. **UX（LLM）**: MCP クライアント（LLM エージェント）側の複雑性。逆引き・enrichment・free-text OR を LLM 側で実装するか find 層が担うか。
2. **実装コスト**: 工数および新規バグ混入リスク。N02+ で実装するフェーズのスコープに直接影響する。
3. **技術的負債解消**: TODO 8 箇所 / SKIP 70 件 / 3,945 行の処理方針。
4. **MCP LLM 親和性**: tool シグネチャの明快さ。名前で検索できるか、enriched 結果が返るかが LLM の効率に影響する。
5. **v0.7.0 以降の拡張性**: api 層に新リソース追加時の find 層追従コスト。
6. **v0.7.0 リリース時期影響**: 工数推定からのリリーススケジュールへの影響。

## Considered Options

### A: 全廃止

find 層（`internal/service/find/`）を全廃止し、MCP は api_* 直呼びに切り替える。
逆引き・enrichment・free-text OR はクライアント側（LLM または MCP server 内 helper）で実装する。

**詳細内容**:
- `internal/service/find/` の全ファイル（実装 12 ファイル + テスト）を削除
- `internal/mcpserver/tools.go` の全 12 tool Handler を書き換え（方針(a)/(b) で範囲は変動）
- E2E テスト 193 ケース（SKIP 70 件含む）を削除
- 推定工数: 3-5 日

**Pros**:
- find 層関連 3,945 行が削除され、保守対象が縮減する
- TODO 8 箇所・SKIP 70 件が消滅する
- api 層との責任境界が明確化する（api 層だけがデータアクセス層）

**Cons**:
- MCP 方針（a)/(b)/(c) の選択が別途必要（方針(a) 採択時は LLM 側実装負担が増加する）
- 逆引き・enrichment・free-text OR を呼び出し側で毎回記述する必要が生じる
- MCP tool のシグネチャが変わるため MCP クライアント（LLM エージェント）の動作変更が必要

### B: ゼロベース再設計

find 層を api 層で実現困難な 5 件（§4 参照）に特化した薄い層として再構築する。
既存の find 層は破棄し、新しい設計書から実装し直す。

**詳細内容**:
- 既存 `internal/service/find/` を廃止し新設計で再構築
- 5 件特化（リソース横断逆引き / free-text OR / enrichment / response_group 組み合わせ / 複数 status post-filter）
- MCP tools.go を新 find 層に合わせて更新（tool 数・引数が変わる可能性あり）
- E2E テスト再構築
- 推定工数: 2-3 週

**Pros**:
- 不要なメソッドを削除し、付加価値が明確な処理だけを提供する
- api 層との責任分界が明確化する
- 既存の TODO/SKIP 負債を引き継がない

**Cons**:
- 実装コストが 4 選択肢の中で最大（2-3 週）
- v0.7.0 に間に合わない可能性が高い（v0.8.0 送り前提）
- 新設計書策定に要する時間（N02 での策定工数）が追加発生する
- 新規実装に伴う新たなバグ混入リスクが発生する

### C: 数本残す

12 メソッドから AND 条件（api 単独代替不可 AND MCP 実利用頻度上位 AND TODO 負債が小さい）を満たす
3-5 本を選別して残し、他は廃止する。

**詳細内容**:
- §9 の AND 条件と重要度スコアに基づき 3-5 本を選別
- 選別外 7-9 本の find メソッドを削除
- 対応する MCP tool を削除または書き換え
- 残したメソッドの TODO は解消（または解消を条件として選別）
- 推定工数: 1-2 週

**Pros**:
- 付加価値のないメソッドを削除し、残存部分の保守対象を縮減する
- v0.7.0 に間に合う可能性がある（1-2 週）
- 段階的な廃止により影響範囲を制御できる

**Cons**:
- 選定基準の客観性担保が必要（「なぜそのメソッドを残したか」の説明責任）
- 一部残存により find 層という概念の一貫性が失われる
- 残存メソッドは依然として api 層追加時の追従コストを発生させる

### D: 現状維持 + 負債解消

find 層の構造は維持し、TODO(M25-M32) 8 箇所の enrichment / post-filter 復元と
E2E SKIP の仕分けのみを実施する。新規削除・再設計は行わない。

**詳細内容**:
- `find_estimate.go` / `find_order.go` / `find_delivery.go` / `find_receipt.go` の TODO 8 箇所を解消
- E2E SKIP 70 件のうち環境依存 42 件を「テストデータ不足」として別枠で管理
- MCP tools.go は変更なし（12 tool シグネチャ据え置き）
- 推定工数: 2-3 日

**Pros**:
- 既存 MCP UX（12 tool、逆引き・enrichment あり）を温存する
- 工数が最小（2-3 日）で v0.7.0 に確実に間に合う
- TODO 解消により現行の技術的負債を削減できる

**Cons**:
- find 層の本質的な存在意義問題（api 層成熟後の必要性）が先送りになる
- api 層に新リソースが追加された際の find 層追従コストが継続する
- 3,945 行の保守対象が引き続き残存する

## Decision Outcome

（Placeholder — Step 7b のユーザーレビュー後に A / B / C / D のいずれかを採用し、理由を記述する）

**段階的合意の原則**: 本 N01 では「方向性の合意」まで。最終確定は N02 冒頭の「実装前レビュー」で再確認する。

**Status 遷移**:
- `Proposed → Accepted`: Step 7b でユーザーが選択し、実装を開始する場合
- `Proposed → Deferred`: 追加情報（PoC 等）が必要な場合、または意思決定を延期する場合
- `Proposed → Rejected`: 本 ADR の枠組み自体を見直す場合（ADR-002 で上書き）

## Consequences

### Positive / Negative / Neutral

（選択結果に応じて記述する。Step 7b のユーザー選択後に確定）

### 再評価トリガ（後悔保険）

以下のいずれかに該当した場合、ADR-002 で本決定を再評価する:

- 実装着手から 3 マイルストーン完了時点で find 層呼び出し実績が想定の 50% 以下
- 選択した方針で v0.7.0 リリースが予定から 4 週間以上遅延
- LLM 側の MCP 利用パターンが想定と大きく乖離（例: LLM が find_* を呼ばず api_* を直接呼ぶ頻度が 80% 以上）

### N02+ で実施すべき事項

（選択結果に応じた次マイルストーンの骨子を箇条書き — Step 7b 後に確定）

- A 採択: N02 = find 層削除 + MCP 方針(a)/(b)/(c) 選択と実装
- B 採択: N02 = 新 find 層仕様策定（N03-N07 = 5 機能実装、N08 = MCP tools 刷新、N09 = E2E 再構築）
- C 採択: N02 = 3-5 本選別確定 + 削除対象の廃止（N03-N04 = 残存 TODO 解消、N05 = MCP tools 整理）
- D 採択: N02 = TODO(M25-M32) 8 箇所解消（N03 = E2E SKIP 仕分け + テストデータ整備）

## References

- 調査レポート: [plans/board-phase-n-m01-find-rationale.md](../../plans/board-phase-n-m01-find-rationale.md)
- 計画書: [plans/pure-twirling-coral.md](../../plans/pure-twirling-coral.md)
- Phase H アーカイブ: plans/archive/board-m29～m33-*.md
- 仕様書: [docs/specs/board_cli_mcp_ultra_detailed_design_ja.md §7.9 / §8.5 / §22](../specs/board_cli_mcp_ultra_detailed_design_ja.md)
- MADR 3.x 仕様: https://adr.github.io/madr/
