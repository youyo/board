# XDG パスを Linux スタイルに統一

## Context

README に「platform XDG config path」と記載されているが、実際には Linux スタイルの XDG パスに統一したい。
現在の実装は既に `~/.config/` と `~/.local/share/` を使用しているが、以下の問題がある:

1. `os.UserConfigDir()` / `os.UserCacheDir()` のプラットフォーム依存フォールバックがデッドコードとして残存
2. cache.db が `~/.local/share/`（XDG_DATA_HOME）にあるが、再生成可能なキャッシュなので `~/.cache/`（XDG_CACHE_HOME）が適切
3. README の記述が曖昧

## 変更内容

### 1. `internal/config/path.go` — ConfigPath() の簡素化

解決順序を以下に変更:
1. `BOARD_CONFIG_PATH` 環境変数
2. `XDG_CONFIG_HOME/board/config.toml`
3. `$HOME/.config/board/config.toml`
4. `$TMPDIR/board/config.toml`（fallback）

- `os.UserConfigDir()` のステップを削除
- コメントのステップ番号を更新

### 2. `internal/app/app.go` — dbPath() の変更

解決順序を以下に変更:
1. `BOARD_CACHE_PATH` 環境変数
2. `XDG_CACHE_HOME/board/cache.db`（旧: `XDG_DATA_HOME`）
3. `$HOME/.cache/board/cache.db`（旧: `$HOME/.local/share/board/cache.db`）
4. `$TMPDIR/board/cache.db`（fallback）

- 環境変数を `XDG_DATA_HOME` → `XDG_CACHE_HOME` に変更
- パスを `~/.local/share/` → `~/.cache/` に変更
- `os.UserCacheDir()` のステップを削除

### 3. `internal/config/config_test.go` — テスト更新

- `TestConfigPathXDG` — 変更なし
- `TestConfigPathHome` — 変更なし（既に `.config` を期待）
- dbPath のテストがあれば更新（XDG_CACHE_HOME、~/.cache/ を期待するように）

### 4. README.md / README_ja.md — パス記述の明確化

Line 141 付近の記述を具体的なパスに変更:
- Config: `~/.config/board/config.toml`
- Cache: `~/.cache/board/cache.db`
- 環境変数 `XDG_CONFIG_HOME` / `XDG_CACHE_HOME` でオーバーライド可能と明記

### 5. `docs/specs/board_cli_mcp_ultra_detailed_design_ja.md` — スペック更新

Line 560-563 のパス記述を更新:
- `~/.local/share/board/cache.db` → `~/.cache/board/cache.db`

## 対象ファイル

- `internal/config/path.go`
- `internal/app/app.go` (dbPath 関数)
- `internal/config/config_test.go`
- `README.md`
- `README_ja.md`
- `docs/specs/board_cli_mcp_ultra_detailed_design_ja.md`

## 検証

```bash
mise run test     # 全テスト通過
mise run vet      # 静的解析通過
```

手動確認:
- `BOARD_CONFIG_PATH` 未設定時に `~/.config/board/config.toml` を返すこと
- `XDG_CACHE_HOME` 設定時にそのパス配下を返すこと
- `BOARD_CACHE_PATH` 未設定時に `~/.cache/board/cache.db` を返すこと
