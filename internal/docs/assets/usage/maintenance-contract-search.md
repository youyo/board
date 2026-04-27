# 保守契約の有効性確認 — 段階的検索ガイド

特定顧客の保守契約有効性を BOARD 案件情報から確認する際、`find_projects` MCP ツール / `board find project` CLI コマンドを活用した段階的検索方法を説明します。

## 目的と関連機能

保守契約有効性確認は、以下のようなユースケースに対応します：

> 「アクメ社の保守契約は現在有効か？有効であればプロジェクト ID、契約期間を確認したい。」

このプロセスは手動検索では煩雑（複数 status 名の把握、AND/OR セマンティクスの確認）なため、LLM / MCP ツール経由での自動化が有効です。v0.7.0 以降で対応した `contract_status` alias により、LLM が業務用語（`active` / `ended` / `prospect` / `all`）をそのまま指定できます。

関連する主要機能：
- `find_projects` MCP tool（ローカル HTTP MCP サーバー）
- `board find project` CLI コマンド
- `--contract-status` alias フラグ
- `--statuses` / `statuses[]` 配列フィルタ（status 名を複数指定）

## クイックスタート

### MCP プロンプト雛形

LLM に以下のインストラクションを与えて段階的検索を実現します：

```
「アクメ社の保守契約の有効性を確認してください。

1) find_projects(contract_status="active", client_name="アクメ", name="保守") を呼ぶ
2) 0 件だった場合、contract_status="ended" で再呼び出し
3) それでも 0 件なら contract_status="prospect" で再呼び出し
4) ヒットしたら案件 ID と関連情報を要約」
```

### CLI コマンド例

```bash
# アクメ社の保守案件の中で、契約が有効（未着手/着手中/納品済）なもの
board find project --client-name "アクメ" --name "保守" --contract-status active

# 同じく、契約が終了済み（検収済）なもの
board find project --client-name "アクメ" --name "保守" --contract-status ended

# 見積中の見込み案件を確認
board find project --client-name "アクメ" --name "保守" --contract-status prospect

# または細粒度指定: BOARD status 名をそのまま複数指定（OR 検索）
board find project --name "保守" --statuses 受注済,納品済
```

## contract_status alias 値域表

`contract_status` は以下 4 つの alias 値をサポートします。各 alias は特定の delivery status / order status の集合に対応：

| alias | 評価対象フィールド | 含まれる status 名 | 用途 |
|-------|---|---|---|
| `active` | DeliveryStatusName のみ | 未着手 / 着手中 / 納品済 | 進行中または完了した契約 |
| `ended` | DeliveryStatusName のみ | 検収済 | 契約が検収完了した案件 |
| `prospect` | OrderStatusName のみ | 見積中(高) / 見積中(中) / 見積中(低) / 見積中(除) | 見積段階の見込み案件 |
| `all` | 上記すべて | 上記 4 種すべて | 全段階の案件を一括取得 |

## 段階的検索の推奨ループ

MCP / CLI で単一呼び出しでは結果が 0 件でも、段階的に別 alias で再検索することで契約状態の全体像を把握できます。推奨ステップ：

1. **Step 1: `active` を検索**
   - 意図: 現在進行中・納品済の契約を確認
   - 該当する場合: 契約有効。案件 ID 等を報告して終了

2. **Step 2: `active` で 0 件の場合、`ended` を検索**
   - 意図: 過去に検収完了した契約がないか確認
   - 該当する場合: 過去契約。実績として報告

3. **Step 3: `ended` でも 0 件の場合、`prospect` を検索**
   - 意図: 見積段階の見込み案件がないか確認
   - 該当する場合: 未確定契約。見積状況として報告

4. **いずれでも該当なし**
   - 契約の記録がない旨を報告

**重要**: このループは **サーバー側ではなく LLM / ユーザー側で実装** します。`board` サーバーはステートレスであり、1 呼び出し 1 結果です。複数段階の検索は別々のリクエストで実現してください。

## statuses 配列の使い分け

`--statuses` / `statuses[]` は BOARD status 名を**複数指定可能** で、OR 検索になります。`contract_status` alias との使い分けは以下の通り：

### contract_status を使う場合（推奨）

業務ドメイン用語で直感的に検索：

```bash
# MCP: find_projects(contract_status="active", client_name="アクメ", name="保守")
board find project --client-name "アクメ" --name "保守" --contract-status active
```

### statuses を使う場合

BOARD status 名そのものをピンポイント指定（より細粒度）:

```bash
# MCP: find_projects(statuses=["受注済", "納品済"], name="保守")
board find project --name "保守" --statuses 受注済,納品済
```

