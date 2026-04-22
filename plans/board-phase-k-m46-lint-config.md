# M46: golangci-lint + .editorconfig 導入

## Meta
| 項目 | 値 |
|------|---|
| ステータス | 完了 ✅ |
| 完了日 | 2026-04-22 |
| 目的 | Go コードの静的解析を自動化し、コーディング規約を強制する |
| 前提 | M45（ProjectCostEntity 再設計）完了 |
| 次のマイルストーン | M47 ユーザー向けドキュメント一括整備 |

## 実施内容

### 作成ファイル
- `.editorconfig` — インデント・改行・文字コード規約の統一
- `.golangci.yml` — golangci-lint v2 設定（errcheck/govet/staticcheck/ineffassign/unused/gofmt/goimports）

### 変更ファイル
- `.github/workflows/ci.yml` — golangci-lint-action@v6 ステップを追加（go vet 後、go test 前）
- `mise.toml` — `lint` タスクを追加（`golangci-lint run`）

### lint 違反解消
検出された違反: 15 件
- **gofmt（3件）**: `gofmt -s -w .` で自動修正（client_branches.go, clients.go, project_costs.go のタグアライメント）
- **errcheck（8件）**: `_, _ =` を付けて明示的に無視（CLI の stdout/stderr 書き込みは失敗しても継続する設計）
  - cmd/board/main.go: json.Encode ×2
  - internal/cli/cache.go: fmt.Fprintf/Fprintln ×4
  - internal/cli/configure.go: fmt.Fprintf/Fprintln ×3
  - internal/cli/configure_current_profile.go: fmt.Fprintln ×1
  - internal/cli/configure_get.go: fmt.Fprintln ×1
  - internal/cli/configure_list_profiles.go: fmt.Fprintln ×1
  - internal/cli/configure_path.go: fmt.Fprintln ×1
  - internal/cli/configure_show.go: fmt.Fprintln ×1
  - internal/cli/mcp.go: fmt.Fprintf ×1
- **staticcheck QF1001（4件）**: De Morgan's law 適用（`!(A && B)` → `(A || B)`）
  - internal/service/find/find_invoice.go
  - internal/service/find/find_payment.go
  - internal/service/find/find_project.go
  - internal/service/find/find_purchase_order.go
- **staticcheck S1000（1件）**: single case select を `<-ch //nolint:staticcheck` に変換
  - internal/boardapi/client_test.go

最終結果: **0 issues**

## 検証結果

| チェック | 結果 |
|---------|------|
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `go test -count=1 ./...` | PASS（12 パッケージ全 Green） |
| `golangci-lint run ./...` | 0 issues |

## コミット一覧

| hash | 種別 | 内容 |
|------|------|------|
| 6708a80 | chore | .editorconfig を追加 |
| 25093ee | chore | .golangci.yml を追加 |
| c106001 | chore | mise.toml に lint task 追加 |
| 2e16f3a | ci | GitHub Actions に golangci-lint step 追加 |
| 3f97de8 | style | gofmt -s 適用 |
| 5b58570 | fix | golangci-lint 違反を解消（12件） |

## 設計メモ

- golangci-lint は `go install` 経由で v2.11.4 をインストール（Homebrew のパーミッション問題のため）
- CI では `golangci/golangci-lint-action@v6` を利用（バイナリ版自動インストール）
- errcheck の `_, _ =` パターンは CLI 慣習に準拠。Close/Write 系は設定の exclusion rules でカバー
- staticcheck の `-ST1000` 除外は「パッケージコメント必須」を緩和（Go 標準では任意）
