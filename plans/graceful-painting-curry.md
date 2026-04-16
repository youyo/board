# fix: UserEntity / ContactEntity のフィールドマッピング修正

## Context

BOARD API の `/v1/users` は `name` フィールドを返さず、`last_name` + `first_name` を返す。
現在の `UserEntity` は `json:"name"` のみでマッピングしているため、`Name` が常に空文字になるバグがある。

同様に `/v1/contacts` も `name` ではなく `last_name` + `first_name` + `honorific_title` を返す。

ただし、これは OSS であり、他の組織では `name` フィールドが使われている可能性がある。
両パターンに対応する設計が必要。

### 実際の API レスポンス

**users:**
```json
{"id":38516996, "email":"tachibana@heptagon.co.jp", "last_name":"立花", "first_name":"拓也", "role_id":1, "role_name":"管理者", "last_sign_in_at":"2026-04-15T09:44:50.000+09:00", "valid_flg":1, "created_at":"...", "updated_at":"..."}
```

**contacts:**
```json
{"id":56292528, "client":{"id":51285667, "name":"株式会社Ｊサポート", "name_disp":"Ｊサポート", "custom_no":""}, "last_name":"佐々木", "first_name":"昌代", "honorific_title":"様", "title":null, "department":null, "email":null, "note":null, "archive_flg":0, ...}
```

**vendor_contacts:** エンドポイント 404 のため検証不可。contacts と同構造と仮定。

## 方針

`Name` フィールドを維持しつつ `LastName`/`FirstName` を追加。`DisplayName()` メソッドで統一的に名前を取得。

## Step 1: UserEntity 拡張

**ファイル:** `internal/boardapi/users.go`

```go
type UserEntity struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	LastName       string `json:"last_name"`
	FirstName      string `json:"first_name"`
	Email          string `json:"email"`
	RoleID         int    `json:"role_id"`
	RoleName       string `json:"role_name"`
	LastSignInAt   string `json:"last_sign_in_at"`
	ValidFlg       int    `json:"valid_flg"`
	UpdatedAt      string `json:"updated_at"`
	CreatedAt      string `json:"created_at"`
}

// DisplayName returns a human-readable name.
// Prefers Name if set, otherwise combines LastName + FirstName.
func (u UserEntity) DisplayName() string {
	if u.Name != "" {
		return u.Name
	}
	// last_name + " " + first_name（日本語名: 姓名）
	// どちらかが空でも対応
	switch {
	case u.LastName != "" && u.FirstName != "":
		return u.LastName + " " + u.FirstName
	case u.LastName != "":
		return u.LastName
	case u.FirstName != "":
		return u.FirstName
	default:
		return ""
	}
}
```

## Step 2: ContactEntity 拡張

**ファイル:** `internal/boardapi/contacts.go`

```go
type ContactEntity struct {
	ID              int    `json:"id"`
	ClientID        int    `json:"client_id"`
	ClientBranchID  int    `json:"client_branch_id"`
	Name            string `json:"name"`
	NameKana        string `json:"name_kana"`
	LastName        string `json:"last_name"`
	FirstName       string `json:"first_name"`
	HonorificTitle  string `json:"honorific_title"`
	Title           string `json:"title"`
	Department      string `json:"department"`
	Email           string `json:"email"`
	Phone           string `json:"phone"`
	Note            string `json:"note"`
	Memo            string `json:"memo"`
	ArchiveFlg      int    `json:"archive_flg"`
	UpdatedAt       string `json:"updated_at"`
	CreatedAt       string `json:"created_at"`
}

func (c ContactEntity) DisplayName() string { /* Name 優先、なければ LastName+FirstName */ }
```

既存フィールド（`Name`, `NameKana`, `ClientID`, `ClientBranchID`, `Phone`, `Memo`）は残す。
API が返さないフィールドはゼロ値のまま。他の組織では返す可能性がある。

## Step 3: VendorContactEntity 拡張

**ファイル:** `internal/boardapi/vendor_contacts.go`

ContactEntity と同様に `LastName`, `FirstName`, `HonorificTitle`, `Department`, `Note`, `ArchiveFlg` を追加。
`DisplayName()` メソッドを追加。

## Step 4: find service の Name 参照を DisplayName() に変更

**ファイル:** `internal/service/find/find_user.go`

```go
// L42: containsText でテキスト検索する際、DisplayName() を使う
// 加えて LastName, FirstName も個別に検索対象にする
containsText(q.Text, u.DisplayName(), u.LastName, u.FirstName, u.Email)
```

## Step 5: 単体テスト更新

**ファイル:** `internal/service/find/find_user_test.go`

- テストデータの `Name: "User A"` はそのまま（Name パターンのテスト）
- `LastName`/`FirstName` パターンのテストケースを追加
- `DisplayName()` の単体テストを `internal/boardapi/users_test.go` に追加（Name のみ / LastName+FirstName / 両方 / 空）

## Step 6: E2E テスト更新

**ファイル:**
- `internal/boardapi/e2e_test.go` L108: `requireNonEmpty(t, got.Name, ...)` → `requireNonEmpty(t, got.DisplayName(), ...)`
- `internal/service/find/e2e_test.go` L104: `u.Name != ""` → `u.DisplayName() != ""`

## Step 7: boardapi client_test.go のモック JSON 更新

`UserEntity` のテストで使われているモック JSON に `last_name`/`first_name` を追加。
既存の `name` フィールドのテストも残す（両パターン対応の検証）。

## 対象外（今回のスコープ外）

- ContactEntity の nested `client{}` オブジェクト対応（別途対応）
- CLI/MCP の `--name` フラグのリネーム（API 検索パラメータ `?name=` は引き続き有効と想定）

## 検証

```bash
go test -p 1 ./internal/boardapi/ ./internal/service/find/ ./internal/cli/
go vet ./...
# E2E（レートリミット回復後）
go test -p 1 -tags e2e -run "E2E_Users|E2E_FindUser" -count=1 ./internal/boardapi/ ./internal/service/find/
```
