# N10 詳細計画: v0.7.0 リリース準備

## Meta
| 項目 | 値 |
|------|---|
| Milestone | N10 |
| Status | Ready for Implementation |
| 作成日 | 2026-04-27 |
| 親ロードマップ | plans/board-phase-n-roadmap.md |
| 前マイルストーン | N09（E2E 再構築） |

## Goal

Phase N（N01-N09）で確立した find 層ゼロベース再設計の成果を v0.7.0 として配信できる状態にする。
- CHANGELOG.md の Unreleased を v0.7.0 セクションへ確定。
- README.md の find 仕様を新仕様（11 tool / disambiguate vs fanout / non-fatal enrichment / 構造的不可フラグ）に合わせ最終化。
- マイグレーションガイド `docs/migration/v0.7.0.md` を新規作成（v0.6.0 → v0.7.0 の breaking change ごとの対処）。
- ADR-001 の deferred review trigger を Implementation Status に追記（≈ 2026-05-11）。
- `.claude-plugin/plugin.json` の version を 0.7.0 に bump。
- `git tag v0.7.0` をローカル作成（**push しない**）。

## Scope

### In Scope
1. CHANGELOG.md: Unreleased セクションを `[0.7.0] - 2026-04-27` に確定し、Phase N の breaking changes を構造化（Added / Changed / Changed (Breaking) / Removed / Tests）。
2. docs/migration/v0.7.0.md: 新規作成（v0.6.0 → v0.7.0 のマイグレーションガイド、8 章構成）。
3. README.md: 
   - find tools 12 → 11 に修正（`find_group` 削除）
   - find 層仕様サマリー（disambiguate / fanout / non-fatal）の追記
   - マイグレーションガイドへのリンク
4. ADR-001: Implementation Status に N10 セクション追加（deferred review trigger ≈ 2026-05-11、Payment.Project=nil 本番再評価 / D6 再判断は cache miss 実測待ち）。
5. .claude-plugin/plugin.json: version 0.6.0 → 0.7.0。
6. README_ja.md（存在する場合）: 同等の更新。
7. plans/board-phase-n-roadmap.md: N10 ✅ + Phase N 全完了総括。
8. git tag v0.7.0 ローカル作成。

### Out of Scope
- git push / git push --tags
- GitHub Release 作成 / GoReleaser 起動
- 仕様書 specs の大幅改訂（既に N07c-N09 で同期済み、最終チェックのみ）
- 新規機能追加 / バグ修正

## マイグレーションガイド構造（docs/migration/v0.7.0.md）

