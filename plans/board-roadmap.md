# Roadmap: BOARD CLI / MCP 統合ツール

## Meta
| 項目 | 値 |
|------|---|
| ゴール | BOARD API を叩く Go 製 CLI + ローカル HTTP MCP サーバーを単一バイナリで提供する |
| 成功基準 | 全22リソースの list/get/search が CLI・MCP 両方で動作し、SQLite キャッシュで rate limit に耐える |
| 制約 | Go 1.26 / modernc.org/sqlite (CGO不要) / macOS+Linux (amd64/arm64) / BOARD API rate limit 3000/日・3/秒 |
| 対象リポジトリ | /Users/youyo/src/github.com/youyo/board |
| スペック | docs/specs/board_cli_mcp_ultra_detailed_design_ja.md |
| 作成日 | 2026-04-08 |
| 最終更新 | 2026-04-08 07:40 |
| ステータス | 未着手 |

## 技術スタック決定事項
| カテゴリ | 選定 | 理由 |
|---------|------|------|
| 言語 | Go 1.26 (mise) | ユーザー指定 |
| CLI | spf13/cobra | デファクト |
| TOML | pelletier/go-toml/v2 | 型安全 |
| SQLite | modernc.org/sqlite | CGO不要、クロスコンパイル容易 |
| MCP | mark3labs/mcp-go | Go MCP SDK |

## BOARD API リソース一覧（全22リソース）
| # | API パス | 日本語名 | カテゴリ |
|---|---------|---------|---------|
| 1 | clients | 顧客 | コア |
| 2 | client_branches | 顧客支社 | コア |
| 3 | contacts | 顧客担当者 | コア |
| 4 | projects | 案件 | コア |
| 5 | project_costs | 案件原価 | コア |
| 6 | estimates | 見積書 | ドキュメント |
| 7 | invoices | 請求 | ドキュメント |
| 8 | orders | 発注書 | ドキュメント |
| 9 | deliveries | 納品書 | ドキュメント |
| 10 | receipts | 領収書 | ドキュメント |
| 11 | vendors | 発注先 | ベンダー |
| 12 | vendor_branches | 発注先支社 | ベンダー |
| 13 | vendor_contacts | 発注先担当者 | ベンダー |
| 14 | purchase_orders | 発注 | ベンダー |
| 15 | payments | 支払 | ベンダー |
| 16 | users | ユーザー | マスタ |
| 17 | groups | グループ | マスタ |
| 18 | payment_terms | 支払条件 | マスタ |
| 19 | project_types | 案件区分 | マスタ |
| 20 | purchase_types | 発注区分 | マスタ |
| 21 | accounting_types | 会計区分 | マスタ |
| 22 | document_send_channels | カスタム書類送付方法 | マスタ |

## Current Focus
- **マイルストーン**: M01 プロジェクト初期化
- **直近の完了**: ロードマップ作成
- **次のアクション**: M01 の実装開始

## 併走ロードマップ
- **準拠検証 & E2E 網羅**: plans/board-compliance-roadmap.md（2026-04-20 開始）
  - 全 22 リソースの E2E を 34 マイルストーンで細粒度に整備する別軸計画。
  - 既存 M01〜M40 の機能ロードマップとは独立。Rate limit を理由に手動 1-request 粒度で進める。

## Progress

---

### Phase 1: 基盤（M01〜M10）

#### M01: プロジェクト初期化
- [ ] go mod init + mise 設定 + .gitignore
- [ ] ディレクトリ構成作成
- [ ] 主要依存ライブラリ追加
- [ ] mise タスク定義 + 基本ビルド確認
- 📄 詳細: plans/board-m01-project-init.md

#### M02: config パッケージ
- [ ] Config / ProfileConfig 型定義
- [ ] TOML load / save / defaults / path
- [ ] テスト
- 📄 詳細: plans/board-m02-config-package.md（着手時生成）

#### M03: configure CLI コマンド
- [ ] 対話式 configure
- [ ] set / get / show / list-profiles / use / current-profile / path
- [ ] secrets マスク
- 📄 詳細: plans/board-m03-configure-cli.md（着手時生成）

