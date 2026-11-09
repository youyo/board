# デプロイと運用の手順書

board を自分の Cloudflare アカウントにデプロイし、Access と MCP Server Portal につなぐ手順。
board は**公開の upstream** で、利用者は自分の **private リポジトリ(下流)** に upstream を取り込んでデプロイする(ADR 0005)。
**急ぐ人は 0 で下流を作り、1 で `board.config.json` を書いて、2 の `mise run setup` を実行する。** 以降は詳細と運用。
下流のセットアップを案内するスキルがある: `.claude/skills/board-setup/SKILL.md`(Claude Code / Codex が下流リポジトリで読む)。

**デプロイ・`wrangler secret put`・D1 の作成は利用者が実行する**(upstream の作業用エージェントはしない。下流でスキルに従う場合も、運用者の承認の後)。

Cloudflare 側の画面の名前は 2026-11 時点の公式ドキュメントに基づく。変わっていたら公式ドキュメントを優先する:
[MCP server portals](https://developers.cloudflare.com/cloudflare-one/access-controls/ai-controls/mcp-portals/) /
[Secure MCP servers](https://developers.cloudflare.com/cloudflare-one/access-controls/ai-controls/secure-mcp-servers/) /
[Managed OAuth](https://developers.cloudflare.com/cloudflare-one/access-controls/applications/http-apps/managed-oauth/)。

## 目次

0. 下流リポジトリの作り方
1. `board.config.json`(下流が書く唯一の設定)
2. クイックスタート(`mise run setup`)
3. 個別の手順(D1・secrets・`deploy:config`・`deploy`)
4. upstream の更新の取り込み
5. Access の設定(MCP server application / Managed OAuth / AUD / ポリシー)
6. MCP Server Portal への登録
7. live 用の service token と Service Auth ポリシー
8. 運用(ログ・キャッシュの初回構築・`cache_status`・rate limit)
9. 注意: Workers Free プランの D1 クエリ上限
10. 障害の切り分け

前提: Cloudflare アカウント(**Workers Paid プラン**。9 を参照)、Zero Trust(Access)が有効、公開用のドメインが Cloudflare の zone にある、
BOARD の API キーとトークン、[mise](https://mise.jdx.dev/) が使えること。

## 0. 下流リポジトリの作り方

下流は、board を upstream として登録した**自分の private リポジトリ**。**upstream のファイルは書き換えず、新しいファイルを足すだけ**で使う(更新を衝突なく取り込むため)。

```bash
# 1. upstream を clone して、自分の private リポジトリを origin にする(GitHub で空の private リポジトリを作っておく)
git clone https://github.com/youyo/board.git my-board && cd my-board
git remote rename origin upstream
git remote add origin <自分の private リポジトリの URL>
git push -u origin main          # 運用者が実行する

# 2. 依存を入れる
mise install && mise run install
```

下流が**足す**ファイル(コミットする。秘密は入らない):

| ファイル | 内容 |
|---|---|
| `board.config.json` | 下流の設定(1)。`board.config.example.json` をコピーして値を入れる |
| `deploy/wrangler.jsonc` | `mise run deploy:config` の生成物(1)。手で編集しない |
| `.github/workflows/deploy.yml` | 任意。下流の CI(`deploy.example/deploy.workflow.yml` のコピー。[deploy-ci.md](deploy-ci.md)) |

**`board.config.json` と `deploy/` は upstream の `.gitignore` に入っている**(upstream に実値を入れないため)。下流では最初の 1 回だけ強制的に追跡する。

```bash
git add -f board.config.json deploy/
git commit -m "下流の設定を追加"      # コミットは運用者が行う
```

一度追跡すれば、以降の変更は通常どおり `git status` / `git add` で扱える。`.gitignore` は書き換えない(upstream の更新と衝突するため)。
`wrangler.jsonc` / `mise.toml` / `scripts/` など upstream のファイルも書き換えない。直したい点は upstream への提案にする。

upstream 専用の CI(`.github/workflows/ci.yml` の `github.repository == 'youyo/board'` のジョブ)は、下流では自動で止まる。
`deploy/wrangler.jsonc` の `ACCESS_AUD` は gitleaks の `generic-api-key` に当たるが、秘密値ではない。upstream の `.gitleaks.toml` で許可済みなので、下流で設定を足す必要はない([deploy-ci.md の 5](deploy-ci.md))。

## 1. `board.config.json`

`board.config.example.json` をコピーして値を入れる。**秘密でない値だけ**を書く(API キー・トークンは書かない。2 の secrets は別に登録する)。
形式が不正な項目は、`mise run deploy:config` / `setup` が**項目名と理由だけ**を挙げて失敗する(値は表示しない)。

| 項目 | 必須 | 内容・形式 | 例 |
|---|---|---|---|
| `workerName` | 必須 | Worker 名(小文字・数字・ハイフン) | `board` |
| `customDomain` | 任意 | 公開するホスト名(`https://` は付けない)。既存の DNS レコードがないこと。**空(キーなし、または `""`)なら workers.dev で公開する**(下の 5 の『workers.dev で公開する場合』) | `board.example.com` |
| `d1.databaseName` | 必須 | D1 の名前 | `board-cache` |
| `d1.databaseId` | 必須 | D1 の uuid。`<YOUR_D1_DATABASE_ID>` のままにしておくと `setup` が作成して書き戻す | |
| `access.teamDomain` | 必須 | Zero Trust のチームドメイン | `<team>.cloudflareaccess.com` |
| `access.aud` | 必須 | Access アプリの AUD タグ(64 桁の 16 進)。Access を作る前(5)は 64 桁の 0 などのダミーにしておく | |
| `boardBaseUrl` | 任意 | BOARD API の URL(https のオリジン)。空なら upstream の値(`https://api.the-board.jp`) | |
| `boardUiBaseUrl` | 任意 | BOARD の UI の URL(https のオリジン。`find_*` の `url` の組み立てに使う)。空なら `boardBaseUrl` の `://api.` を `://` にして導く | |
| `accountId` | 任意 | Cloudflare のアカウント ID(32 桁の 16 進)。複数のアカウントに入っているときに指定する | |

使わない任意の項目は空文字のままにする。未知の項目名はエラー(綴りの間違いに気づくため)。

`mise run deploy:config` が `board.config.json` と upstream の `wrangler.jsonc` から `deploy/wrangler.jsonc` を生成する。
- `customDomain` があれば `routes` は `customDomain`(`custom_domain: true`)だけで `workers_dev` は `false`。なければ `workers_dev` は `true` で `routes` は出さない。どちらも `preview_urls` は `false`(Version URL は作らない)。`customDomain` がホスト名でない・`<...>` のままならエラー。
- `main` などの相対パスは `deploy/` から見たパス(`../src/index.ts`)に直る。
- 生成物は**下流でコミットする**(デプロイ時に再生成しないので、CI に特別な権限が要らない)。
- 設定の検証だけなら `mise run deploy:config:validate`(`d1.databaseId` が未作成でもよい)。

`mise run deploy:config:check` は、作り直した結果とコミット済みの `deploy/wrangler.jsonc` を比べる(ファイルは書かない。資格情報も不要)。次の 3 つを検出する。
- `board.config.json` を直したのに生成し直していない
- 生成物を手で書き換えた
- upstream の `wrangler.jsonc` の変更を取り込んでいない

差分があれば `mise run deploy:config` で作り直してコミットする。

## 2. クイックスタート(`mise run setup`)

```bash
pnpm exec wrangler login          # 1 回だけ(ブラウザ)
cp board.config.example.json board.config.json   # 値を入れる(1)。d1.databaseId は <YOUR_D1_DATABASE_ID> のまま
mise run setup                     # 何度でも再実行できる。--yes で D1 の確認を省く(mise run setup -- --yes)
```

`setup` は次の順に進める。**前提に 1 つでも欠けていたら何も実行せずに止まる**(mise・`wrangler login` 済みか・`board.config.json` の検証)。
1. D1 がなければ `wrangler d1 create` を実行し、得た id を `board.config.json` に書き戻す(書き戻す前に確認する。`--yes` で省く)。同名の D1 が既にあれば作らず、id だけ書き戻す。
2. `mise run deploy:config`
3. `mise run deploy`
4. 未登録の secret(`BOARD_API_KEY` / `BOARD_API_TOKEN`)だけを `wrangler secret put`(隠した入力)。値はスクリプトを通らず、引数・env・ファイル・ログに残らない。登録済みのものは上書きしない。対話端末でなければ飛ばして手順を案内する。
5. `https://<customDomain>/healthz` で確認する。503 なら、欠けている項目名を表示する。DNS・証明書の反映待ちで届かないことがあるので、5 秒おきに 6 回(約 30 秒)再試行する。`customDomain` が空なら、`wrangler deploy` の出力の `https://<worker>.<subdomain>.workers.dev` で確認する。workers.dev の新しいホスト名は反映に時間がかかることがあるので、再試行は約 2 分(5 秒おきに 24 回)まで伸ばす。
   - それでも届かない(workers.dev のとき)場合は、失敗にしない。デプロイと secrets は済んでいるので、警告を出して終わる(exit 0)。`/healthz` を手で確認し、しばらくして `mise run setup` を再実行すれば続きから進む。カスタムドメインのときは、届かなければ失敗で終わる。
   - URL を取れなければ失敗にせず、`https://<worker>.<アカウントのサブドメイン>.workers.dev/healthz` を手で確認する案内を出して続ける。
   - Access が `/healthz` の手前で止めているとき(Access のログインへの 302、または JSON でない 401 / 403)も失敗にしない。「Access が有効」と案内する。確認の方法は 5 の『`/healthz` の確認』。
6. 画面の作業(Access・Portal)は 5〜7 を案内して終わる。

途中で失敗・中断しても、原因を直して `mise run setup` を再実行すればよい(D1・登録済みの secret・生成物は冪等)。

`setup` 後の画面の作業は、5(Access)→ 6(Portal)→ 7(service token と `test:live`)→ 8(キャッシュの初回構築)の順。
Access の AUD が分かったら `board.config.json` の `access.aud` を実値にして、`mise run deploy:config && mise run deploy`(または `setup` を再実行)で反映する。

## 3. 個別の手順(`setup` を使わない場合)

### D1 の作成

```bash
pnpm exec wrangler d1 create board-cache   # 名前は任意(d1.databaseName と同じにする)
```

出力された `database_id`(uuid)を `board.config.json` の `d1.databaseId` に書く。
スキーマ(表)は作らない: Worker が最初のリクエストで冪等に適用する(ADR 0002)。`wrangler d1 migrations` は使わない。

### secrets の登録

BOARD の API キーとトークン(全利用者で共有する。ADR 0001)を Worker の secret に入れる。値はコマンドの入力プロンプトに貼り付ける(シェル履歴・リポジトリに残さない)。

```bash
pnpm exec wrangler secret put BOARD_API_KEY   -c deploy/wrangler.jsonc
pnpm exec wrangler secret put BOARD_API_TOKEN -c deploy/wrangler.jsonc
```

Worker がまだ存在しない場合、wrangler は『その名前で Worker を作って secret を追加するか』を聞く。`y` で進めてよい(deploy で中身が入る)。

### 生成とデプロイ

```bash
mise run deploy:config   # deploy/wrangler.jsonc を生成して中身を確認する
mise run deploy          # wrangler deploy -c deploy/wrangler.jsonc
```

- `mise run deploy` は、`deploy/wrangler.jsonc` が無い、または `<...>` のプレースホルダが残っていると、何も実行せずに止まる。
- 事前の動作確認は `mise run deploy:check`(example の設定をダミー値にして生成し、`wrangler deploy --dry-run`。Cloudflare には何も送らない。upstream の CI でも走る)。
- デプロイ直後(Access を付ける前)は `curl https://<domain>/healthz` で確認する。`{"status":"ok"}` なら設定がそろっている。`503` なら本文の `missing` / `invalid` に項目名が出る(値は出ない)。
  Worker は `/healthz` だけ認証なしで返す。それ以外のパスは JWT がなければ 401。
  **Access を付けた後は、アプリケーションがホスト全体を保護するので `/healthz` も Access の内側に入り、`curl` は Worker まで届かない**(Access のログインへの 302、または 401)。確認の方法は 5 の『`/healthz` の確認』。

## 4. upstream の更新の取り込み

```bash
git fetch upstream
git merge-base --is-ancestor main upstream/main && echo "fast-forward できる"
git diff --no-ext-diff --stat main upstream/main     # 差分を確認する(wrangler.jsonc・scripts/・deploy.example/・この runbook の設定の表・mise.toml)
git merge --ff-only upstream/main                    # 下流に独自のコミット(追加ファイルのコミットを除く)がなければ fast-forward
mise run deploy:config                               # upstream の wrangler.jsonc が変わっていれば deploy/ に差分が出る
mise run lint && mise run test && mise run deploy:config:check
```

- 下流のコミットは追加ファイル(`board.config.json`・`deploy/`・`.github/workflows/deploy.yml`)だけなので、通常は upstream の更新と衝突しない。fast-forward できない(履歴が分岐した)ときは `git merge upstream/main` になるが、upstream のファイルを書き換えていない限り自動でマージできる。衝突したら、下流で upstream のファイルを編集していないか確かめる。
- `board.config.json` に新しい項目が要る更新は、upstream のこの runbook の 1 の表と `board.config.example.json` に出る。先に追加する。
- 新しい secret が要る更新は README と runbook に書く。先に登録する。
- 差分を確認したうえで `mise run deploy`(または下流の CI にデプロイさせる)。ツールの名前や引数が変わった更新は、Portal の一覧に反映されるまで再同期が要ることがある。

## 5. Access の設定

Worker を Access の **MCP server application**(または self-hosted application)として保護し、**Managed OAuth** を有効にする。
Worker は OAuth サーバーを持たず、Access が発行した JWT(`Cf-Access-Jwt-Assertion`)を検証する(ADR 0001)。

1. Zero Trust ダッシュボード → **Access controls** → **Applications** → アプリケーションを作る。
   種類は MCP server application(ない場合は self-hosted application)。公開ホスト名は `board.config.json` の `customDomain`、パスは Worker 全体(または `/mcp`)。`customDomain` が空(workers.dev)なら下の『workers.dev で公開する場合』。
2. アプリケーションを編集 → **Advanced settings** → **Managed OAuth** を ON にして保存する。
   (Managed OAuth は MCP server application / self-hosted application では既定 OFF。Portal では新規作成時に既定 ON。)
3. **AUD タグ**を取得する: アプリケーションの設定画面の **Basic information**(Application Audience (AUD) Tag)。64 桁の 16 進。
   `board.config.json` の `access.aud` に入れる。`access.teamDomain` は Zero Trust の **Settings** → **Custom Pages**(または Team name)で確認できるチームドメインにする。
   `mise run deploy:config` で `deploy/wrangler.jsonc` を作り直し(差分を確認してコミットする)、`mise run deploy` で再デプロイする。
4. **ポリシー**: 人が使う **Allow** ポリシー(例: 自分のメールアドレスや自組織の IdP のグループ)を付ける。BOARD のデータが見られるので、広げすぎない。
   live 用の service token は 7 で別に追加する。
5. 動作確認: `curl -i https://<domain>/mcp` は Access のログインへのリダイレクトか 401 になる(JWT なし)。ブラウザの MCP クライアントでつなぎ、ログインできること。

> Access を迂回する経路がないことを確かめる: `customDomain` がある場合、`workers_dev` は無効で、カスタムドメイン以外に Worker の URL は存在しない。`preview_urls`(Version URL)はどちらの場合も無効。

**workers.dev で公開する場合**(`customDomain` が空。Portal に登録するサーバーを workers.dev で公開している環境向け)

1. `mise run setup`(または `deploy`)で Worker を作る。公開 URL は `https://<worker>.<subdomain>.workers.dev`。
2. Worker の Settings > Domains & Routes の workers.dev で **Enable Cloudflare Access** する。作られる self-hosted アプリケーションを開き、**Managed OAuth** を有効にする。その AUD タグを `board.config.json` の `access.aud` に入れ、`deploy:config` → `deploy` で反映する(上の 3、4 と同じ)。
3. Portal の HTTP URL は `https://<worker>.<subdomain>.workers.dev/mcp`(6)。
4. **【未検証】** MCP server application 型を workers.dev ホストで作れるか、workers.dev で Access と Managed OAuth が動き JWT が Worker に届くかは実機で確かめていない(`docs/manual-verification.md` の V10)。作れなければ self-hosted application で足りるかを確かめる。
5. Access を付けるまでの間も、Worker は `/healthz` 以外で JWT を検証するので、workers.dev に直接来ても 401 / 403 になる(Access の代わりにはならない。多層防御の 2 枚目)。

### `/healthz` の確認(Access を付けた後)

Access のアプリケーションはホスト全体を保護するので、Access を付けた後は `curl .../healthz` が Worker に届かない。次のどれかで確認する。

**A. `/healthz` を Bypass にする**(`curl` で確認できるようになる)

`/healthz` は設定の項目名(`missing` / `invalid`)しか返さず、値は返さない設計なので、認証なしで公開してよい。

1. Zero Trust ダッシュボード → **Access controls** → **Applications** → self-hosted application を別に作る。
2. 公開ホスト名は、Worker のホスト(`customDomain`、または `<worker>.<subdomain>.workers.dev`)と同じ。**パスは `/healthz` だけ**にする(Worker 全体のアプリは変えない)。
3. ポリシーは **Bypass** を 1 つだけ付ける(Include は Everyone)。Managed OAuth は有効にしない。
4. 確認: `curl -i https://<domain>/healthz` が 200 になる。`/mcp` は引き続き Access に止められる(JWT なしで 302 か 401)ことも見る。広がっていたらこのアプリのパスを見直す。

- **【未確認】** 「パスつきのアプリがホスト全体のアプリより優先される」ことは、実機で確かめていない(workers.dev ホストについて `docs/manual-verification.md` の V12)。優先されないときは `/healthz` が Access の内側のままになるので、B か C を使う。
- Bypass にするのは `/healthz` だけ。`/mcp` など他のパスを含めない。

**B. ブラウザでログインしてから開く**

Bypass にしない場合は、ブラウザで `https://<domain>/healthz` を開く。Access のログインを通ると、Worker の応答(`{"status":"ok"}`、または 503 と項目名)が見える。

**C. `wrangler tail` で見る**

```bash
pnpm exec wrangler tail --config deploy/wrangler.jsonc
```

ログインしたブラウザなどから Worker にリクエストを送り、ログに種別だけが出るのを見る(`access jwt rejected: <理由>` など。JWT や値は出ない)。

`mise run setup` は、`/healthz` が Access の手前で止まっていると分かったら失敗にせず、「Access が有効」と案内して終わる。

## 6. MCP Server Portal への登録

Portal を使うと、複数の MCP サーバーを 1 つの URL にまとめ、クライアントには Portal の URL だけを渡せる。

1. Zero Trust → **Access controls** → **MCP Portals** → **MCP servers** タブ → **Add MCP server**。
2. 名前を入れ、**HTTP URL** に `https://<domain>/mcp`(`customDomain` + `/mcp`。workers.dev なら `https://<worker>.<subdomain>.workers.dev/mcp`)を入れる。
3. Access ポリシーでサーバーを見せる利用者を絞る。OAuth credentials は **Automatic**(DCR)。保存して接続し、状態が **Ready** になることを確認する。
4. **Add server portal** で Portal を作り(または既存の Portal に)このサーバーを追加する。Portal の URL は `https://<portal-subdomain>.<domain>/mcp`。
5. Portal 経由だとツール名の前に `board_` が付く(例 `board_list_clients`)。

**Require user auth の設定**: 既定では **ON**(利用者ごと)を勧める。各利用者が Portal の Connect から Access(Managed OAuth)にログインし、
その人の JWT(`email` と `sub` を持つ)が Worker に届く。board は共有キーなので BOARD 側の権限は全員同じだが、Access のポリシーで利用者を絞れ、Access の監査ログに利用者が残る。
**OFF**(`on_behalf: false`、Portal の管理者資格情報で接続)にするのは、Portal に service token で自動化エージェントをつなぐ場合だけ。この場合 Worker に届くのは
service token の JWT(`email` なし・`sub` は空・`common_name` に Client ID)で、Worker はこれも受け入れる(`src/auth/access-jwt.ts` の `kind: "service"`)。
Portal が `Cf-Access-Jwt-Assertion` を付けて届くかは実機で確認する(docs/manual-verification.md の V1)。

## 7. live 用の service token と Service Auth ポリシー

`mise run test:live` は、デプロイ済みの Worker に Access の **service token** で接続する(ブラウザのログインができないため)。

1. Zero Trust → **Access controls** → **Service credentials** → **Service Tokens** → **Create Service Token**(名前 `board-live`、有効期間を決める)。
   **Client ID と Client Secret をコピーする(Secret はこのとき一度しか表示されない)。**
2. 5 のアプリケーションに、2 つ目のポリシーを追加する: **Action = Service Auth**、Include = **Service Token** = `board-live`。
   (人の Allow ポリシーとは別のポリシーにする。service token は Allow ポリシーには一致しない。)
3. Portal 経由の確認もしたい場合は、Portal のアプリケーションにも同じ Service Auth ポリシーを追加する。`test:live` は Worker の URL に直接つなぐので、通常は不要。
4. 値を gitignore 済みの `mise.local.toml` の `[env]` に置いてから実行する(**コマンド行に書くとシェル履歴に残るので書かない。`board.config.json` にも書かない**):

```toml
# mise.local.toml(gitignore 済み。エディタで運用者が自分で書く)
[env]
BOARD_LIVE_URL = "https://<domain>/mcp"
BOARD_LIVE_ACCESS_CLIENT_ID = "<Client ID>"
BOARD_LIVE_ACCESS_CLIENT_SECRET = "<Client Secret>"
```

```bash
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
- Workers Logs(`observability.enabled: true`)で `wrangler tail --config deploy/wrangler.jsonc` かダッシュボードの Observability を見る。
- ログは JSON の 1 行(`level` / `event` / `request_id` + 各イベントの項目)。`request_id` はリクエストの `cf-ray`(なければ生成した UUID)で、1 回のリクエストのログを束ねて検索できる。
- 想定外の例外は `{"event":"tool_failed_unexpectedly","tool":"<name>","error_name":"...","error_message":"..."}` (Bearer と x-api-key の値、BOARD_API_KEY / BOARD_API_TOKEN の実値とその URL エンコード形は伏せる。`error_message` は 500 文字まで)。応答は `generic_error`。
- JWT の拒否は `{"event":"access_jwt_rejected","reason":"<種別>"}`(種別だけ。JWT は出ない)。設定の欠けは `{"event":"config_invalid","missing":[...],"invalid":[...]}`(項目名だけ)。
- 書き込み系の操作(`cache_clear`、`refresh=delta|full` を付けた呼び出し)は `{"event":"write_operation","tool":"...","refresh":"...","actor":"<email>"}` を残す。BOARD の認証情報は全員で共有するので、誰が実行したかはこのログで追う。`actor` は Access の JWT の email(service token は Client ID)。読み取りだけの呼び出しでは主体を出さない。`write_operation` は試行の記録で、成否は問わない(失敗しても残る)。document 系に `refresh=delta` を渡した場合も記録される。
- API キー・トークン・JWT はログにも応答にも出さない。

### キャッシュの初回構築
最初はキャッシュが空で、`refresh=none`(既定)の一覧は空を返す。使うリソースごとに、`refresh=full` を **`refresh_state` が `complete` になるまで**繰り返す。

1. ツール(例 `list_clients`)を `refresh: "full"`(`limit` を付けると応答が小さい)で呼ぶ。
2. 応答の `cache[].refresh_state` が `partial` なら、同じ呼び出しをもう一度。最後に取り終えたページから再開する(30 秒の締め切りのため、件数の多いリソースは数回かかる)。
3. `refresh_in_progress`(`retry_after_seconds`)が返ったら、その秒数だけ待って再試行する(別の refresh が動いている)。
4. `complete` になったら、以降は `refresh=delta`(差分)で足りる。削除されたエンティティを反映したいときだけ、ときどき `full`。ただし BOARD の一覧に `updated_at_gteq` が無いリソース(users / groups / payment_terms / project_types / purchase_types / accounting_types / document_send_channels / project_costs)は差分を取れないので、`delta` は `full` として動く(全件を取り直し、応答の `mode` は `full`。v1.5.0 以降、issue #28)。
5. 書類(見積・受注・納品・領収)を ID で引く `find_*` は、案件との対応(逆引き)を覚えるまで、最初の 1〜2 回は `warnings` に `project_unresolved`(`reason: deadline`)が出うる。同じ呼び出しを繰り返すと続きから進む(ADR 0004)。

### `cache_status`
BOARD を呼ばずに D1 の状態を返す(読み取り専用)。見るところ:
- `refresh_state`: `complete` / `partial`(full が途中。続きを `refresh=full` で)/ `failed`(`last_error_code`)/ `not_synced`(未取得)。
- `row_count`、`cached_at`、`full_refreshed_at`、`full_prune_skipped`(true なら次の full で取り直す)、`locked`(refresh 中)。
- `document_scans`: 書類種別ごとの逆引きの走査状態。

`cache_clear` は破壊的(`resources` を省略すると全リソース)。壊れたと疑うとき以外は、`refresh=full` で足りる。refresh 中は `refresh_in_progress` で何も消さない。逆引きの対応表は既定で残る(`include_document_map: true` で消す)。

### rate limit
- BOARD API は **3 回/秒・3,000 回/日**。Worker は BOARD への通信を直列化し、429 / 5xx は `Retry-After` を尊重して最大 3 回まで再試行する(残り時間を超える待ちは待たずに `retryable: true` で失敗)。
- 複数人が同時にキャッシュミスすると 3 回/秒を超えうる(再試行で吸収する)。直列化(`src/board/client.ts` の `serialize`)は 1 つの isolate の中でしか効かず、Workers は負荷に応じて複数の isolate を同時に動かすため、isolate をまたぐと合計が上限を超えることがある。超えた分は BOARD が 429(`rate_limited`)で返し、上記の再試行で吸収する。設計どおりの挙動である(ADR 0003)。問題になったら Durable Objects による全体制御を再検討する(ロードマップの決定済み事項)。
- 1 日の上限が近いときは `refresh` を使わずキャッシュを読む。

## 9. 注意: Workers Free プランの D1 クエリ上限

**Workers Free プランでは、1 回の Worker 実行あたりの D1 クエリが 50 まで**(Paid は 1,000。`db.batch()` は 1 回と数える。2026-11 時点の Cloudflare D1 Limits)。
full refresh(1 ページごとに複数のクエリ)や、書類の逆引きの走査は、1 回の呼び出しで多数のクエリを使う。**これらは Paid プランを前提にしている。**
Free プランで動かすと、`refresh=full` や `find_*` の途中で D1 のエラーになりうる(`generic_error`)。Free で使う場合は `refresh=none` / `delta` と、件数の少ないリソースに限る。

## 10. 障害の切り分け

まず `curl -s https://<domain>/healthz`(Access を付けた後で `/healthz` を Bypass にしていないときは、Worker まで届かない。5 の『`/healthz` の確認』のとおり、ブラウザでログインして開くか、`wrangler tail` で見る)。

| 症状 | 原因と対処 |
|---|---|
| `/healthz` が 503(本文に `missing` / `invalid`) | 本文の項目名が直す箇所(値は出ない)。`BOARD_API_KEY` / `BOARD_API_TOKEN` は secret の登録漏れ → 3 の secrets(または `mise run setup` の再実行)。`ACCESS_TEAM_DOMAIN` / `ACCESS_AUD` / `DB` / `BOARD_BASE_URL` / `BOARD_UI_BASE_URL` は `board.config.json` を直して `deploy:config` → `deploy` |
| Access で 401 | JWT がない。Access を通っていない(アプリケーションの公開ホスト名が違う)、Portal が `Cf-Access-Jwt-Assertion` を付けていない(docs/manual-verification.md の V1) |
| Access で 403 | JWT の検証に失敗。`access.aud` / `access.teamDomain` の誤り(5 の 3)、JWKS が取れない。`wrangler tail --config deploy/wrangler.jsonc` で `access_jwt_rejected` の `reason` を見る(理由の種別だけ。JWT は出ない) |
| `/mcp` が 403(Access は通っている) | `Origin` ヘッダー付きのリクエストは 403 にしている(CORS 無効・Origin の許可リストが空。ADR 0001 の追記)。ブラウザから呼ぶクライアント(MCP Inspector など)が該当する。Portal や Managed OAuth の経路は `Origin` を付けない(docs/manual-verification.md の V11)。ブラウザ経由ではなく、Portal かサーバー側のクライアントから呼ぶ |
| `refresh_in_progress` | 同じリソースの別の refresh が動いている。`retry_after_seconds` 後に再試行する。長く続くなら `cache_status` の `locked` を見る(stale なロックは 10 分で奪われる) |
| `rate_limited` | BOARD の 3 回/秒・3,000 回/日(8)。`retryable: true` なら待って再試行する。日次の上限が近いなら `refresh` を使わずキャッシュを読む |
| 締め切り(30 秒) | 時間内に終わらない処理は、進捗を保存して `refresh_state: partial` で返る。同じ `refresh=full` を繰り返すと最後に取り終えたページから進む(8) |
| Free プランでの D1 のクエリ上限 | `refresh=full` や `find_*` の途中で D1 のエラー(`generic_error`)。Workers Paid が前提(9) |
| `find_*` の `warnings` に `project_unresolved` | 書類 ID から案件 ID の対応を、走査が時間切れで覚え切れていない(`reason: deadline`)。同じ呼び出しを繰り返すと続きから進む(ADR 0004)。`cache_status` の `document_scans` で進み具合を見る |
| `deploy:config:check` が失敗 | `board.config.json` を直したのに生成し直していない / 生成物を手で書き換えた / upstream の `wrangler.jsonc` の変更を取り込んでいない。`mise run deploy:config` で作り直す(手で直さない) |
| `mise run deploy` が止まる | `deploy/wrangler.jsonc` が無い、または `<...>` が残っている。`board.config.json` を直して `deploy:config` |
| `mise run setup` が `wrangler login` を求める | `pnpm exec wrangler login`(ブラウザ)。複数のアカウントに入っているなら `accountId` を指定する |
