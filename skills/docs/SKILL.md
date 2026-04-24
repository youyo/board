---
name: board:docs
description: >-
  Look up BOARD CLI command usage, flag references, Ransack filters, and API
  resource schemas via the embedded `board docs` subcommand — offline and
  always in sync with the installed binary.

  Use this skill whenever the user asks how to use `board api <resource>`,
  which flags are available, how a BOARD resource is shaped, or needs to
  discover what the `board` binary can do.

  Common scenarios:
  - "How do I list clients with filters?" / "board api clients の使い方"
  - "What does --response-group do?" / "どんな Ransack フィルタが使える？"
  - "Show me the invoices resource schema" / "invoices リソースの仕様"
  - "Which resources does BOARD expose?" / "BOARD のリソース一覧"

  Prefer this skill over web search or training-data recall when accuracy
  matters — the documentation ships inside the installed binary and reflects
  its exact version.
---

# /board:docs

`board docs` サブコマンドを呼び出す薄いラッパー。
一次情報は board バイナリに埋め込まれたドキュメントに集約されており、本ファイルは呼び出し手順だけを示す。

## Prerequisites

`board` バイナリが PATH に存在すること。未インストール時の代替:

- Homebrew: `brew install youyo/tap/board`
- ソースから: `go run ./cmd/board docs ...`（本リポジトリ内で実行する場合）

## Workflow

### 1. 目次を取得する

```sh
board docs --list
```

埋め込まれたドキュメントの一覧（相対パスとバイトサイズ）を返す。まずここから始めて、対象ファイルを決める。

### 2. API リソースの詳細を調べる

```sh
board docs <resource> --format json
```

`--format json` を付けると LLM / MCP 向けの機械可読な出力 `{mode, query, results[]}` が得られる。
例として `board docs clients --format json` を実行すると `api-reference.md` 内の該当節が抽出される。

### 3. キーワード横断検索

```sh
board docs --search "Ransack" --format json
```

全埋め込みファイルを大文字小文字無視で検索し、マッチ行と前後 2 行のコンテキスト、直近の `####` 見出し名を返す。

### 4. README や全文を読む

```sh
board docs
```

引数なしで実行すると README が表示される。ガイド類は `board docs --list` で得たパスを引数に取ることで閲覧可能。

## Output format (JSON)

`--format json` を付けた場合のスキーマ:

```json
{
  "mode": "readme | list | search | resource",
  "query": "<keyword or resource name; omitted for list/readme>",
  "results": [
    {
      "file": "api-reference.md",
      "section": "clients",
      "content": "...",
      "line": 27,
      "size": 15479
    }
  ]
}
```

`results[]` の各フィールドは mode により出現有無が変わる（`omitempty`）。
最新スキーマは `board docs --search "JSON 出力スキーマ" --format json` で取得するのが確実。

## Best practices for LLMs

- **JSON モードを優先**: `--format json` で構造化出力を受けると parse が安定する
- **リソース名をハードコードしない**: `board docs --list` → `board docs --search "####"` で動的に確認できる
- **エラー時は stderr 参照**: リソース未存在時は非ゼロ exit + stderr に `section not found` 形式

## Troubleshooting

| 症状 | 対処 |
|------|------|
| `command not found: board` | `brew install youyo/tap/board` もしくはリポジトリ内で `go run ./cmd/board docs ...` |
| `section not found: <name>` | `board docs --list` で利用可能なドキュメント一覧を確認、`board docs --search <name>` で候補探索 |
| `unsupported format: xml` | `--format` は `text` / `json` のみ対応 |
| 結果が古い | `brew upgrade board` でバイナリ更新（埋め込みドキュメントは各リリース時点のスナップショット） |

## 関連

- `board --help` — ルートコマンドヘルプ
- `board api --help` — low-level API 操作
- `board find --help` — LLM 向け横断検索
- `board mcp serve` — MCP サーバー常駐
