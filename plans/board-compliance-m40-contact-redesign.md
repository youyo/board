# M40: ContactEntity 実 API 準拠への再設計

## 概要

M39 ClientBranchEntity 再設計のパターンをそのまま ContactEntity に適用する。
実 API dump（tmp/e2e-artifacts/contacts_56292528.json）を根拠に、幻フィールドを削除し
nested client 構造と nullable フィールドを導入する。

## 実 API dump（根拠）

```json
{
  "id": 56292528,
  "client": {"id": 51285667, "name": "...", "name_disp": "...", "custom_no": ""},
  "last_name": "佐々木",
  "first_name": "昌代 ",
  "honorific_title": "様",
  "title": null,
  "department": null,
  "email": null,
  "note": null,
  "archive_flg": 0,
  "created_at": "2015-02-03T17:13:03.000+09:00",
  "updated_at": "2015-02-03T17:13:03.000+09:00"
}
```

## 削除フィールド（幻フィールド）

| フィールド | 旧型 | 理由 |
|-----------|------|------|
| `ClientID` | `int` (json:"client_id") | 実 API に不在。nested client.id に置換 |
| `ClientBranchID` | `int` (json:"client_branch_id") | 実 API に不在 |
| `Name` | `string` (json:"name") | 実 API に不在 |
| `NameKana` | `string` (json:"name_kana") | 実 API に不在 |
| `Phone` | `string` (json:"phone") | 実 API に不在 |
| `Memo` | `string` (json:"memo") | 実 API に不在 |

## 新フィールド

| フィールド | 型 | json タグ |
|-----------|-----|-----------|
| `Client` | `*ClientRef` | `json:"client"` |
| `LastName` | `string` | `json:"last_name"` |
| `FirstName` | `string` | `json:"first_name"` |
| `HonorificTitle` | `string` | `json:"honorific_title"` |
| `Title` | `*string` | `json:"title"` |
| `Department` | `*string` | `json:"department"` |
| `Email` | `*string` | `json:"email"` |
| `Note` | `*string` | `json:"note"` |
| `ArchiveFlg` | `int` | `json:"archive_flg"` |
| `CreatedAt` | `string` | `json:"created_at"` |
| `UpdatedAt` | `string` | `json:"updated_at"` |

## 追加 accessor / メソッド

- `ClientID() int` — nested Client.ID を返す。Client が nil の場合 0。
- `DisplayName() string` — LastName + FirstName を結合（Name フィールド廃止のため単純化）。

## 修正ファイル一覧

### 本体
- `internal/boardapi/contacts.go` — ContactEntity 再定義

### repository
- `internal/repository/contacts.go` — filterContactsByNameEmail を DisplayName() + *string nil ガード対応

### テスト
- `internal/boardapi/client_test.go` — T55/T56/T57 の mock JSON を新形式に更新
- `internal/boardapi/contacts_test.go` — U1-U5 の mock JSON を新形式に更新、wantKeys を 12 件に更新
- `internal/boardapi/e2e_contacts_test.go` — Logf フィールド名を新形式に更新（幻フィールド削除）
- `internal/boardapi/display_name_test.go` — Name フィールドを使うケースを削除、LastName/FirstName ケースに整理
- `internal/repository/contacts_test.go` — sampleContacts を新形式に更新（Client/LastName/FirstName/Email*string）、アサーション修正
- `internal/service/api/service_test.go` — ContactEntity リテラルを新形式に更新
- `internal/service/find/find_client_test.go` — contacts スタブを新形式に更新
- `internal/service/find/e2e_test.go` — `c.ClientID` → `c.ClientID()` 3 箇所

## ステータス

✅ 完了（2026-04-21）
- go build ./... OK
- go build -tags e2e ./... OK
- go vet ./... OK
- go test -count=1 ./... 全 Green（全 12 パッケージ）
- 実 API smoke: ⏳ 429 Rate Limit のため日次リセット後に再実行予定

## M41 向け申し送り

次は VendorBranchEntity の実 API 準拠再設計（M41）。
同様に nested vendor 構造が予想される。`ClientRef` と同型の `VendorRef` が必要になる可能性あり。
実 API dump が tmp/e2e-artifacts/ にある場合はそれを根拠にする。
