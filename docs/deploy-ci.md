# GitHub Actions からデプロイする(下流向け)

下流(private リポジトリ)を、ローカルの `mise run deploy` ではなく GitHub Actions からデプロイするときの手順。
全体の手順は [runbook.md](runbook.md)、下流モデルの理由は [ADR 0005](adr/0005-upstream-downstream-deploy.md)。環境固有の値はすべてプレースホルダ(`<YOUR_...>`)で書く。

**印について**: この repository では実際の Cloudflare・GitHub Actions に対して流していない。確かめていないことには「**【未検証】**」と印を付けた。印の箇所は事実として扱わず、自分の環境で確かめる。

## 1. 構成

雛形は [`deploy.example/deploy.workflow.yml`](../deploy.example/deploy.workflow.yml)。下流の `.github/workflows/deploy.yml` にコピーして使う。

検査とデプロイは別のワークフローに分けている。

| ワークフロー | 起動 | 内容 |
|---|---|---|
| upstream の `ci.yml`(下流でもそのまま動く。書き換えない) | Pull request / main への push | `mise run lint` → `mise run test` → `mise run deploy:check`、`secrets:scan`(gitleaks)、`pinact:check`、`audit` |
| 下流の `deploy.yml`(雛形) | `ci.yml` が main への push で**成功したあと**(`workflow_run`) | `mise run install` → `mise run deploy:config:check` → `mise run deploy` |

- `deploy.yml` は lint・test を実行しない(`ci.yml` が実行済み)。同じ手順が二重に走らない。
- `on.workflow_run.workflows` は `ci.yml` の `name:`(`CI`)と一致させる。下流は `ci.yml` を書き換えないので、名前は変わらない。upstream が名前を変えたら雛形も合わせる(unit テストが突き合わせる)。
- 起動の条件は `conclusion == 'success'` かつ `event == 'push'`(`branches: [main]`)。PR の CI が終わってもデプロイしない。
- `workflow_run` は既定ブランチの最新を checkout してしまうので、`actions/checkout` に `ref: ${{ github.event.workflow_run.head_sha }}` を渡し、**CI が検査したコミット**をデプロイする。
- `deploy` は `deploy/wrangler.jsonc`(コミット済みの生成物)を使う。CI で `deploy:config` を実行しない。`deploy:config:check` が、`board.config.json` を直したのに生成し直していない / 生成物を手で書き換えた / upstream の `wrangler.jsonc` の変更を取り込んでいない、を検出して止める。
- 前提: `board.config.json` と `deploy/` がコミット済み(`git add -f`。runbook の 0)。D1 の作成と secrets の登録は済んでいる(`mise run setup`)。**CI は D1 も secrets も作らない**。
- アクションは、upstream の `.github/workflows/ci.yml` と同じ commit SHA で固定している(タグではなく SHA。更新するときは `ci.yml` に合わせる)。checkout には `persist-credentials: false` を付け、`permissions` は `contents: read` だけにしている。
- ジョブは GitHub Environment `production` で動く。トークンは `deploy` のステップだけに渡す。

## 2. デプロイ用 API トークンの権限

Cloudflare ダッシュボードのアカウント API トークンで作る。board のデプロイ(`wrangler deploy`)に要るのは次のとおり。

| 権限 | レベル | 用途 |
|---|---|---|
| Workers Scripts | Edit | Worker のデプロイ |
| D1 | Edit | D1 のバインディングを持つ Worker のデプロイ(`wrangler deploy` が D1 を参照する) |
| Workers Routes / Zone(カスタムドメイン) | Edit(対象のゾーン) | `customDomain` の登録(`custom_domain: true` のルート)に DNS レコードと証明書が作られる |

- `customDomain` を使わない(workers.dev で公開する)場合、Zone の権限は不要になる見込み。**【未確認】** 実アカウントで確かめていない。
- 「Edit Cloudflare Workers」テンプレートは Workers Scripts・Workers Routes などを含む。D1 の権限を足す。
- 対象は、このアカウントとゾーンだけに絞る。
- **【未検証】** 最小の権限の組み合わせは実アカウントで確かめていない(特に、カスタムドメインの登録に Zone のどの権限が要るか)。足りないときは wrangler のエラーに出る権限名を手がかりに足す。最小にしたいときは、足した権限を 1 つずつ外してデプロイが通るか確かめる。
- トークンは `wrangler login`(OAuth)とは別。`mise run setup` を手元で実行するときは `wrangler login` の資格情報を使い、CI だけがこのトークンを使う。
- **BOARD の API キー・トークン(`BOARD_API_KEY` / `BOARD_API_TOKEN`)は CI に渡さない**。Worker の secret に直接入れる(`wrangler secret put`)。