```
# v0.7.0 マイグレーションガイド

## 0. 概要
- find 層 (internal/service/find/) を Phase H 仕様から完全ゼロベース再設計
- Tool 数: 12 → 11（find_groups 削除）
- enrichment セマンティクス変更（non-fatal）
- MCP schema 刷新

## 1. find_groups の削除（Breaking）
- 旧: `board find group --name xxx` / MCP `find_groups`
- 新: `board api groups list --name-cont xxx` または `board api users list`
- 理由: Phase H で機能未充実、ADR-001 で削除確定

## 2. enrichment の non-fatal 化（Behavior change）
- 旧: enrichment 失敗で全体 error
- 新: 失敗時は当該フィールド nil / 空配列、warn ログ、主検索 entity は確実に返る
- 影響: Result.Project / Result.Client / Result.Vendor の nil チェックが必要

## 3. name → ID 解決の重複ヒット仕様（Behavior change）
- 旧: silent take-first
- 新: 候補上限 5 件列挙 + ambiguity error → `--id` で絞る
- 影響: 旧コードで部分一致のままの呼び出しは error 化する可能性

## 4. Status / Statuses narrowing 必須化（Breaking）
- find_projects: `--status` / `--statuses[]` のみのクエリは reject（narrow 必須）
  - 必ず `--id` / `--client-name` / `--name` / `--text` のいずれかを併用
- find_invoices / find_purchase_orders / find_payments: `Statuses[]` 単独は reject、`Status` 単独は API delegation で許容
- 理由: full-scan 抑止（rate limit 保護）

## 5. find2 → find rename（Internal API）
- 旧: `internal/service/find/`（Phase H 産物）
- 新: 同一パス、ただし完全に書き直し
- 影響: 外部利用者なし（internal）。Phase N02 開発期 PoC では `find2/` だったが N07b で rename 完了

## 6. Document 4 種 fanout 検索（Behavior change）
- find_estimates / find_orders / find_deliveries / find_receipts は disambiguate ではなく fanout
- `--client-name` / `--project-name` がマッチした全 entity を集約検索
- 1 件に絞る場合は `--project-id` を使用
- delivery / receipt は配列全件をループ（旧 [0] のみ → 完全網羅）

## 7. MCP schema 刷新（Breaking）
- 削除: `find_estimates/orders/deliveries/receipts.status`（構造的不可、Document Entity に Status なし）
  - LLM は schema レベルで使えなくなる
- 警告: `find_invoices.project_name` / `find_purchase_orders.project_name` / `find_payments.purchase_order_id` に `(NOT YET SUPPORTED)`（schema 残置 + description で警告）
- description 全面刷新: disambiguate vs fanout / narrowing 必須を明記

## 8. E2E 構造変更（Internal）
- 旧: 47 関数 / 193 ケース
- 新: Service 層 41 + MCP handler 経由 5 = 46 ケース（78% 削減）
- per-batch 実行（rate-limit 配慮）+ CI 非実行（ローカル手動）
- SKIP 統一テンプレ 4 種: `[SKIP:no-creds|no-data|cache-warm|rate-limit] msg`

## 確認手順
1. `board --version` で 0.7.0 を確認
2. `board mcp serve` 起動後、tools/list で 11 tool（find_groups なし）を確認
3. 既存の MCP 呼び出しコードで Result.Project / Result.Client の nil チェックを追加
4. status-only クエリ箇所を narrowing 付きに修正

## ロールバック
- `brew install youyo/tap/board@0.6.0` または GitHub Releases から v0.6.0 バイナリ取得
```

## CHANGELOG v0.7.0 セクション骨子

```markdown
## [0.7.0] - 2026-04-27

Phase N: find 層ゼロベース再設計 + MCP schema 刷新（v0.7.0 minor bump、ただし複数 breaking changes 含む）

詳細は [docs/migration/v0.7.0.md](docs/migration/v0.7.0.md) 参照。

### Breaking Changes

- find 層を Phase H 仕様から完全ゼロベース再設計（N01-N07）
- MCP tool 数: 12 → 11（`find_groups` 削除、N07b）
- MCP `find_estimates/orders/deliveries/receipts.status` schema 削除（構造的不可、N08）
- `find_projects` の `status` / `statuses[]` 単独クエリを reject（narrowing 必須、N05）
- `find_invoices/purchase_orders/payments` の `statuses[]` 単独クエリを reject（N07a）
- enrichment 失敗時のセマンティクスを非致命に変更（旧: 全体 error → 新: nil + warn + 主 entity 返却、N04）
- name → ID 解決の重複ヒットは silent take-first → ambiguity error（候補上限 5 件、N07c）

### Added

- find 層: name → ID 解決の disambiguate 経路（N07c、`ResolveClientByName` / `ResolveVendorByName`）
- find 層: reverseMapper（document ID → project ID 逆引き、cold > 10s フォールバック、N06）
- find 層: filterByStatuses[T] ジェネリック post-filter（N07a）
- MCP description helper 4 種: disambiguateNameDesc / fanoutNameDesc / notYetSupportedDesc / limitDesc（N08）

### Changed

- MCP tool description / property description を LLM 向けに刷新（N08）
- Document 4 種 fanout 検索: client_name / project_name のマッチ全件集約（N06、disambiguate なし）
- delivery / receipt 配列を全件ループに修正（旧 [0] のみ → 完全網羅、N06）

### Removed

- 旧 `internal/service/find/`（Phase H 産物、47 E2E + 旧 unit、N07b）
- `find_groups` MCP tool / `cli/find_group.go`（ADR-001 forced、N07b）
- 旧 E2E 193 ケース（N09）

### Tests

- E2E テスト再構築: Service 層 41 + MCP handler 経由 5 = 46 ケース（旧 193 から 78% 削減、N09）
- SKIP 統一テンプレート 4 種（grep 集計対応、N09）
- per-batch 実行 + CI 非実行運用（rate-limit 配慮、N09）

### Internal

- find 層実装新規 LOC: 約 +3,000（find2/ → find/ rename 含む）
- Phase N コミット数: 30+
- ADR-001 (find layer) Status: Accepted（B 採択、ゼロベース再設計）
- ADR-001 N06 deferred review trigger: ≈ 2026-05-11（N08 完了 + 2 週間後）
- Payment.Project = nil 仮説（D1）の本番再評価は実環境データ取得後（T37 結果次第）
- MCP refresh 公開判断（D6）は T45/T46 cache miss 頻度実測後

### Migration

`docs/migration/v0.7.0.md` 参照。
```

