# デプロイと運用の手順書

board を自分の Cloudflare アカウントにデプロイし、Access と MCP Server Portal につなぐ手順。
**コマンドは利用者が実行する**(リポジトリの作業用エージェントはデプロイ・`wrangler secret put`・D1 の作成をしない)。

Cloudflare 側の画面の名前は 2026-11 時点の公式ドキュメントに基づく。変わっていたら公式ドキュメントを優先する:
[MCP server portals](https://developers.cloudflare.com/cloudflare-one/access-controls/ai-controls/mcp-portals/) /
[Secure MCP servers](https://developers.cloudflare.com/cloudflare-one/access-controls/ai-controls/secure-mcp-servers/) /
[Managed OAuth](https://developers.cloudflare.com/cloudflare-one/access-controls/applications/http-apps/managed-oauth/)。

## 目次

1. D1 の作成
2. secrets の登録
3. `mise.local.toml` の `[env]`
4. deploy
5. Access の設定(MCP server application / Managed OAuth / AUD / ポリシー)
6. MCP Server Portal への登録
7. live 用の service token と Service Auth ポリシー
8. 運用(ログ・キャッシュの初回構築・`cache_status`・rate limit)
9. 注意: Workers Free プランの D1 クエリ上限

前提: Cloudflare アカウント(**Workers Paid プラン**。9 を参照)、Zero Trust(Access)が有効、公開用のカスタムドメインが Cloudflare の zone にある、
BOARD の API キーとトークン、`mise install` と `mise run install` が済んでいること。

## 1. D1 の作成

```bash
pnpm exec wrangler d1 create board-cache   # 名前は任意
```

出力された `database_id`(UUID)を控える。名前と id は 3 で `mise.local.toml` に書く。
スキーマ(表)は作らない: Worker が最初のリクエストで冪等に適用する(ADR 0002)。`wrangler d1 migrations` は使わない。

## 2. secrets の登録

BOARD の API キーとトークン(全利用者で共有する。ADR 0001)を Worker の secret に入れる。値はコマンドの入力プロンプトに貼り付ける(シェル履歴・リポジトリに残さない)。

```bash
pnpm exec wrangler secret put BOARD_API_KEY   --name <BOARD_WORKER_NAME>
pnpm exec wrangler secret put BOARD_API_TOKEN --name <BOARD_WORKER_NAME>
```

Worker がまだ存在しない場合、wrangler は『その名前で Worker を作って secret を追加するか』を聞く。`y` で進めてよい(4 の deploy で中身が入る)。
`<BOARD_WORKER_NAME>` は 3 で決める Worker 名と同じにする。

## 3. `mise.local.toml` の `[env]`

リポジトリ直下に `mise.local.toml`(gitignore 済み)を作る。**アカウント固有の値はここだけに書く**(`wrangler.jsonc` には書かない)。

```toml
[env]
BOARD_WORKER_NAME = "board"                                  # Worker 名(小文字・数字・ハイフン)
BOARD_D1_DATABASE_ID = "<1 で控えた UUID>"
BOARD_D1_DATABASE_NAME = "board-cache"                       # 1 で付けた名前
BOARD_ACCESS_TEAM_DOMAIN = "<team>.cloudflareaccess.com"     # Zero Trust のチームドメイン
BOARD_ACCESS_AUD = "<5 で取得する AUD タグ(64 桁の 16 進)>"
BOARD_CUSTOM_DOMAIN = "board.example.com"                    # 公開するホスト名(ドメインだけ。https:// は付けない)
# 任意
# BOARD_BASE_URL = "https://api.the-board.jp"                # BOARD API の URL(省略すると wrangler.jsonc の値)
# BOARD_UI_BASE_URL = "https://the-board.jp"                 # find_* の url の組み立て先(省略すると BOARD_BASE_URL の api. を外して導く)
```

5 で AUD が分かるまでは、ダミー(64 桁の 0 など)を入れておき、5 のあとに差し替えて再デプロイする。
`mise run deploy:render` は、未設定・形式違い・プレースホルダ(`<...>`)の項目を**項目名だけ**挙げて失敗する(値は表示しない)。

## 4. deploy

```bash
mise run deploy:render   # wrangler.deploy.jsonc を生成して中身を確認する(gitignore 済み)
mise run deploy          # 生成 → wrangler deploy -c wrangler.deploy.jsonc
```

- `workers_dev` は `false`。カスタムドメイン(`BOARD_CUSTOM_DOMAIN`、`custom_domain: true`)だけで公開する。
- 事前の動作確認は `mise run deploy:check`(ダミー値で生成して `wrangler deploy --dry-run`。Cloudflare には何も送らない。CI でも走る)。
- デプロイ直後は `curl https://<domain>/healthz` で確認する。`{"status":"ok"}` なら設定がそろっている。`503` なら本文の `missing` / `invalid` に項目名が出る(値は出ない)。
  Access の保護前でも `/healthz` だけは認証なしで返る。それ以外のパスは JWT がなければ 401。

## 5. Access の設定

Worker を Access の **MCP server application**(または self-hosted application)として保護し、**Managed OAuth** を有効にする。
Worker は OAuth サーバーを持たず、Access が発行した JWT(`Cf-Access-Jwt-Assertion`)を検証する(ADR 0001)。

1. Zero Trust ダッシュボード → **Access controls** → **Applications** → アプリケーションを作る。
   種類は MCP server application(ない場合は self-hosted application)。公開ホスト名は `BOARD_CUSTOM_DOMAIN`、パスは Worker 全体(または `/mcp`)。
2. アプリケーションを編集 → **Advanced settings** → **Managed OAuth** を ON にして保存する。
   (Managed OAuth は MCP server application / self-hosted application では既定 OFF。Portal では新規作成時に既定 ON。)
3. **AUD タグ**を取得する: アプリケーションの設定画面の **Basic information**(Application Audience (AUD) Tag)。64 桁の 16 進。
   `mise.local.toml` の `BOARD_ACCESS_AUD` に入れ、`BOARD_ACCESS_TEAM_DOMAIN` は Zero Trust の **Settings** → **Custom Pages**(または Team name)で確認できるチームドメインにして、`mise run deploy` で再デプロイする。
4. **ポリシー**: 人が使う **Allow** ポリシー(例: 自分のメールアドレスや自組織の IdP のグループ)を付ける。BOARD のデータが見られるので、広げすぎない。
   live 用の service token は 7 で別に追加する。
5. 動作確認: `curl -i https://<domain>/mcp` は Access のログインへのリダイレクトか 401 になる(JWT なし)。ブラウザの MCP クライアントでつなぎ、ログインできること。

> Access を迂回する経路がないことを確かめる: `workers_dev` は無効で、カスタムドメイン以外に Worker の URL は存在しない。

## 6. MCP Server Portal への登録

Portal を使うと、複数の MCP サーバーを 1 つの URL にまとめ、クライアントには Portal の URL だけを渡せる。

1. Zero Trust → **Access controls** → **MCP Portals** → **MCP servers** タブ → **Add MCP server**。
2. 名前を入れ、**HTTP URL** に `https://<domain>/mcp`(`BOARD_CUSTOM_DOMAIN` + `/mcp`)を入れる。
3. Access ポリシーでサーバーを見せる利用者を絞る。OAuth credentials は **Automatic**(DCR)。保存して接続し、状態が **Ready** になることを確認する。
4. **Add server portal** で Portal を作り(または既存の Portal に)このサーバーを追加する。Portal の URL は `https://<portal-subdomain>.<domain>/mcp`。
5. Portal 経由だとツール名の前に `board_` が付く(例 `board_list_clients`)。

注意: Portal 経由では **Require user auth**(利用者ごとの OAuth)の扱いに注意する。board は共有キーなので利用者ごとのトークンは持たないが、
Access の Managed OAuth でログインした利用者の JWT が Worker に届く。Portal が `Cf-Access-Jwt-Assertion` を付けて届くかは実機で確認する(docs/manual-verification.md の V1)。

## 7. live 用の service token と Service Auth ポリシー

`mise run test:live` は、デプロイ済みの Worker に Access の **service token** で接続する(ブラウザのログインができないため)。

1. Zero Trust → **Access controls** → **Service credentials** → **Service Tokens** → **Create Service Token**(名前 `board-live`、有効期間を決める)。
   **Client ID と Client Secret をコピーする(Secret はこのとき一度しか表示されない)。**
2. 5 のアプリケーションに、2 つ目のポリシーを追加する: **Action = Service Auth**、Include = **Service Token** = `board-live`。
   (人の Allow ポリシーとは別のポリシーにする。service token は Allow ポリシーには一致しない。)
3. Portal 経由の確認もしたい場合は、Portal のアプリケーションにも同じ Service Auth ポリシーを追加する。`test:live` は Worker の URL に直接つなぐので、通常は不要。
4. 実行(**値はシェル履歴に残さないよう `mise.local.toml` の `[env]` に置くのが安全**):

```bash
BOARD_LIVE_URL=https://<domain>/mcp \
BOARD_LIVE_ACCESS_CLIENT_ID=<Client ID> \
BOARD_LIVE_ACCESS_CLIENT_SECRET=<Client Secret> \
mise run test:live
```

- ヘッダー `CF-Access-Client-Id` / `CF-Access-Client-Secret` を送ると、Access が JWT に変換して Worker に渡す。
- 環境変数が足りなければ、足りない変数名を表示して全体を skip する(`mise run test` と CI には含まれない)。
- 呼ぶツール: `server_info` / `cache_status` / `list_users` / `list_clients` / `find_projects`(`limit` 付き)と、`users` / `clients` / `projects` の `refresh=full` を complete になるまで。
  計測して出力する: 1 回あたりの時間、full refresh が complete になるまでの呼び出し回数、1 ページあたりの時間、応答サイズ。結果は docs/manual-verification.md の記入欄に転記する。
- **full refresh は D1 のキャッシュを実際に育てる**(8 の初回構築と同じ操作)。BOARD への書き込みはしない。BOARD の呼び出し回数(3,000 回/日)を使う。
- 使い終わったら service token を失効させるか、Service Auth ポリシーを外す。

## 8. 運用

### ログ
- Workers Logs(`observability.enabled: true`)で `wrangler tail --config wrangler.deploy.jsonc` かダッシュボードの Observability を見る。
- 想定外の例外は `tool <name> failed unexpectedly: <例外名>: <message>` が出る(Bearer と x-api-key の値は伏せる)。応答は `generic_error`。
- JWT の拒否は `access jwt rejected: <理由>`(種別だけ。JWT は出ない)。設定の欠けは `{"event":"config_invalid","missing":[...],"invalid":[...]}`(項目名だけ)。
- API キー・トークン・JWT はログにも応答にも出さない。

### キャッシュの初回構築
最初はキャッシュが空で、`refresh=none`(既定)の一覧は空を返す。使うリソースごとに、`refresh=full` を **`refresh_state` が `complete` になるまで**繰り返す。

1. ツール(例 `list_clients`)を `refresh: "full"`(`limit` を付けると応答が小さい)で呼ぶ。
2. 応答の `cache[].refresh_state` が `partial` なら、同じ呼び出しをもう一度。最後に取り終えたページから再開する(30 秒の締め切りのため、件数の多いリソースは数回かかる)。
3. `refresh_in_progress`(`retry_after_seconds`)が返ったら、その秒数だけ待って再試行する(別の refresh が動いている)。
4. `complete` になったら、以降は `refresh=delta`(差分)で足りる。削除されたエンティティを反映したいときだけ、ときどき `full`。
5. 書類(見積・受注・納品・領収)を ID で引く `find_*` は、案件との対応(逆引き)を覚えるまで、最初の 1〜2 回は `warnings` に `project_unresolved`(`reason: deadline`)が出うる。同じ呼び出しを繰り返すと続きから進む(ADR 0004)。

### `cache_status`
BOARD を呼ばずに D1 の状態を返す(読み取り専用)。見るところ:
- `refresh_state`: `complete` / `partial`(full が途中。続きを `refresh=full` で)/ `failed`(`last_error_code`)/ `not_synced`(未取得)。
- `row_count`、`cached_at`、`full_refreshed_at`、`full_prune_skipped`(true なら次の full で取り直す)、`locked`(refresh 中)。
- `document_scans`: 書類種別ごとの逆引きの走査状態。

`cache_clear` は破壊的(`resources` を省略すると全リソース)。壊れたと疑うとき以外は、`refresh=full` で足りる。refresh 中は `refresh_in_progress` で何も消さない。逆引きの対応表は既定で残る(`include_document_map: true` で消す)。

### rate limit
- BOARD API は **3 回/秒・3,000 回/日**。Worker は BOARD への通信を直列化し、429 / 5xx は `Retry-After` を尊重して最大 3 回まで再試行する(残り時間を超える待ちは待たずに `retryable: true` で失敗)。
- 複数人が同時にキャッシュミスすると 3 回/秒を超えうる(再試行で吸収する)。問題になったら Durable Objects による全体制御を再検討する(ロードマップの決定済み事項)。
- 1 日の上限が近いときは `refresh` を使わずキャッシュを読む。

## 9. 注意: Workers Free プランの D1 クエリ上限

**Workers Free プランでは、1 回の Worker 実行あたりの D1 クエリが 50 まで**(Paid は 1,000。`db.batch()` は 1 回と数える。2026-11 時点の Cloudflare D1 Limits)。
full refresh(1 ページごとに複数のクエリ)や、書類の逆引きの走査は、1 回の呼び出しで多数のクエリを使う。**これらは Paid プランを前提にしている。**
Free プランで動かすと、`refresh=full` や `find_*` の途中で D1 のエラーになりうる(`generic_error`)。Free で使う場合は `refresh=none` / `delta` と、件数の少ないリソースに限る。
