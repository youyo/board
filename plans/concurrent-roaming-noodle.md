# CI テストワークフロー追加プラン

## Context

`.goreleaser.yaml` の `before.hooks` から `go test ./...` を除外したため、CI でテスト・lint を実行するワークフローが必要。

## 作成ファイル

### `.github/workflows/ci.yml`

- **トリガー**: push (main) + pull_request (main)
- **OS**: ubuntu-latest
- **Go**: go-version-file: go.mod（mise.toml と同期不要）
- **ステップ**:
  1. checkout
  2. setup-go (with cache)
  3. `go vet ./...`
  4. `go test ./...`

シンプルに `go vet` + `go test` のみ。golangci-lint 等は現時点では不要（mise.toml にも定義なし）。

## 検証

- YAML の構文確認（`actionlint` があれば使用）
- push/PR 時に GitHub Actions で動作確認
