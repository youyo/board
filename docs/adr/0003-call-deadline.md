# ADR 0003: 呼び出しの締め切り 30 秒

- 状態: 採用(M02 追補、2026-11-05。M06 で文書化)
- 関連: ADR 0002(再開できる refresh)、ADR 0004(逆引き)、`plans/board-workers-m02-board-client.md`

## 背景

Go 版の BOARD クライアントは CLI 前提で、429 / 5xx に最大 5 回再試行し、`Retry-After` やバックオフを待っていた。
そのまま使うと 1 回の get が最悪 300〜480 秒待ち、MCP クライアントのタイムアウトを超える。
Workers でも、クライアントが切断したあとに BOARD を叩き続けるのは無駄で、rate limit も食う(M02 の verifier 指摘)。

## 決定

- **1 回のツール呼び出しにつき 1 つの呼び出しコンテキスト(`signal` + `deadline`、既定 30 秒)を作り**、一覧のページ巡回全体で共有する(`src/board/context.ts` `src/mcp/call-context.ts`)。
  `signal` は MCP のリクエストの abort(クライアントの切断)で、BOARD への fetch・待機・キュー待ちまで渡す。
- 締め切りは `Date.now()` を基準にする(BOARD クライアントの残り時間の判定と食い違わないように。ツールの時計 `ctx.now()` は使わない)。
- `Retry-After` やバックオフの待ちが残り時間を超えるなら、**待たずに `retryable: true` で失敗させる**。`maxRetries` は 3(Go の 5 は CLI 前提)。
- クライアントは isolate 内で共有し、リクエストをまたいで BOARD への通信を直列化する(rate limit 3 回/秒のため)。
  この直列化は 1 つの isolate の中でしか効かない(`src/board/client.ts` の `serialize` は isolate 内の Promise 連鎖)。Workers は複数の isolate を同時に動かしうるので、isolate をまたぐと合計が 3 回/秒を超えることがある。超えた分は BOARD が 429(`rate_limited`)で返し、`Retry-After` に従う再試行(上記)で吸収する。これは設計どおりで、全体制御が要るほど問題になったら Durable Objects を再検討する(ロードマップの決定済み事項)。
- 時間内に終わらない処理(full refresh、逆引きの走査)は、締め切りの手前で止めて**進捗を保存し、途中経過を返す**(ADR 0002・0004)。
  逆引きの走査は `SCAN_RESERVE_MS`(8 秒)を残して止め、その後の案件・顧客の取得に時間を残す。

## 却下した案

- **Go と同じ再試行(5 回、待ちは無制限)**: MCP クライアントのタイムアウトを超える。
- **締め切りなし(Workers の CPU 時間・リクエスト時間に任せる)**: 切断後も BOARD を叩き続け、rate limit を食う。

## 帰結

- 429 が続くと `retryable: true` で失敗する。LLM は少し待って再試行する(`refresh_in_progress` の `retry_after_seconds` と同じ扱い)。
- 30 秒は MCP クライアント側のタイムアウト(多くは 60 秒以上)より前に返すための値。実機の 1 ページあたりの時間は `mise run test:live` で測る(docs/manual-verification.md)。
- MCP の abort が切断で発火するかは実機でしか確かめられない(docs/manual-verification.md)。発火しなくても締め切りで必ず止まる。
