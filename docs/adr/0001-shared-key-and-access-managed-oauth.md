# ADR 0001: 共有キー + Access Managed OAuth

- 状態: 採用(M01、2026-11-05。M06 で文書化)
- 関連: ADR 0002(D1 キャッシュ)、`docs/runbook.md`、姉妹リポジトリ logbutler の ADR 0009(Access 連携の削除)

## 背景

BOARD API の認証は、リクエストごとのヘッダー `x-api-key`(API キー)と `Authorization: Bearer`(API トークン)の 2 つで、
**OAuth がない**。利用者ごとに別のトークンを発行する仕組みもない(トークンは BOARD のアカウント単位)。
一方、MCP クライアント(Claude など)は、リモートの MCP サーバーに OAuth でつなぐ。

姉妹リポジトリ logbutler(Backlog 用 Remote MCP)は、Backlog に OAuth があるので、Worker 自身を OAuth 認可サーバーにして
利用者ごとの Backlog トークンを預かる。その代わり、Cloudflare Access の Managed OAuth は使わない
(公式ドキュメントが『自前の OAuth を持つサーバーには使わない』と明記しているため。logbutler の ADR 0009)。
board は条件が逆である。

## 決定

- **BOARD の API キー / トークンは Worker の secret(`BOARD_API_KEY` / `BOARD_API_TOKEN`)に置き、全利用者で共有する。** 1 デプロイ = 1 BOARD アカウント。profile は持たない。
- **Worker は OAuth サーバーを持たない。** Cloudflare Access の MCP server application(または self-hosted application)で保護し、**Managed OAuth を有効にする**。
  MCP クライアントとの OAuth(DCR・PKCE・同意)は Access が担う。
- Worker は `/healthz` 以外の全リクエストで `Cf-Access-Jwt-Assertion` の Access JWT(署名・iss・aud・exp)を検証する(`src/auth/access-jwt.ts`。jose、JWKS は `https://{team}/cdn-cgi/access/certs` から取得し isolate 内で 5 分キャッシュ)。
  Cloudflare の公式ドキュメントが、Managed OAuth を使う MCP server application について『Access JWT を検証しなければならない』としている。
  JWT なしは 401、それ以外の失敗(JWKS が取れない場合を含む)は 403。理由は応答に出さず、ログに種別だけ残す。
- `workers_dev: false`。カスタムドメインだけで公開し、Access を迂回できる経路を作らない。JWT 検証は多層防御の 2 枚目として働く。
- 誰が使えるかは Access のポリシー(と Portal)で決める。Worker は『誰か』を区別しない。
- BOARD の API キー・トークンと JWT は、ログ・エラー・MCP の応答に出さない。

## 却下した案

- **利用者ごとの BOARD トークンの入力(Worker 内に OAuth サーバーを持つ)**: BOARD に OAuth がないので、トークンを人が貼り付ける画面を自作することになる。
  KV / D1 に預かるトークンが増え、責務も重い。少人数で 1 つの BOARD アカウントを共有する想定には過剰。
- **Access を使わず Worker で独自に認証する**: JWT の検証より弱くなり、車輪の再発明になる。

## 帰結

- **BOARD 側の操作履歴では、誰が呼んだかを区別できない**(全員が同じキー)。必要なら Worker のログ(Access の JWT の `email` など)で追う。現状は JWT の中身をログに出していない。
- 共有キーが漏れると全員分の権限が漏れる。secret は `wrangler secret put` だけで登録し、リポジトリ・`board.config.json`・`deploy/wrangler.jsonc` には載せない。
- Access の AUD と team domain を間違えると全リクエストが 403 になる。設定チェック(`src/config-check.ts`)が欠けと形式違いを `/healthz` で伝える。
- logbutler との違い: logbutler は『利用者ごとのトークン + Access なし』、board は『共有キー + Access』。この表は board の方が運用が軽い代わりに、利用者の区別がない。

## 追記(2026-11-06): カスタムドメインは任意

上の『`workers_dev: false`。カスタムドメインだけで公開し…』を改める。`customDomain` を `board.config.json` の任意項目にした。空(キーなし、または `""`)なら workers.dev で公開する(`workers_dev: true`、`routes` なし)。
理由: MCP Server Portal に登録するサーバーを workers.dev で公開している組織があるため。
Access を迂回できない根拠は、公開経路ではなく、Worker が `/healthz` 以外の全パスで JWT を検証すること(`src/` は変えない)。workers.dev に直接来ても 401 / 403 になる。
`preview_urls: false` を両方の場合で明示し、Version URL という Access を付け忘れやすい経路を作らない。
workers.dev で Access と Managed OAuth が実際に動くかは未確認(`docs/manual-verification.md` の V10)。
