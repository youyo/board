# Plan: CLI エラー出力の改善 — JSON + ヒント付きメッセージ

## Context

`board` CLI のエラー出力が不親切:
- プレーンテキスト `boardapi error [FORBIDDEN] status=403: Forbidden` → JSON にしたい
- Cobra の Usage/Help テキストが表示される → 不要
- 403 が「権限不足」なのか「プラン制限」なのかわからない
- 429 が「秒間制限」なのか「日次制限」なのかわからない

complexity: L

## 修正方針

### Step 1: Cobra の SilenceUsage / SilenceErrors を設定

`internal/cli/root.go`:
```go
cmd.SilenceUsage = true
cmd.SilenceErrors = true
```

### Step 2: APIError にヒントメッセージを追加

`internal/boardapi/error.go` に `Hint()` メソッドを追加:

```go
func (e *APIError) Hint() string {
    switch e.Code {
    case APIErrorUnauthorized:
        return "APIキーまたはAPIトークンが無効です。`board configure` で設定を確認してください。"
    case APIErrorForbidden:
        return "このリソースへのアクセス権限がありません。BOARD管理画面（設定→API）でAPIキーの権限を確認してください。"
    case APIErrorNotFound:
        return "指定されたリソースが見つかりません。IDが正しいか確認してください。"
    case APIErrorRateLimit:
        if e.RetryAfter > 0 {
            return fmt.Sprintf("APIレート制限に達しました。%s後に再試行してください。", e.RetryAfter)
        }
        return "APIレート制限に達しました（日次上限3000リクエスト、または秒間3リクエスト）。しばらく待ってから再試行してください。"
    case APIErrorValidation:
        return "リクエストパラメータが不正です。"
    case APIErrorTemporary:
        return "BOARD APIサーバーで一時的なエラーが発生しています。しばらく待ってから再試行してください。"
    case APIErrorNetwork:
        return "BOARD APIサーバーに接続できません。ネットワーク接続を確認してください。"
    default:
        return ""
    }
}
```

### Step 3: main.go でエラーを JSON 出力

`cmd/board/main.go`:

```go
if err := rootCmd.Execute(); err != nil {
    var apiErr *boardapi.APIError
    if errors.As(err, &apiErr) {
        result := map[string]interface{}{
            "error":       true,
            "code":        string(apiErr.Code),
            "status_code": apiErr.StatusCode,
            "message":     apiErr.Message,
        }
        if hint := apiErr.Hint(); hint != "" {
            result["hint"] = hint
        }
        if apiErr.RetryAfter > 0 {
            result["retry_after_seconds"] = int(apiErr.RetryAfter.Seconds())
        }
        json.NewEncoder(os.Stderr).Encode(result)
    } else {
        json.NewEncoder(os.Stderr).Encode(map[string]interface{}{
            "error":   true,
            "message": err.Error(),
        })
    }
    os.Exit(1)
}
```

### 期待する出力例

**403 Forbidden:**
```json
{"code":"FORBIDDEN","error":true,"hint":"このリソースへのアクセス権限がありません。BOARD管理画面（設定→API）でAPIキーの権限を確認してください。","message":"Forbidden","status_code":403}
```

**429 Rate Limit (日次):**
```json
{"code":"RATE_LIMIT","error":true,"hint":"APIレート制限に達しました（日次上限3000リクエスト、または秒間3リクエスト）。しばらく待ってから再試行してください。","message":"Limit Exceeded","status_code":429}
```

**429 Rate Limit (Retry-After あり):**
```json
{"code":"RATE_LIMIT","error":true,"hint":"APIレート制限に達しました。60s後に再試行してください。","message":"Limit Exceeded","retry_after_seconds":60,"status_code":429}
```

**401 Unauthorized:**
```json
{"code":"UNAUTHORIZED","error":true,"hint":"APIキーまたはAPIトークンが無効です。`board configure` で設定を確認してください。","message":"Unauthorized","status_code":401}
```

**ネットワークエラー:**
```json
{"code":"NETWORK","error":true,"hint":"BOARD APIサーバーに接続できません。ネットワーク接続を確認してください。","message":"...","status_code":0}
```

**非APIエラー (フラグ不正等):**
```json
{"error":true,"message":"unknown flag: --foo"}
```

## 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `internal/cli/root.go` | SilenceUsage + SilenceErrors 設定 |
| `internal/boardapi/error.go` | `Hint()` メソッド追加 |
| `cmd/board/main.go` | JSON エラー出力 + hint/retry_after フィールド |

## 検証

1. `go build ./...` でビルド確認
2. `go run ./cmd/board/ api users list` → 403 JSON + hint
3. 不正フラグ `go run ./cmd/board/ api users list --foo` → 汎用 JSON エラー、Usage なし
4. `go test ./...` で既存テスト通過