**OR セマンティクス**: `statuses=["受注済", "納品済"]` は「受注済 **OR** 納品済」で評価されます。

**制約**: `statuses` は最大 10 件まで（BOARD status 総数と同一）。`contract_status` との併用は**相互排他** — どちらか一方のみ指定可能です。

## alias マッピング初期値と注意点

### マッピング採用の背景

`contract_status` alias は業務ドメイン知識に基づいています。初期値は以下の設計判断に基づいています：

**採用案 (b): fields-aware filter**
- `active` / `ended` は **DeliveryStatusName のみで評価**
- `prospect` は **OrderStatusName のみで評価**
- これにより active と ended が排他になり、「契約有効 → 契約終了の段階的確認」が成立

**不採用案 (d): AND ロジック**
- `active` = `OrderStatus ∈ {受注確定,受注済} AND DeliveryStatus ∈ {未着手,着手中,納品済}`
- この案は「受注と進捗の両方が条件」という AND セマンティクスになりますが、初期実装では採用案 (b) を選択（運用に応じてレビュー予定）

### 重要な注意点

current implementation は **初期値** です。以下の点に留意してください：

- alias マッピングは業務運用と密接に関連するため、実運用フィードバックに基づいて将来変更される可能性があります
- alias マッピング自体は code 内 const として定義されており、変更の際の実装コストは最小化されています
- 定期的なレビュー（advisor 判断）で「BOARD の実務運用と乖離していないか」を確認します

## 既知の semantics（fields-aware filter の挙動）

採用案 (b) の field-aware filter により、以下のような「一見意外」な挙動が存在します：

### 例: `OrderStatus="見積中(中)"` × `DeliveryStatus="未着手"` の案件

- 見積段階（order-side）で進捗（delivery-side）の準備段階にある案件
- `contract_status="active"` 検索では **含まれます**（DeliveryStatusName="未着手" ∈ active に該当）
- `contract_status="prospect"` 検索では **含まれません**（OrderStatusName のみ見て、この案件は DeliveryStatus を持つため）

この動作は「**進捗中の案件を広く拾う**」という方針に基づいており、設計時にユーザーと合意済です。

### 他の combination は排他

- `active` と `ended` は DeliveryStatusName の値（納品済 vs 検収済）で完全に排他
- `prospect` と `active` / `ended` も OrderStatusName のみ vs DeliveryStatusName のみで排他

## narrowing 必須ルール

`contract_status` を指定する場合、**必ず以下のいずれかと組み合わせ** てください：

- `--id` — 案件 ID で直指定
- `--client-name` — 顧客名（部分一致）で絞り込み
- `--name` — 案件名（部分一致）で絞り込み
- `--text` — 全文キーワード検索

**contract_status 単独クエリは reject** されます。これは BOARD API の rate limit（3回/秒、3,000回/日）を尊重し、full-scan を避けるための設計です。

```bash
# ❌ これは reject される
board find project --contract-status active

# ✅ これは OK
board find project --contract-status active --client-name "アクメ"
```

## レビュー余地と将来拡張

### 現在のギャップ

以下の拡張は future milestone 候補です：

1. **payment_status / invoice_status alias**
   - 現在: `find invoice` / `find payment` は `--status` のみ対応
   - 将来: invoice/payment ドメインにも alias を導入

2. **order / delivery 分離フィルタ**
   - 現在: `--statuses` は OrderStatus と DeliveryStatus を OR で混在
   - 将来: `--order-statuses` / `--delivery-statuses` で分離指定

3. **alias マッピングの DB 化 / 設定ファイル化**
   - 現在: code 内 const
   - 将来: `config.toml` で組織別にカスタマイズ可能に

### AND ロジック案（記録）

検討時点では以下の AND セマンティクス案もありました：

```
active = OrderStatus ∈ {受注確定,受注済} ∧ DeliveryStatus ∈ {未着手,着手中,納品済}
```

この案は「受注と進捗の両方が条件」という厳密さがありますが、初期実装では採用案 (b)（進捗ベースで広く拾う）を選択しました。理由は以下の通り：

- **検索対象の多くが delivery-side プログレス** に基づく傾向
- **見積中×未着手のような見込み案件も有用** という業務判断
- **段階的ループ（active → ended → prospect）で十分に絞り込める** という評価

今後のユーザーフィードバック、特に false positive（想定外に多数ヒット）が報告された場合は、AND ロジック案の再評価を検討します。

---

**Last Updated**: v0.7.0 (2026-04-27)
