---
status: "Accepted"
date: 2026-04-25
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

**採択**: B — ゼロベース再設計（2026-04-25 確定）

**理由**:
- Phase L で api 層が成熟した今、find 層の本質的な付加価値は「api 層で実現困難な処理（逆引き・enrichment・free-text OR 等）を担う薄い抽象層」に絞られる。
- 既存の 12 メソッドは api 層で代替可能なものを多数含んでおり、負債（TODO 8 箇所・SKIP 70 件）を抱えたまま維持するよりゼロベースで再設計する方が長期コストを下げられる。
- D（現状維持）では存在意義問題が先送りになり追従コストが継続する。C（数本残す）では選定基準の説明責任と一貫性の喪失が生じる。A（全廃止）では MCP UX が後退し LLM エージェントの負担が増大する。

**段階的合意の原則**: 方向性を B に確定。新 find 層の具体的な API 設計（Query/Result 型・5 件特化内容）は N02 で確定する。

## Consequences

### Positive

- 不要なメソッドを削除し、付加価値が明確な処理（逆引き・enrichment・free-text OR・response_group 組み合わせ・複数 status post-filter）だけを提供する薄い層になる。
- 既存の TODO/SKIP 負債を引き継がない。
- api 層との責任分界が明確化する。

### Negative

- 実装コストが 4 選択肢中最大（推定 2-3 週）。
- v0.7.0 は v0.8.0 以降送りになる可能性が高い。
- 新設計書策定（N02）で追加工数が発生する。
- 新規実装に伴うバグ混入リスクが生じる。

### Neutral

- 既存 `internal/service/find/` は N03-N07 実装完了後に削除（移行期間中は並存）。
- MCP tool 数・シグネチャは N02 の設計で決定する。

### 再評価トリガ（後悔保険）

以下のいずれかに該当した場合、ADR-002 で本決定を再評価する:

- 実装着手から 3 マイルストーン完了時点で find 層呼び出し実績が想定の 50% 以下
- 選択した方針で v0.7.0 リリースが予定から 4 週間以上遅延
- LLM 側の MCP 利用パターンが想定と大きく乖離（例: LLM が find_* を呼ばず api_* を直接呼ぶ頻度が 80% 以上）

### N02+ で実施すべき事項

（選択結果に応じた次マイルストーンの骨子を箇条書き — Step 7b 後に確定）

- **B 採択（確定）**: N02 = 新 find 層仕様策定（Query/Result 型・5 件特化 API 設計）→ N03-N07 = リソース別実装（client / project / document / vendor / master）→ N08 = MCP tools 刷新 → N09 = E2E テスト再構築 → N10 = v0.7.0 リリース

## Implementation Status

- N03 完了（2026-04-25）: find2/ パッケージ骨格 + 共通ヘルパー + helpers_test
- N04 完了（2026-04-25）: FindClient + FindVendor 具象実装、non-fatal enrichment 規約確立
- N05 完了（2026-04-26）: FindProject 実装、Status/Statuses-only validation reject
- N06 完了（2026-04-26）: Document 4 種実装、reverseMapper 初実用
- N07a 完了（2026-04-27）: FindInvoice/PurchaseOrder/Payment/User 実装
- N07b 完了（2026-04-27）: 旧 find/ 削除 + find2/ → find/ rename、Group 削除確定
- **N07c 完了（2026-04-27）**: CLI/MCP の name → ID 解決配線（`ResolveClientByName` / `ResolveVendorByName`）、構造的未対応フラグの最終エラー文言確定、enrichment non-fatal の breaking change を CHANGELOG / api-reference に告知
- **N08 完了（2026-04-27）**: MCP tools schema 刷新（11 tool）。Document 4 種の `status` プロパティを構造的不可（D1, never-implementable）として schema 削除（primary defense として handler reject 残置）、契約上未実装フラグ（D4: `find_invoices.project_name` / `find_purchase_orders.project_name` / `find_payments.purchase_order_id`）は schema 残置 + property description で `(NOT YET SUPPORTED)` 警告。`find_projects.status` は narrowing 必須（N05 確立、API delegation 不可）を description に明記。disambiguate 系 4 tool（find_projects/invoices/purchase_orders/payments）と fanout 系 4 tool（find_estimates/orders/deliveries/receipts）の挙動差を description で峻別。
  - **MCP の refresh / force_refresh 未公開（意図的設計）**: handler は resolver に空 `repository.ReadOptions{}` を渡し、cache hit 前提で動作する。LLM 検索の典型ユースケース（短時間 N 連投）には cache が有効であり、refresh パラメタは tool 設計を複雑化させ LLM の選択精度を低下させるため敢えて公開しない。stale data は CLI 側 `--refresh` で予熱する運用とし、N09 以降の E2E 実機観測で cache miss 多発時に再判断する。
- **N09 完了（2026-04-27）**: E2E テスト再構築。旧 193 ケースから Service 層 41 ケース + MCP handler 経由 5 ケース = **計 46 ケース**へ削減。SKIP 統一テンプレート 4 種（`[SKIP:no-creds|no-data|cache-warm|rate-limit] msg`）で grep 集計可能化。実 API 感応するケースのみを残し、per-batch 実行（rate-limit 配慮）+ CI 非実行（ローカル開発者手動）の運用に移行。
  - **MCP refresh 未公開の再判断（D6）**: 本 N09 では handler 経由 5 ケース (T42-T46) で挙動を確認。cache miss 頻度が観測時に高ければ ADR-002 起票候補（リリース前 N10 で再判断）。
  - **Payment.Project = nil 仮説の E2E 確認（D1）**: T37 (`TestE2E_FindPayment_Search_Smoke`) で実 API 取得結果の全件 `Result.Project == nil` を assert。データ 0 件 (E2E dump 0 件と整合) なら `[SKIP:no-data]`、件数 > 0 で nil 違反検出時はテスト fail → ADR-002 起票が必要。N09 時点では実環境にデータ無しが前提のため D1 仮定維持。

## References

- 調査レポート: [plans/board-phase-n-m01-find-rationale.md](../../plans/board-phase-n-m01-find-rationale.md)
- 計画書: [plans/pure-twirling-coral.md](../../plans/pure-twirling-coral.md)
- Phase H アーカイブ: plans/archive/board-m29～m33-*.md
- 仕様書: [docs/specs/board_cli_mcp_ultra_detailed_design_ja.md §7.9 / §8.5 / §22](../specs/board_cli_mcp_ultra_detailed_design_ja.md)
- MADR 3.x 仕様: https://adr.github.io/madr/