#### M04: boardapi 共通クライアント基盤
- [ ] Client 構造体 + HTTP transport
- [ ] 認証ヘッダ付与 (x-api-key + Bearer)
- [ ] APIError 型 + エラー正規化
- [ ] テスト (httptest)
- 📄 詳細: plans/board-m04-boardapi-base.md（着手時生成）

#### M05: boardapi retry + pagination
- [ ] 指数バックオフ + ジッター + Retry-After
- [ ] 429 / 5xx リトライ、4xx 非リトライ
- [ ] ページネーション吸収ヘルパー (per_page, page)
- [ ] テスト
- 📄 詳細: plans/board-m05-boardapi-retry-pagination.md（着手時生成）

#### M06: boardapi コアエンティティ
- [ ] 型定義: clients, client_branches, contacts, projects, project_costs
- [ ] API メソッド: List / Get / Search（各リソース）
- [ ] テスト
- 📄 詳細: plans/board-m06-boardapi-core-entities.md（着手時生成）

#### M07: boardapi ドキュメント系
- [ ] 型定義 + API メソッド: estimates, invoices, orders, deliveries, receipts
- [ ] テスト
- 📄 詳細: plans/board-m07-boardapi-documents.md（着手時生成）

#### M08: boardapi ベンダー + 支払系
- [ ] 型定義 + API メソッド: vendors, vendor_branches, vendor_contacts, purchase_orders, payments
- [ ] テスト
- 📄 詳細: plans/board-m08-boardapi-vendors.md（着手時生成）

#### M09: boardapi マスタ系
- [ ] 型定義 + API メソッド: users, groups, payment_terms, project_types, purchase_types, accounting_types, document_send_channels
- [ ] テスト
- 📄 詳細: plans/board-m09-boardapi-masters.md（着手時生成）

#### M10: SQLite 初期化 + マイグレーション
- [ ] db.go (接続管理, WAL モード)
- [ ] migrate.go + schema.go
- [ ] DDL: resource_cache, sync_state, cache_meta
- [ ] テスト (in-memory DB)
- 📄 詳細: plans/board-m10-sqlite-init.md（着手時生成）

---

### Phase 2: キャッシュ + リフレッシュ + リポジトリ（M11〜M18）

#### M11: cache - resource_cache 実装
- [ ] Upsert / UpsertMany / Get / List / Search / Delete
- [ ] JSON blob 保存 / 読み出し
- [ ] keys.go (キー生成)
- [ ] テスト
- 📄 詳細: plans/board-m11-resource-cache.md（着手時生成）

#### M12: cache - sync_state + cache_meta
- [ ] sync_state CRUD
- [ ] cache_meta CRUD
- [ ] テスト
- 📄 詳細: plans/board-m12-sync-state.md（着手時生成）

#### M13: refresh policy + daily 判定
- [ ] policy.go: needsDailyRefresh
- [ ] daily.go: timezone考慮の日付判定
- [ ] テスト
- 📄 詳細: plans/board-m13-refresh-policy.md（着手時生成）

#### M14: refresh - delta + force + updater
- [ ] resource_refresh.go: 差分取得 (updated_at_gteq cursor)
- [ ] force_refresh.go: 全件再取得
- [ ] updater.go: sync_state 更新
- [ ] テスト
- 📄 詳細: plans/board-m14-refresh-engine.md（着手時生成）

#### M15: ロック + 多重実行制御
- [ ] profile×resource の in-process mutex
- [ ] stale lock 検出 (10分タイムアウト)
- [ ] refresh_started_at / refresh_owner 管理
- [ ] テスト
- 📄 詳細: plans/board-m15-lock-control.md（着手時生成）

#### M16: repository - コアエンティティ
- [ ] clients / client_branches / contacts / projects / project_costs
- [ ] List / GetByID / Search（cache → refresh → API fallback）
- [ ] ReadOptions (refresh, forceRefresh, limit)
- [ ] テスト (SQLite temp DB)
- 📄 詳細: plans/board-m16-repo-core.md（着手時生成）