## ADR-001 Implementation Status 追記

```
- **N10 完了（2026-04-27）**: v0.7.0 リリース準備完了（git tag v0.7.0 作成済 / push 待機）。
  CHANGELOG / README / マイグレーションガイド最終化。
  - **N06 deferred review trigger**: N08 完了 (2026-04-27) + 2 週間後 ≈ **2026-05-11** に find 層呼び出し実績 / LLM 利用パターンを再評価。50% 以下なら ADR-002 起票候補。
  - **Payment.Project = nil 仮説（D1）**: 実環境データ 0 件で N09 T37 検証保留。本番でデータ蓄積後再評価。
  - **MCP refresh 公開判断（D6）**: T45/T46 で cache miss 頻度未観測（[SKIP:no-creds] 想定）。本番運用後実測。
```

## テスト戦略

本マイルストーンは docs / chore のみで production code 0 行変更だが、以下を必須実施:

- `go test ./...` 全 pass（既存テストの retention 確認）
- `go vet ./...` クリーン
- `gofmt -s -w .` 差分なし
- `go build ./...` 成功
- `go run ./cmd/board --version` 出力確認（dev のはず、tag は ldflags 経由で渡す）

## リリース手順（push 待機）

```bash
# 1. すべての doc コミットを完了後
git log --oneline | head

# 2. ローカルタグ作成（push しない）
git tag v0.7.0 -m "v0.7.0: Phase N find layer ground-up redesign"

# 3. ユーザーに push 待ちを通知
git tag --list | grep 0.7.0  # 確認のみ
```

**禁止**:
- `git push origin v0.7.0`
- `git push --tags`
- `gh release create` / GoReleaser 直接起動

## Commit Strategy

論理単位で 4 コミットに分割:

1. `docs(migration): v0.7.0 マイグレーションガイド新規作成`
2. `docs(release): CHANGELOG v0.7.0 セクション + ADR-001 N10 追記`
3. `docs: README v0.7.0 対応（find 11 tool / 仕様サマリー / migration link）+ plugin.json bump`
4. `docs(plans): N10 完了マーク + Phase N 全完了総括`

タグは最後の commit 後に作成。

## Risk

| Risk | 対策 |
|------|------|
| マイグレーションガイドの抜け漏れ | CHANGELOG Unreleased 全項目を章立てで網羅、advisor() レビューで再点検 |
| CHANGELOG 表現の重複・矛盾 | Unreleased を素直に v0.7.0 へ昇格させ、新規重複記述を増やさない |
| README の find tool 数齟齬（12 → 11） | grep "12" / "find_group" でカバレッジ確認 |
| version bump の漏れ | plugin.json / git tag の 2 箇所を必ず更新 |
| タグ誤 push | 手順書で push 禁止を明示、CYCLE_RESULT で push 待ち明示 |

## Rollback

- タグ削除: `git tag -d v0.7.0`
- commit revert: `git revert <hash>` で各 docs コミットを巻き戻し
- 影響: なし（production code 変更ゼロ、docs のみ）

## Open Questions

なし（ハンドオフ情報で全項目確定済）。
