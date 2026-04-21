# M39: ClientBranchEntity 実 API 準拠への再設計

## 概要

M09 で発見した「nested client 構造バグ」＋「フィールド全面不一致」を修正し、
`ClientBranchEntity` を実 API レスポンスに完全準拠した構造に書き換える。

## 前提: 実 API スモーク結果（M09 確認済み）

`tmp/e2e-artifacts/client_branches_195311.json` の実体:
```json
{
  "id": 195311,
  "client": {"id": 51346407, "name": "株式会社テクノサポートカンパニー", "name_disp": "...", "custom_no": ""},
  "name": "東京支社",
  "zip": "103-0025",
  "pref": "東京都",
  "address1": "中央区日本橋茅場町1-11-9",
  "address2": "山本ビル2F",
  "tel": null,
  "fax": null,
  "archive_flg": 0,
  "created_at": "2017-07-14T09:22:21.000+09:00",
  "updated_at": "2017-07-14T09:22:21.000+09:00"
}
```

## 旧 ClientBranchEntity（修正前）

```go
type ClientBranchEntity struct {
    ID         int    `json:"id"`
    ClientID   int    `json:"client_id"`   // 幻フィールド（API は client.id nested）
    Name       string `json:"name"`
    PostalCode string `json:"postal_code"` // 幻（API は "zip"）
    Address    string `json:"address"`     // 幻（API は "address1"+"address2"）
    Phone      string `json:"phone"`       // 幻（API は "tel"）
    Fax        string `json:"fax"`
    Memo       string `json:"memo"`        // 幻（API に存在しない）
    UpdatedAt  string `json:"updated_at"`
    CreatedAt  string `json:"created_at"`
}
```

id/name/fax/updated_at/created_at の 5 フィールドのみ正しい。残り 5 は幻。
さらに API 側キー `client(nested)/zip/pref/address1/address2/tel/archive_flg` が未マップ。

## 新 ClientBranchEntity（修正後）

### ClientRef 共通型（client_ref.go 新規）

```go
type ClientRef struct {
    ID       int    `json:"id"`
    Name     string `json:"name"`
    NameDisp string `json:"name_disp"`
    CustomNo string `json:"custom_no"`
}
```

### ClientBranchEntity 新定義

```go
type ClientBranchEntity struct {
    ID         int        `json:"id"`
    Client     *ClientRef `json:"client"`
    Name       string     `json:"name"`
    Zip        string     `json:"zip"`
    Pref       string     `json:"pref"`
    Address1   string     `json:"address1"`
    Address2   string     `json:"address2"`
    Tel        *string    `json:"tel"`
    Fax        *string    `json:"fax"`
    ArchiveFlg int        `json:"archive_flg"`
    CreatedAt  string     `json:"created_at"`
    UpdatedAt  string     `json:"updated_at"`
}

func (e ClientBranchEntity) ClientID() int {
    if e.Client == nil { return 0 }
    return e.Client.ID
}
```

## 実施タスク

| # | タスク | 状態 |
|---|--------|------|
| 1 | ロードマップに M39 を追加 | ✅ |
| 2 | `internal/boardapi/client_ref.go` 新規作成 | ✅ |
| 3 | `internal/boardapi/client_branches.go` Entity 全面書き換え | ✅ |
| 4 | downstream 修正（client_test.go / client_branches_test.go / repository / find） | ✅ |
| 5 | E2E テスト更新（e2e_client_branches_test.go）| ✅ |
| 6 | go build/vet/test 全 Green | ✅ |
| 7 | 実 API smoke | 429 Rate Limit（日次リセット後に再実行） |
| 8 | plan ファイル作成 | ✅ |
| 9 | ロードマップ更新 | ✅ |
| 10 | コミット 3 件 | ✅ |

## downstream 修正箇所一覧

| ファイル | 変更内容 |
|---------|---------|
| `internal/boardapi/client_ref.go` | 新規作成: ClientRef 型 |
| `internal/boardapi/client_branches.go` | Entity 全面書き換え + ClientID() accessor 追加 |
| `internal/boardapi/client_branches_test.go` | U1-U5 モック JSON を新スキーマに更新 |
| `internal/boardapi/client_test.go` | T52/T53/T54 モック JSON を新スキーマに更新、T52 の `result[0].ClientID` → `result[0].ClientID()` |
| `internal/boardapi/e2e_client_branches_test.go` | Get テストのログ行を新フィールド名に更新 |
| `internal/repository/client_branches_test.go` | sampleClientBranches を新スキーマに更新、T_R26 の target を更新 |
| `internal/service/find/find_client_test.go` | branches 構造体リテラルを新スキーマに更新 |
| `internal/service/find/e2e_test.go` | `b.ClientID` → `b.ClientID()`（3 箇所、contact 側は M40 まで保留） |

## 実測値

- build: ✅ clean（通常 + `-tags e2e`）
- vet: ✅ clean
- test: ✅ 全 12 パッケージ Green（go test -count=1 ./...）
- e2e smoke: ⏳ 429 Rate Limit（日次リセット後に再実行）

## 申し送り: M40 ContactEntity 再設計

M10 で確認した ContactEntity の問題（nested client + 逆方向不整合 6 フィールド）は M40 で対応予定。
`ClientRef` 型は M39 で定義済みのため M40 でそのまま再利用可能。

M40 の主な変更予定:
- `ContactEntity.ClientID int` → `Contact.Client *ClientRef`（M39 と同パターン）
- `ContactEntity.ClientBranchID` 削除（API レスポンスに不在）
- `ContactEntity.Name / NameKana / Memo / Phone` 削除（逆方向不整合）
- `ContactEntity.Note` 追加（`memo` の代替が `note`）
- `ContactEntity.ClientID()` accessor 追加
- find/e2e_test.go の `c.ClientID` → `c.ClientID()` 置換