#### M17: repository - ドキュメント + ベンダー系
- [ ] estimates / invoices / orders / deliveries / receipts
- [ ] vendors / vendor_branches / vendor_contacts / purchase_orders / payments
- [ ] テスト
- 📄 詳細: plans/board-m17-repo-docs-vendors.md（着手時生成）

#### M18: repository - マスタ系
- [ ] users / groups / payment_terms / project_types 等
- [ ] テスト
- 📄 詳細: plans/board-m18-repo-masters.md（着手時生成）

---

### Phase 3: CLI + low-level コマンド（M19〜M28）

#### M19: app パッケージ + CLI 基盤
- [ ] app.go / container.go / runtime.go（DI コンテナ）
- [ ] root.go（Cobra root command + 共通フラグ）
- [ ] 共通フラグ: --profile, --refresh, --force-refresh, --pretty, --limit
- 📄 詳細: plans/board-m19-app-cli-base.md（着手時生成）

#### M20: output パッケージ
- [ ] json.go: デフォルト JSON 出力
- [ ] pretty.go: --pretty 整形表示
- [ ] mask.go: secrets マスク
- [ ] テスト
- 📄 詳細: plans/board-m20-output.md（着手時生成）

#### M21: service/api - コアエンティティ
- [ ] clients_service / projects_service / contacts_service 等
- [ ] List / GetByID / Search
- [ ] テスト
- 📄 詳細: plans/board-m21-service-api-core.md（着手時生成）

#### M22: service/api - ドキュメント + ベンダー + マスタ系
- [ ] 残り全リソースの service/api
- [ ] テスト
- 📄 詳細: plans/board-m22-service-api-rest.md（着手時生成）

#### M23: api CLI - コアエンティティコマンド
- [ ] board api clients list/get/search
- [ ] board api projects list/get/search
- [ ] board api contacts list/get/search
- [ ] board api client_branches / project_costs
- 📄 詳細: plans/board-m23-api-cli-core.md（着手時生成）

#### M24: api CLI - ドキュメント系コマンド
- [ ] board api estimates / invoices / orders / deliveries / receipts
- 📄 詳細: plans/board-m24-api-cli-docs.md（着手時生成）

#### M25: api CLI - ベンダー系コマンド
- [ ] board api vendors / vendor_branches / vendor_contacts / purchase_orders / payments
- 📄 詳細: plans/board-m25-api-cli-vendors.md（着手時生成）

#### M26: api CLI - マスタ系コマンド
- [ ] board api users / groups / payment_terms / project_types / purchase_types / accounting_types / document_send_channels
- 📄 詳細: plans/board-m26-api-cli-masters.md（着手時生成）

#### M27: cache CLI コマンド
- [ ] board cache status / expire / clear / path
- [ ] --profile 必須 (clear all)
- [ ] テスト
- 📄 詳細: plans/board-m27-cache-cli.md（着手時生成）

#### M28: completion zsh
- [ ] board completion zsh
- [ ] 動作確認
- 📄 詳細: plans/board-m28-completion.md（着手時生成）

---

### Phase 4: high-level CLI（M29〜M34）

#### M29: service/find - 顧客・案件横断検索
- [x] find_client: 名前 → 顧客 + 支社 + 担当者を横断
- [x] find_project: 顧客名 → client 解決 → project 検索
- [x] テスト
- 📄 詳細: plans/board-m29-service-find-core.md

#### M30: service/find - ドキュメント横断検索
- [ ] find_estimate / find_invoice: 顧客名・案件名から書類検索
- [ ] find_order / find_delivery / find_receipt
- [ ] テスト
- 📄 詳細: plans/board-m30-service-find-docs.md（着手時生成）

#### M31: service/find - ベンダー・マスタ検索
- [x] find_vendor / find_purchase_order / find_payment
- [x] find_user / find_group 等
- [x] テスト
- 📄 詳細: plans/board-m31-service-find-rest.md（着手時生成）

