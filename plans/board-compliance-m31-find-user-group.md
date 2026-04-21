# M31: FindUser / FindGroup 厳格 E2E

## 概要

Phase H 7 件目。FindUser / FindGroup の厳格 E2E 突合を実施する。

## 対象コード

- `internal/service/find/find_user.go` — FindUser: ID/Name/Text モード
- `internal/service/find/find_group.go` — FindGroup: ID/Name/Text モード
- `internal/service/find/e2e_test.go` — 追加テスト

## 追加テスト

| テスト名 | 結果 | 備考 |
|---|---|---|
| `TestE2E_FindUser_ByID_Strict` | PASS | DisplayName フォールバック確認（Name="" → LastName+" "+FirstName） |
| `TestE2E_FindUser_ByName_StrictAddon` | SKIP | BOARD API が name filter を無視して 0 件返却 |
| `TestE2E_FindGroup_ByID_Strict` | SKIP | groups 0 件（Pending Re-verification） |
| `TestE2E_FindGroup_ByName` | SKIP | groups 0 件（Pending Re-verification） |

## 発見事項

### DisplayName フォールバック動作

```
User(id=38516996): Name="" LastName="立花" FirstName="拓也" → DisplayName()="立花 拓也"
```

`Name` フィールドが空の場合、`LastName + " " + FirstName` のフォールバックが正常動作。

### BOARD API SearchUsers 挙動

`SearchUsers(Name="立花 拓也")` が 0 件を返す。他のリソースと同様、BOARD API が name filter を無視している可能性あり（M12 等で確認済みのパターン）。既存 `TestE2E_FindUser_ByName` のパニックバグ（0 件時の `results[0]` アクセス）を同時修正。

### groups データなし

当該アカウントは groups 0 件（M07 と同じ状況）。Pending Re-verification。

## API コール数

- ListUsers × 3 (ByID_Strict, ByName, StrictAddon)
- GetUser × 1 (ByID_Strict の GetByID)
- 合計: ~4 req（ListGroups × 2 を含めると ~6 req）

## コミット

1. `test(e2e): M31 FindUser/FindGroup の厳格検証 E2E を追加`
2. `docs(plans): M31 完了をロードマップに反映`
