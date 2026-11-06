# GitHub Actions からデプロイする(下流向け)

下流(private リポジトリ)を、ローカルの `mise run deploy` ではなく GitHub Actions からデプロイするときの手順。
全体の手順は [runbook.md](runbook.md)、下流モデルの理由は [ADR 0005](adr/0005-upstream-downstream-deploy.md)。環境固有の値はすべてプレースホルダ(`<YOUR_...>`)で書く。

**印について**: この repository では実際の Cloudflare・GitHub Actions に対して流していない。確かめていないことには「**【未検証】**」と印を付けた。印の箇所は事実として扱わず、自分の環境で確かめる。

## 1. 構成

雛形は [`deploy.example/deploy.workflow.yml`](../deploy.example/deploy.workflow.yml)。下流の `.github/workflows/deploy.yml` にコピーして使う。

| イベント | 内容 |
|---|---|
| Pull request | `mise run lint` → `mise run test` → `mise run deploy:config:check`(デプロイしない) |
| main への push | 上に続けて `mise run deploy` |

- `deploy` は `deploy/wrangler.jsonc`(コミット済みの生成物)を使う。CI で `deploy:config` を実行しない。`deploy:config:check` が、`board.config.json` を直したのに生成し直していない / 生成物を手で書き換えた / upstream の `wrangler.jsonc` の変更を取り込んでいない、を検出して止める。
- 前提: `board.config.json` と `deploy/` がコミット済み(`git add -f`。runbook の 0)。D1 の作成と secrets の登録は済んでいる(`mise run setup`)。**CI は D1 も secrets も作らない**。
- アクションは、upstream の `.github/workflows/ci.yml` と同じ commit SHA で固定している(タグではなく SHA。更新するときは `ci.yml` に合わせる)。checkout には `persist-credentials: false` を付け、`permissions` は `contents: read` だけにしている。
- トークンは `deploy` のステップだけに渡す(lint・test には渡さない)。

## 2. デプロイ用 API トークンの権限

Cloudflare ダッシュボードのアカウント API トークンで作る。board のデプロイ(`wrangler deploy`)に要るのは次のとおり。

| 権限 | レベル | 用途 |
|---|---|---|
| Workers Scripts | Edit | Worker のデプロイ |
| D1 | Edit | D1 のバインディングを持つ Worker のデプロイ(`wrangler deploy` が D1 を参照する) |
| Workers Routes / Zone(カスタムドメイン) | Edit(対象のゾーン) | `customDomain` の登録(`custom_domain: true` のルート)に DNS レコードと証明書が作られる |

- `customDomain` を使わない(workers.dev で公開する)場合、Zone の権限は不要。Account の Workers Scripts: Edit と D1: Edit で足りる。既存の Worker の再デプロイで確認した(下流の CI が main への push で `mise run deploy` まで成功した)。D1 の新規作成と、workers.dev のサブドメインの初回の有効化は確認していない(手元で `wrangler login` して行った)。
- 「Edit Cloudflare Workers」テンプレートは Workers Scripts・Workers Routes などを含む。D1 の権限を足す。
- 対象は、このアカウントとゾーンだけに絞る。
- **【未検証】** 最小の権限の組み合わせは実アカウントで確かめていない(特に、カスタムドメインの登録に Zone のどの権限が要るか)。足りないときは wrangler のエラーに出る権限名を手がかりに足す。最小にしたいときは、足した権限を 1 つずつ外してデプロイが通るか確かめる。
- トークンは `wrangler login`(OAuth)とは別。`mise run setup` を手元で実行するときは `wrangler login` の資格情報を使い、CI だけがこのトークンを使う。
- **BOARD の API キー・トークン(`BOARD_API_KEY` / `BOARD_API_TOKEN`)は CI に渡さない**。Worker の secret に直接入れる(`wrangler secret put`)。

## 3. 必要な Secrets

リポジトリの Settings → Secrets and variables → Actions に登録する。値は運用者が自分で入れる(`gh secret set CLOUDFLARE_API_TOKEN --repo <OWNER/REPO>` の対話入力。`--body` や stdin のパイプで渡さない)。

| secret | 中身 |
|---|---|
| `CLOUDFLARE_API_TOKEN` | デプロイ用の API トークン(2) |

名前は変えてよい(雛形の `secrets.<名前>` も合わせる)。トークンが複数のアカウントに権限を持つときは、`board.config.json` の `accountId` を指定する。

## 4. 雛形の使い方

1. 下流のリポジトリで `.github/workflows/` を作り、雛形をコピーする。

   ```bash
   mkdir -p .github/workflows
   cp deploy.example/deploy.workflow.yml .github/workflows/deploy.yml
   ```

2. `CLOUDFLARE_API_TOKEN` を登録する(3)。
3. `board.config.json` と `deploy/` がコミット済みであることを確かめる(`git ls-files board.config.json deploy`)。
4. PR を作って、`deploy:config:check` までが通ることを確かめる。main に入れるとデプロイが走る。

upstream の `.github/workflows/ci.yml` は下流でもそのまま動く(lint・test・`deploy:check` は example のダミー値で、`board.config.json` を使わない)。`github.repository == 'youyo/board'` のジョブ(追跡してはいけないファイルの点検)は、下流では自動で止まる。
`deploy.yml` と `ci.yml` の両方が走ると lint・test が二重になる。下流で `ci.yml` を無効にしてもよい(ファイルは編集せず、GitHub の Actions の設定で無効にする)。

## 5. 既知のつまずき

- `deploy` が `deploy/wrangler.jsonc` を見つけられない: `deploy/` が追跡されていない。`.gitignore` に入っているので `git add -f deploy/` が要る(runbook の 0)。
- `deploy:config:check` が失敗する: runbook の 10。
- `/healthz` が 503: secrets の登録漏れ。CI は secret を入れないので、`mise run setup` か `wrangler secret put` を手元で行う。

## 6. 未検証のまとめ

- デプロイ用トークンの最小の権限(2)
- 雛形のワークフロー全体は、この repository では実際の GitHub Actions で流していない。アクションの SHA と `mise run` の各タスクの呼び方は、upstream の `ci.yml` と `mise.toml` に合わせた