#### M32: find CLI - 顧客・案件コマンド
- [x] board find client / board find project
- [x] --id, --name, --text 等フラグ
- 📄 詳細: plans/board-m32-find-cli-core.md（着手時生成）

#### M33: find CLI - ドキュメント系コマンド
- [ ] board find estimate / invoice / order / delivery / receipt
- 📄 詳細: plans/board-m33-find-cli-docs.md（着手時生成）

#### M34: find CLI - ベンダー・マスタ系コマンド
- [ ] board find vendor / purchase_order / payment / user 等
- 📄 詳細: plans/board-m34-find-cli-rest.md（着手時生成）

---

### Phase 5: MCP サーバー（M35〜M39）

#### M35: MCP server 基盤
- [ ] server.go: mcp-go 初期化
- [ ] transport_http.go: ローカル HTTP transport
- [ ] テスト
- 📄 詳細: plans/board-m35-mcp-server-base.md（着手時生成）

#### M36: MCP tools 定義 + schema
- [ ] tools.go: ツール登録
- [ ] schema.go: JSON Schema 定義（全ツール分）
- [ ] 共通入力・出力スキーマ
- 📄 詳細: plans/board-m36-mcp-tools-schema.md（着手時生成）

#### M37: MCP tools - コアエンティティ + ドキュメント
- [ ] find_clients / find_projects / find_estimates / find_invoices 等
- [ ] テスト
- 📄 詳細: plans/board-m37-mcp-tools-core-docs.md（着手時生成）

#### M38: MCP tools - ベンダー + マスタ
- [ ] find_vendors / find_purchase_orders / find_users 等
- [ ] テスト
- 📄 詳細: plans/board-m38-mcp-tools-rest.md（着手時生成）

#### M39: MCP serve CLI + E2E 検証
- [ ] board mcp serve (--profile, --host, --port)
- [ ] E2E テスト: CLI → MCP → cache 一気通貫
- [ ] エッジケース確認
- 📄 詳細: plans/board-m39-mcp-serve-e2e.md（着手時生成）

### Phase 6: 英語化 + 仕上げ（M40）

#### M40: コード内メッセージ・ヘルプ表示の英語化
- [ ] CLI コマンドの Use/Short/Long/Example を全て英語に統一
- [ ] エラーメッセージを英語に統一
- [ ] コード内コメントの英語化（公開インターフェース）
- [ ] --help 出力の確認
- [ ] テスト内の期待値文字列を更新
- [ ] README.md を英語で作成（プロジェクト概要、インストール、使い方、設定）
- [ ] docs/README_ja.md を日本語版として作成
- [ ] README.md から日本語版へのリンクを設置
- 📄 詳細: plans/board-m40-english-messages.md（着手時生成）

---

## Blockers
なし

## Architecture Decisions
| # | 決定 | 理由 | 日付 |
|---|------|------|------|
| ADR-001 | SQLite は modernc.org/sqlite を使用 | CGO不要でクロスコンパイル容易。このユースケースではパフォーマンス差が体感不可 | 2026-04-08 |
| ADR-002 | BOARD API リソース名はAPI準拠（clients等）に修正 | スペック原案は仮名(customers,deals,tickets)だったが、実API名に合わせる | 2026-04-08 |
| ADR-003 | Go 1.26 + mise で管理 | ユーザー指定 | 2026-04-08 |
| ADR-004 | 全22リソースをMVPスコープに含める | ユーザー指定。パターンが共通なので追加コストは逓減する | 2026-04-08 |
| ADR-005 | コード内メッセージ・ヘルプは全て英語 | ユーザー指定。CLI ツールの国際的な慣例に合わせる | 2026-04-08 |

## Changelog
| 日時 | 種別 | 内容 |
|------|------|------|
| 2026-04-08 07:40 | 作成 | ロードマップ初版作成。全39マイルストーン、半日粒度 |
| 2026-04-08 14:45 | 追加 | M40 英語化マイルストーンを Phase 6 として追加。ADR-005 追加 |
