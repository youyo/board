# N08: MCP tools schema 刷新（11 tool）

## Meta
| 項目 | 値 |
|------|---|
| 親 | plans/board-phase-n-roadmap.md（Phase N） |
| 前段 | N07c（CLI/MCP name → ID 解決配線完了） |
| 後段 | N09（E2E テスト再構築） |
| 作成日 | 2026-04-27 |
| ステータス | Ready for Implementation |

## Goal

11 tool（find_groups は N07b で削除済）の MCP schema / description を LLM 利用観点で磨き直す。
具体的には:

1. **schema property の整理** — fanout vs disambiguate の挙動差を反映、project_name の有無を tool 別に正しく
2. **構造的未対応フラグの schema 削除** — Document `--status`（Entity に Status 不存在）を schema からも削除
3. **将来拡張フラグ（"not yet supported"）は schema に残す** — 将来追加時の互換性維持
4. **description の LLM 向け磨き** — 用途・disambiguate 挙動・error 条件を簡潔に追記
5. **refresh パラメタは現状維持（追加しない）** — N07c の handoff known_constraint だが、advisor 判断で deferred

## Scope / Non-Goals

- **In**:
  - `internal/mcpserver/tools.go` 11 tool 定義の schema / description 刷新
  - `internal/mcpserver/tools_test.go` schema 期待値テスト更新（Document の `status` プロパティ削除を反映）
  - error 文言の CLI と MCP の整合性確認
  - CHANGELOG / docs/specs §22-23 / docs/api-reference 更新
- **Out**:
  - refresh / force_refresh パラメタの新設（将来課題、N09 以降で検討）
  - LLM 連携の実機テスト（N09 / 手動）
  - tool handler 内ロジック変更（schema と error 文言のみ）

## 設計判断

### D1: Document 4 種（estimate/order/delivery/receipt）の `status` を schema から削除

- **理由**: Entity に Status フィールド不存在（**構造的に不可能**、never-implementable）。schema に残ると LLM が「使える」と誤認し無駄な call を発生させる。
- **D4 との非対称性の根拠**: D4 の 3 件は「contingently unimplemented（実装可能だが未着手）」であり将来追加見込みあり。D1 は「structurally impossible」で恒久的に追加されない。よって D1 は schema から完全削除、D4 は schema 残置 + description 警告。
- **影響**: handler の reject ガードは **primary defense** として残す（mark3labs/mcp-go は server level で input schema validation を強制しないため、schema 削除のみでは silent ignore になり得る）。
- **breaking**: 旧プロンプトで `status` を渡していた場合、handler reject で error。schema validator 経由でも reject される client もあり挙動は環境依存だが、いずれも error で一貫。CHANGELOG 告知必要。

### D2: Document 4 種の `client_name` / `project_name` description は fanout を明示

- **理由**: LLM が「1 つに絞られる」誤認を避ける。N07c 確立した動作仕様の schema 反映。
- **文言例**: `Resolve project name (substring match) and fanout-search across all matching projects (no disambiguation; use project_id to narrow to a single document).`

### D3: find_projects / find_invoices / find_purchase_orders / find_payments の resolver tool は disambiguate を明示

- **文言例**: `Resolve client name to filter projects. Returns ambiguity error with candidates if multiple clients match; use id to disambiguate.`

### D4: 「not yet supported」3 件は schema に残し description で警告

- 対象:
  - `find_invoices.project_name`（API filter 側で実装可能だが service Query 拡張要、N09+ 課題）
  - `find_purchase_orders.project_name`（同上）
  - `find_payments.purchase_order_id`（同上）
- **理由**: contingently unimplemented（D1 の structurally impossible と対比）。将来追加見込みありのため schema 残置で互換性確保、追加時に schema 再公開不要。
- **文言例**: `(NOT YET SUPPORTED) Will filter purchase orders by project name in a future release. Currently returns an error.`

### D5: 構造的未対応 `find_payments.project_name` は schema に追加しない

- **理由**: Payment Entity が Project 参照を持たない（N07a で確定、PaymentEntity に ProjectID 不在 + E2E dump 0 件）。
- 既に schema に存在しないので「追加しない」だけ。description にも触れない（混乱回避）。

### D6: refresh / force_refresh は現状維持

- 現状 MCP handler は `repository.ReadOptions{}` 空を resolver に渡している（N07c handoff）。
- **理由**:
  1. MCP の典型ユースケース（LLM 検索）は cache hit で十分高速
  2. パラメタ追加で tool 設計が複雑化、LLM の選択精度が下がる
  3. 必要時は CLI 側 `--refresh` で予熱 → MCP は cache を使う運用が現実的
