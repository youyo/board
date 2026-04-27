# board DynamoDB cache backend — 設計ドキュメント

ステータス: **draft（実装前のレビュー待ち）**

Lambda Function URL での board MCP デプロイにおいて、SQLite キャッシュの永続性とマルチコンテナ共有の限界を解消するため、DynamoDB をバックエンドに選択できるようにする。SQLite はローカル CLI 用途に維持する。

## 1. 動機と前提

### 動機
- Lambda コンテナの `/tmp` SQLite は warm reuse 期間のみ有効、cold start で消える
- マルチコンテナ並行で同じ cache を共有できない
- BOARD API は 3 req/sec / 3000 req/day の rate limit があり、cache miss の山積みを避けたい
- S3 Files (April 2026 GA) も検討対象だが VPC 必須で cold start オーバヘッドあり、SQLite + NFS の lease lock は信頼性に懸念

### 非目標
- `cache_meta`（DB schema version）の DynamoDB 化 — schemaless なので不要
- per-user OAuth token store — board は共有 API key のため不要
- SQLite 廃止 — ローカル CLI では引き続き第一選択

### 前提
- 1 profile = 1 BOARD アカウント = 1 セット cache
- entity 1 件の payload は最大 ~数十 KB（プロジェクト等）想定。400KB 上限内
- entity 全件 list は数百〜数千件オーダー（22 リソース合計でも数万件）

## 2. 現状の cache 層と抽象化方針

### 現状（SQLite）
```
internal/cache/
  ├── ResourceCache      — 22 resource × profile の entity blob CRUD
  ├── SyncStateStore     — per-resource sync state CRUD (refresh lock 含む)
  └── CacheMetaStore     — KV (db_schema_version) ※ 内部のみ
```

`internal/repository/*.go` は **構造体ポインタ** (`*cache.ResourceCache`, `*cache.SyncStateStore`) を直接受けており差し替え不可。

### 提案: interface 切り出し

`internal/cache` にインターフェースを追加し、SQLite 実装はそのまま、DynamoDB 実装を別パッケージで提供:

```go
// internal/cache/interfaces.go (新規)
package cache

type ResourceCacheBackend interface {
    Upsert(ctx context.Context, e Entry) error
    UpsertMany(ctx context.Context, es []Entry) error
    Get(ctx context.Context, key EntityKey) (*Entry, error)
    List(ctx context.Context, profile, resource string) ([]Entry, error)
    Delete(ctx context.Context, key EntityKey) error
    DeleteAll(ctx context.Context, profile, resource string) error
}

type SyncStateBackend interface {
    Get(ctx context.Context, profile, resource string) (*SyncState, error)
    Upsert(ctx context.Context, s SyncState) error
    Delete(ctx context.Context, profile, resource string) error
    ListAll(ctx context.Context, profile string) ([]SyncState, error)
    Expire(ctx context.Context, profile, resource, now string) error
    ExpireAll(ctx context.Context, profile, now string) error
    DeleteAll(ctx context.Context, profile string) error
}
```

repository 側を `ResourceCacheBackend` / `SyncStateBackend` 受け取りに変える（型名置換のみ）。

DynamoDB 実装は `internal/cache/dynamo/` に置く:

```
internal/cache/dynamo/
  ├── resource_cache.go   — ResourceCacheBackend
  ├── sync_state.go       — SyncStateBackend
  ├── client.go           — aws-sdk-go-v2 client wrapper
  ├── schema.go           — CreateTableInput
  └── *_test.go           — DynamoDB Local 用テスト
```

## 3. DynamoDB スキーマ

### 設計方針
- **2 テーブル構成** （resource_cache / sync_state を 1:1 で対応させる）
  - 1 テーブルにまとめると複合 PK が複雑化、コスト見積もりも掴みにくい
  - 2 テーブルなら schema が直感的、コスト分析しやすい
- **Composite primary key**: PK + SK で profile/resource/entity を表現
- **GSI 使用**: `List` を `updated_at` 順にするため必要
- **PAY_PER_REQUEST** （on-demand）: Lambda の不定 traffic と相性が良い

### Table 1: `<prefix>-resource-cache`