## 3. GitHub Environment と Secrets

secret は**リポジトリ全体の Secrets ではなく、Environment `production` の Secrets** に置く(ジョブが `environment: production` を指定したときだけ渡る。ほかのワークフローや PR からは読めない)。

Environment の作り方(下流のリポジトリで):

1. Settings → Environments → New environment で `production` を作る。
2. 任意: Deployment branches and tags を「Selected branches」にして `main` だけにする。Required reviewers を付ければ、デプロイの前に承認を挟める。
3. 同じ画面の Environment secrets に `CLOUDFLARE_API_TOKEN` を登録する。値は運用者が自分で入れる(`gh secret set CLOUDFLARE_API_TOKEN --env production --repo <OWNER/REPO>` の対話入力。`--body` や stdin のパイプで渡さない)。

| secret(Environment `production`) | 中身 |
|---|---|
| `CLOUDFLARE_API_TOKEN` | デプロイ用の API トークン(2) |

secret の名前は変えてよい(雛形の `secrets.<名前>` も合わせる)。Environment の名前を変えるときも、雛形の `environment:` を合わせる。トークンが複数のアカウントに権限を持つときは、`board.config.json` の `accountId` を指定する。

## 4. 雛形の使い方

1. 下流のリポジトリで `.github/workflows/` を作り、雛形をコピーする。

   ```bash
   mkdir -p .github/workflows
   cp deploy.example/deploy.workflow.yml .github/workflows/deploy.yml
   ```

2. Environment `production` を作り、`CLOUDFLARE_API_TOKEN` を登録する(3)。
3. `board.config.json` と `deploy/` がコミット済みであることを確かめる(`git ls-files board.config.json deploy`)。
4. main に入れる。`ci.yml` が成功すると、続けて `deploy.yml` が起動してデプロイする。

upstream の `ci.yml` は下流でもそのまま動く(lint・test・`deploy:check` は example のダミー値で、`board.config.json` を使わない)。`github.repository == 'youyo/board'` のジョブ(追跡してはいけないファイルの点検)は、下流では自動で止まる。
`deploy.yml` は `ci.yml` の成功を待つので、`ci.yml` を無効にするとデプロイは起動しない。無効にしない。

## 5. gitleaks と ACCESS_AUD

`deploy/wrangler.jsonc` の `ACCESS_AUD`(64 桁の 16 進)は、gitleaks の既定のルールの `generic-api-key` に当たる。**AUD は秘密値ではない**(Access の JWT の `aud` に入る値。[ADR 0005](adr/0005-upstream-downstream-deploy.md) でも下流で追跡してよい値としている)。

upstream のルートの [`.gitleaks.toml`](../.gitleaks.toml) で、`deploy/wrangler.jsonc` の `"ACCESS_AUD":` の行を**許可済み**。下流は upstream を fast-forward でマージするので、このファイルを自動で受け取る。下流で `.gitleaks.toml` を足す・書き換える必要はない。pre-commit の gitleaks も、リポジトリのルートの `.gitleaks.toml` を読む。
`--no-verify` で回避せず、止まったら upstream の更新を取り込めているか(`.gitleaks.toml` が手元にあるか)を確かめる。

upstream の CI は `mise run secrets:scan`(全履歴の gitleaks)を実行する。GitHub Action の gitleaks-action は、組織ではライセンスが要るので使っていない。

## 6. 既知のつまずき

- `deploy` が `deploy/wrangler.jsonc` を見つけられない: `deploy/` が追跡されていない。`.gitignore` に入っているので `git add -f deploy/` が要る(runbook の 0)。
- `deploy:config:check` が失敗する: runbook の 10。
- `/healthz` が 503: secrets の登録漏れ。CI は secret を入れないので、`mise run setup` か `wrangler secret put` を手元で行う。
- `deploy.yml` が起動しない: `workflows: ["CI"]` が `ci.yml` の `name:` と一致しているか、`deploy.yml` が main にあるか(`workflow_run` はデフォルトブランチのワークフローだけが起動する)、`ci.yml` が main への push で成功したかを見る。
- `CLOUDFLARE_API_TOKEN` が空: secret が Environment `production` に登録されていない、または Environment の名前が雛形の `environment:` と違う。

## 7. 未検証のまとめ

- デプロイ用トークンの最小の権限(2)
- 雛形のワークフロー全体は、この repository では実際の GitHub Actions で流していない。アクションの SHA と `mise run` の各タスクの呼び方は、upstream の `ci.yml` と `mise.toml` に合わせた。特に `workflow_run` が下流で期待どおり起動するか、Environment の secrets が渡るかは [manual-verification.md の V13](manual-verification.md)