- **N09 以降で再判断**: E2E 実機テストで cache miss 多発が観測された場合のみ追加。
- 現状の空 ReadOptions{} は意図的な設計として ADR-001 / api-reference に明記。

## Schema 設計表（11 tool）

| tool | property | type | required | description（要約） | 変更 |
|------|----------|------|----------|---------------------|------|
| find_clients | id | number | no | Direct ID lookup (highest priority) | - |
| | name | string | no | Substring match on client name | - |
| | text | string | no | Free-text search (lowest priority) | - |
| | limit | number | no | Max results (default 50, max 100) | desc 微修正 |
| find_vendors | id/name/text/limit | 同上（vendor 文言） | - | - | - |
| find_users | id/name/text/limit | 同上（user 文言） | - | - | - |
| find_projects | id | number | no | Direct ID lookup | - |
| | client_name | string | no | Resolve client name; ambiguity error if multiple match | desc 強化 |
| | name | string | no | Substring match on project name | - |
| | text | string | no | Free-text | - |
| | status | string | no | Status filter. **MUST be combined with id / client_name / name / text — status-only query is rejected** (narrowing required, find_projects-only constraint per N05) | desc 強化（N05 narrowing 反映） |
| | limit | number | no | Max results | - |
| find_estimates | id | number | no | Direct lookup | - |
| | project_id | number | no | Filter by project ID | - |
| | client_name | string | no | Fanout search (no disambiguation) | desc 強化 |
| | project_name | string | no | Fanout search; use project_id to narrow | desc 強化 |
| | ~~status~~ | - | - | **削除** | breaking |
| | limit | number | no | Max results | - |
| find_orders | 同 estimates | - | - | - | status 削除 |
| find_deliveries | 同 estimates | - | - | - | status 削除 |
| find_receipts | 同 estimates | - | - | - | status 削除 |
| find_invoices | id | number | no | Direct lookup | - |
| | client_name | string | no | Resolve client; disambiguate if multiple | desc 強化 |
| | project_name | string | no | (NOT YET SUPPORTED) | desc 強化 |
| | text | string | no | Free-text | - |
| | status | string | no | Status filter (single, API delegated; no narrowing required) | desc 強化 |
| | limit | number | no | Max results | - |
| find_purchase_orders | id | number | no | Direct lookup | - |
| | vendor_name | string | no | Resolve vendor; disambiguate if multiple | desc 強化 |
| | project_name | string | no | (NOT YET SUPPORTED) | desc 強化 |
| | text | string | no | Free-text | - |
| | status | string | no | Status filter (single only) | desc 強化 |
| | limit | number | no | Max results | - |
| find_payments | id | number | no | Direct lookup | - |
| | vendor_name | string | no | Resolve vendor; disambiguate if multiple | desc 強化 |
| | purchase_order_id | number | no | (NOT YET SUPPORTED) | desc 強化 |
| | text | string | no | Free-text | - |
| | status | string | no | Status filter (single only) | desc 強化 |
| | limit | number | no | Max results | - |

### required field 方針

全 tool で required = []（すべて optional）。これは既存方針 (`TestRegisterTools_NoRequiredFields`) を維持。
- 理由: LLM が ID だけ・name だけ・text だけ等、ユースケースに応じて最小入力で呼べる柔軟性を保つ
- バリデーション（最低 1 つの絞り込み条件）は handler 内ではなく find service が「無条件 list + limit」で対応（既存挙動）

## Tool description 統一フォーマット

```
<1 行目: 動詞 + 対象>. <2 行目: returns 何を>.
[disambiguation policy: <fanout | disambiguate-error>]
[currently-unsupported flags note if any]
```

例（find_projects）:
```
Search for BOARD projects by ID, client name, project name, or free text.
Returns project entities with enriched client/branch info.
client_name resolves to a single client (ambiguity error if multiple match).
status accepts a single status string only.
```

例（find_estimates）:
```
Search for BOARD estimate documents by ID, project ID, client name, or project name.
Returns estimate document entities (no Status field on entity, hence no status filter).
client_name / project_name perform fanout search across all matching entities; use project_id to narrow to one.
```

## テスト戦略（TDD）

### Red（先に書くテスト）

`tools_test.go` の更新:

1. **`TestRegisterTools_InputSchemaProperties` 更新**:
   - `docTools` の `docProps` から `"status"` を削除
   - 新規 sub-test: Document 4 種が `status` プロパティを **持たない** ことを assert
2. **`TestRegisterTools_DescriptionMentionsDisambiguation`** 新規:
   - find_projects/find_invoices/find_purchase_orders/find_payments の description が "ambiguity" or "disambiguate" or "ambiguous" のいずれかを含むことを assert