| Attribute | Type | Role |
|---|---|---|
| `pk` (PK)  | S | `profile#resource` 例: `default#projects` |
| `sk` (SK)  | S | `entity_id` 例: `123456` |
| `payload`  | B (binary) | gzip 圧縮された entity JSON |
| `updated_at` | S | API entity の updated_at（RFC3339 / nullable=空文字） |
| `fetched_at` | S | RFC3339 |
| `ttl`      | N | epoch sec、削除予定時刻（任意、`expired_at` ベース）|

**GSI**: `updated_at-index`
- partition key: `pk`
- sort key: `updated_at`
- 用途: `List` を `updated_at ASC` で返す（既存 SQLite 挙動踏襲）
- projection: ALL（payload も同梱、Query 1 回で完結）

**操作マッピング**:
| 既存 API | DynamoDB |
|---|---|
| `Upsert(entry)` | `PutItem` |
| `UpsertMany(entries)` | `BatchWriteItem`（25 件チャンクで分割）|
| `Get(key)` | `GetItem` |
| `List(profile, resource)` | `Query` on GSI `updated_at-index`, `pk = profile#resource` |
| `Delete(key)` | `DeleteItem` |
| `DeleteAll(profile, resource)` | `Query → BatchWriteItem(Delete)` をループ |

### Table 2: `<prefix>-sync-state`

| Attribute | Type | Role |
|---|---|---|
| `pk` (PK)  | S | `profile` |
| `sk` (SK)  | S | `resource` |
| `last_synced_at` ... 18 fields | S/N | SyncState の各フィールドをそのまま map |
| `must_full_resync` | N (0/1) | bool 代替 |

**GSI 不要**: `ListAll(profile)` は `Query pk = profile` で全件取得（resource ASC は SK ソート順）。

**操作マッピング**:
| 既存 API | DynamoDB |
|---|---|
| `Get` | `GetItem` |
| `Upsert` | `PutItem`（19 フィールド）|
| `Delete` | `DeleteItem` |
| `ListAll(profile)` | `Query pk = profile`、 SK ASC |
| `Expire` | `UpdateItem`（条件付き）|
| `ExpireAll` | `Query → UpdateItem` ループ（コスト要監視） |
| `DeleteAll` | `Query → BatchWriteItem(Delete)` ループ |

### Refresh Lock の整合性

現状: `MarkLockAcquired` → `Upsert(state)` で `refresh_started_at` + `refresh_owner` をセット。SQLite はファイル単位ロックで「先勝ち」が成立するが、DynamoDB では条件付き書き込みで実装する必要がある。

```go
// 提案: 新メソッド追加
type SyncStateBackend interface {
    // ...
    AcquireLock(ctx, profile, resource, owner, now string) (acquired bool, err error)
    ReleaseLock(ctx, profile, resource, owner string) error
}
```

DynamoDB `UpdateItem` with `ConditionExpression`:
```
SET refresh_started_at = :now, refresh_owner = :owner
ConditionExpression: attribute_not_exists(refresh_owner)
                    OR refresh_started_at < :stale_threshold
```

これで stale lock takeover も atomic に行える。SQLite 実装でも equivalent な lock 取得関数を新設し、refresh layer はその interface を呼ぶ形にリファクタする（refactor scope: `internal/refresh/lock.go`）。

## 4. payload 圧縮と 400KB 上限

DynamoDB item の上限は 400KB。生 JSON のままだと:
- `clients` entity: 数 KB 〜 10KB → OK
- `projects` (with embeddings): 50KB〜? → 上限内
- 万一超えるリソースが出た場合に備え **gzip 圧縮で B 型に格納**

```go
// 書き込み: payload_json → gzip → []byte
// 読み出し: []byte → gunzip → json.RawMessage
```

圧縮率は JSON で 60-80% 期待。50KB → 10-20KB に縮む。

## 5. CLI / 設定の追加

### 設定ファイル (`config.toml`)
```toml
[profile.default]
api_key = "..."
api_token = "..."
daily_auto_refresh = false   # 別 PR で OFF default 化済（仮定）

[profile.default.cache]
backend = "sqlite"  # "sqlite" | "dynamodb"  (default: sqlite)

# backend = "dynamodb" 時のみ
[profile.default.cache.dynamodb]
resource_table = "board-cache-resource"
sync_state_table = "board-cache-sync-state"
region = "ap-northeast-1"
gzip = true   # default true
```

