# Plan: BOARD_API_KEY / BOARD_API_TOKEN 環境変数オーバーライド

## Context

BOARD API の認証情報（api_key, api_token）は現在 config.toml にのみ保存される。
CI/CD や秘密管理ツール連携、一時的なプロファイル切り替えのために、
環境変数 `BOARD_API_KEY` / `BOARD_API_TOKEN` で config.toml の値を上書きできるようにする。

## 設計方針

config パッケージに `ApplyEnvOverrides` 関数を新設し、`app.New()` 内で `ApplyDefaults` の直後に呼ぶ。

**優先順位（低→高）:**
1. `DefaultProfileConfig()` デフォルト値
2. config.toml の値（`Load`）
3. `ApplyDefaults()` でゼロ値フィールド補完
4. **`ApplyEnvOverrides()` で環境変数上書き**（新規）
5. `boardapi.New()` に渡される

**空文字列の扱い:** `os.Getenv` は未設定・空設定とも `""` を返す。`v != ""` ガードにより、どちらの場合も上書きしない。

## 変更ファイル

### 1. `internal/config/profile.go` — `ApplyEnvOverrides` 追加

- import に `"os"` 追加
- `ApplyDefaults` の後（L60 以降）に関数追加:

```go
// ApplyEnvOverrides overrides ProfileConfig fields with environment variable values.
// Only non-empty environment variable values are applied.
//
// Supported environment variables:
//   - BOARD_API_KEY: overrides APIKey
//   - BOARD_API_TOKEN: overrides APIToken
func ApplyEnvOverrides(p ProfileConfig) ProfileConfig {
	if v := os.Getenv("BOARD_API_KEY"); v != "" {
		p.APIKey = v
	}
	if v := os.Getenv("BOARD_API_TOKEN"); v != "" {
		p.APIToken = v
	}
	return p
}
```

### 2. `internal/app/app.go` — 呼び出し追加

`ApplyDefaults` の直後（L57 付近）に1行追加:

```go
prof = config.ApplyDefaults(prof)
prof = config.ApplyEnvOverrides(prof)  // 環境変数で上書き
```

### 3. `internal/config/config_test.go` — ユニットテスト

`t.Setenv` を使用して3パターンをテスト:
- 両方設定 → 両方上書き
- 片方のみ設定 → 片方だけ上書き
- 両方未設定 → config.toml の値を維持

### 4. `internal/app/app_test.go` — 統合テスト

`app.New("")` 経由で環境変数が `boardapi.Client` まで伝搬することを確認。

## 検証手順

```bash
mise run test                    # 全テスト通過
mise run vet                     # 静的解析

# 手動確認
export BOARD_API_KEY="test-key"
export BOARD_API_TOKEN="test-token"
./board api clients list         # 環境変数の認証情報で API アクセス
```