3. **`TestRegisterTools_DescriptionMentionsFanout`** 新規:
   - find_estimates/find_orders/find_deliveries/find_receipts の description が "fanout" を含むことを assert
4. **`TestRegisterTools_DescriptionMentionsNotYetSupported`** 新規:
   - find_invoices.project_name / find_purchase_orders.project_name / find_payments.purchase_order_id の **property description** が "not yet supported" を含むことを assert
5. **handler reject の保険テスト**（既存挙動の維持）:
   - 既存の Document `status` reject ガード handler は残すので、handler 直叩きで status を渡すと error を返すことを test（schema 経由なら schema validator で reject されるが二重防御）

### Green

tools.go の編集で全テストを通す。

### Refactor

description の重複文言を helper 関数化（例: `disambiguationDesc("client")` / `fanoutDesc("project")` / `notYetSupportedDesc("project_name", "purchase orders")`）して可読性向上。

## ドキュメント更新

### CHANGELOG.md (Unreleased)

`### Changed (Breaking)` セクションに追加:

- **MCP tool schema から Document 4 種の `status` プロパティを削除（N08）**
  - 対象: `find_estimates` / `find_orders` / `find_deliveries` / `find_receipts`
  - 理由: Entity に Status フィールドが存在しないため、LLM 誤認防止
  - 旧 schema で status を渡していた場合、JSON schema validation で reject されます（handler 側の reject ガードも保険として残置）

`### Changed` セクションに追加:

- **MCP tool description を LLM 向けに磨き込み（N08）**
  - 全 11 tool の description / property description を統一フォーマットに刷新
  - disambiguate 挙動（client_name / vendor_name → ambiguity error）を明示
  - fanout 挙動（Document 4 種の client_name / project_name → no disambiguation）を明示
  - "not yet supported" フラグを property description で警告

### docs/specs/board_cli_mcp_ultra_detailed_design_ja.md §22 / §23

- §23.2 ツール一覧を 11 tool に更新（find_groups 削除済を明記）
- §23.4 共通入力 schema に N08 で確定した property 一覧を反映
- §23.5 入力方針に「全フィールド optional / required = [] 維持」を明記
- §23.8 エラー: disambiguate / not-yet-supported / does-not-support の 3 種類を区別
- refresh パラメタを「現状未公開、N09+ で再評価」と注記

### docs/api-reference.md

MCP セクションがあれば、新 description / schema 例を反映。なければ簡易追記。

## Rollback plan

- schema 変更は data-loss なし
- 全変更は `internal/mcpserver/tools.go` + `internal/mcpserver/tools_test.go` + ドキュメント のみ
- `git revert <commit>` で完全復帰

## Risk 評価

| risk | 影響 | mitigation |
|------|------|------------|
| 旧 LLM プロンプトが Document の `status` を送信 | schema validator reject（=従来 handler reject と同じ error） | CHANGELOG breaking 告知 + handler reject ガード残置 |
| description が長すぎて context 浪費 | tool selection 精度低下リスクの逆 | 統一フォーマット（3-4 行以内）で制約 |
| description の magic value（"id="/"name="）が古いプロンプトと乖離 | 旧プロンプトとの非互換 | resolver error 文言は CLI/MCP 共通（N07c 確立済） |
| refresh 未公開で stale data | 検索結果が古い場合あり | ADR-001 / api-reference に明記、N09 で再判断 |

## 実装手順

1. **テスト Red**: `tools_test.go` を 5 観点（D1-D5）で更新（既存 test の docProps から status 削除 / 新規 4 test）
2. **`go test ./internal/mcpserver/`** で fail 確認
3. **tools.go 編集**:
   - Document 4 種の `mcp.WithString("status", ...)` を削除
   - 全 tool description / property description を新フォーマットで書き直し
   - helper 関数 (`disambiguationDesc` / `fanoutDesc` / `notYetSupportedDesc`) を追加
4. **`go test -race ./...` / `go vet ./...` / `gofmt -s -w .`** 全 pass
5. **CHANGELOG / docs/specs §22-23 / docs/api-reference / ADR-001 更新**
6. **commit 分割**:
   - commit 1: tools_test.go の Red（test 更新のみ、未 pass 状態でも OK）
   - commit 2: tools.go 実装（test pass）
   - commit 3: ドキュメント更新（CHANGELOG / specs / api-reference）
   - commit 4 (optional): refactor（helper 切り出し）

## 完了条件

- [ ] `go test -race ./...` 全 pass
- [ ] `go vet ./...` clean
- [ ] `gofmt -s -w .` 差分なし
- [ ] CHANGELOG Unreleased に N08 entry
- [ ] docs/specs §23 が 11 tool 体制に更新
- [ ] roadmap N08 完了マーク + Current Focus → N09