### 環境変数 (Lambda 用)
- `BOARD_CACHE_BACKEND` = `dynamodb`
- `BOARD_CACHE_DYNAMODB_RESOURCE_TABLE`
- `BOARD_CACHE_DYNAMODB_SYNC_STATE_TABLE`
- `BOARD_CACHE_DYNAMODB_REGION`

## 6. 実装マイルストーン

| M | 名称 | 工数 | 内容 |
|---|---|---|---|
| D01 | interface 切り出し | S | `cache.ResourceCacheBackend` / `SyncStateBackend` を定義し repository を interface 受け取りに |
| D02 | refresh lock の atomic 化 | M | `AcquireLock` / `ReleaseLock` API 追加、SQLite 実装、refresh layer の切り替え |
| D03 | DynamoDB schema + table 作成 task | S | `internal/cache/dynamo/schema.go`, mise task 追加 |
| D04 | DynamoDB ResourceCacheBackend 実装 | M | gzip + GSI Query + BatchWrite |
| D05 | DynamoDB SyncStateBackend 実装 | M | 19 フィールド mapping + atomic lock |
| D06 | DynamoDB Local テスト整備 | M | CI で localstack/dynamodb-local を起動して contract test |
| D07 | factory 切り替え | S | `app.go` で config/env を見て backend を選択 |
| D08 | examples/lambroll/ 統合 | S | function.json に env 追加、IAM policy 追加、mise task 追加 |
| D09 | docs / migration ガイド | S | README, migration ガイド |

合計: **S=4, M=4, total ~6-8 営業日**（テスト・review 込み）

## 7. リスクと未解決ポイント

### R1: List クエリのコスト
`List(profile, resource)` は GSI Query で全件取得。N=数千件 のリソース（projects 等）で 1 回 Query = 全件 RCU を消費。
- 対策: PAY_PER_REQUEST なら絶対量で課金、極端ではない見込み（projects 1000 件 ≈ 1MB ≈ 0.5 RCU × 2 = 1 RCU 相当）
- Worst case でも 1 list = $0.0001 オーダー

### R2: BatchWrite サイズ制限
`BatchWriteItem` は 25 items / 16MB / item 400KB。`UpsertMany` で chunk 分割 + retry が必要。

### R3: Refresh lock の stale 検知タイミング
SQLite 実装と stale 判定（5分以上経過で takeover 等）の整合性をテストで担保。

### R4: payload size 上限超過
監視ログで「item too large」検知時に slog.Error + 該当 entity をスキップ。Hard fail させない方針（現状の SQLite と挙動を揃える）。

### R5: Lambda cold start
DynamoDB SDK 初期化 ~50-100ms。許容範囲だが計測推奨。

### R6: 既存 SQLite からの移行
Lambda 用には migration 不要（cache は再構築で OK）。CLI ユーザーが DynamoDB に移行する積極的なユースケースは想定していないため、migration script は提供しない方針。

### R7: テスト戦略
- DynamoDB 実装の unit test は `dynamodb-local` (Docker) 利用
- CI でコンテナ起動 → contract test 実行
- E2E は手動（実 AWS）or moto で代替

## 8. 採用しない設計

- **single-table design**: PK/SK 衝突を避ける管理コストが高く、cache の 2 種類の concern を分離する利点を相殺
- **Item collection で transaction**: TransactWriteItems は 100 item / 4MB の制限がきつく、`UpsertMany` の典型サイズを超える
- **DynamoDB Streams で invalidation**: cache 寿命は daily refresh 単位で自然に切り替わるためイベント駆動 invalidation は overkill

## 9. 開発順序の推奨

1. **D01-D02 を先行**（interface 化 + lock の atomic 化）— SQLite だけで完結する変更で main 取り込み
2. **D03-D06**（DynamoDB 本体）— 1 PR で実装 + dynamodb-local テスト
3. **D07-D09**（wire-up + Lambda 統合 + docs）— 動作確認できる状態にする

## 10. 未確定事項（実装着手前に確定したい）

- [ ] テーブル名 prefix の流儀（profile を含めるか、prefix 固定か）
- [ ] payload 圧縮を default ON にするか opt-in か（→ default ON を提案）
- [ ] CLI `board cache info` 等のオブザビリティコマンド追加要否
- [ ] DynamoDB 自身の auto-scaling / TTL 設定の有無（→ TTL は使わず明示削除のみ推奨）
- [ ] 既存 mise task `dynamodb-create` を idproxy store と兼用可能にする方針
