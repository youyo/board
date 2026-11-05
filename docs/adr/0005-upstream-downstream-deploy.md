# ADR 0005: upstream / 下流モデルのデプロイ

- 状態: 採用(M07、2026-11-05)
- 関連: ADR 0001(共有キーと Access)、`docs/runbook.md`、`.claude/skills/board-setup/SKILL.md`、`docs/deploy-ci.md`、姉妹リポジトリ logbutler の ADR 0011(下流モデル)、rovo の `docs/deploy.md`

## 背景

M06 までの決定済み事項は『logbutler の upstream / 下流モデルは持ち込まない。実値は `mise.local.toml` の `[env]`(`BOARD_*`)から `wrangler.deploy.jsonc`(gitignore)を生成し、デプロイは利用者が行う。board は fork して自組織にデプロイする製品ではない』だった。
M06 の方式は、実値が手元の `mise.local.toml` にしかなく、CI から再現できず、変更の履歴も残らない。

2026-11-05、ユーザーが『rovo を参考に、下流のセットアップをする際のスキルも作成して』と決めた。これは上の決定を**覆す**: 今後、このリポジトリは**公開の upstream** で、利用者は自分の private リポジトリ(下流)に upstream を取り込んでデプロイする。

## 決定

rovo の方式に倣う。

1. **下流が書く設定は `board.config.json` の 1 か所だけ。** 秘密でない値に限る(Worker 名・D1 の uuid と名前・Access のチームドメインと AUD・カスタムドメイン・任意で BOARD の URL とアカウント ID)。upstream には `board.config.example.json`(`<YOUR_...>` のプレースホルダ)を置く。
2. **`mise run deploy:config` が `board.config.json` と upstream の `wrangler.jsonc` から `deploy/wrangler.jsonc` を生成する。生成物は下流でコミットする**(デプロイ時に再生成しない。CI に特別な権限が要らない)。`workers_dev: false` とカスタムドメインを強制し、検証エラーは項目名だけを出す(値は出さない)。
3. **`mise run deploy:config:check`** が、作り直した結果とコミット済みの生成物を比べる。設定を直したのに生成し直していない / 生成物を手で書き換えた / upstream の変更を取り込んでいない、の 3 つを検出する。
4. **`mise run deploy`** は `wrangler deploy -c deploy/wrangler.jsonc`。`deploy/` が無い、または `<...>` が残っていれば止まる。upstream の `wrangler.jsonc` はダミー値なので、誤ってデプロイしない。
5. **`mise run setup`** が、前提の確認 → D1 の作成と id の書き戻し → 生成 → デプロイ → 未登録の secret の登録(隠した入力) → `/healthz` の確認を 1 本で進める。何度でも再実行できる。Access・Portal は画面の作業なので、runbook を案内して終わる。
6. **`.claude/skills/board-setup/SKILL.md`** が下流のセットアップを案内する(Codex 用に `.agents/skills/board-setup` を symlink)。
7. **`deploy.example/deploy.workflow.yml`** が下流の CI の雛形(lint → test → `deploy:config:check` → deploy)。
8. **upstream に実値を入れない**: `.gitignore` に `board.config.json` と `deploy/` を入れる。下流は最初の 1 回だけ `git add -f` で追跡する(以降は通常どおり差分が出る。`.gitignore` は書き換えない)。upstream 専用の CI ジョブ(`github.repository == 'youyo/board'`)が、これらが追跡されていないことを点検する。
9. **M06 の `BOARD_*` 環境変数方式(`deploy:render` と `wrangler.deploy.jsonc`)は残さない**。二重の仕組みにしない。`BOARD_LIVE_*`(`test:live`)は別物なので残す。

## 却下した案

- **M06 の環境変数方式**: 上のとおり、履歴・再現性・CI の面で下流の運用に向かない。
- **rovo と同じく `board.config.json` と `deploy/` を gitignore しない**: rovo は ignore せず、下流が普通にコミットする(upstream に実値を入れないのは規律と CI 点検に任せる)。board は、upstream で `git add .` した拍子に実値が入る事故を防ぐほうを優先して ignore し、下流に `git add -f` の 1 回を求める。ユーザーの指示(gitignore に入れる)に合わせた。
- **Terraform(rovo の `infra/`)**: board が作るリソースは D1 1 つ(と Worker)だけで、`wrangler d1 create` と `setup` で足りる。state の管理と専用トークンの権限を増やすほどではない。
- **下流の `wrangler.jsonc` を直接書き換える**: upstream の更新と衝突する。生成物 `deploy/wrangler.jsonc` を足すだけにする。
- **CI で `deploy:config` を実行してデプロイする**: 生成物を CI が書くと、コミットされた内容と実際にデプロイされる内容がずれる。生成物はコミットし、CI は一致の検査だけをする。

## 帰結

- 下流は upstream を `git merge --ff-only` で取り込める(下流の追加ファイルは upstream と衝突しない)。upstream の `wrangler.jsonc` が変わったら、`deploy:config` で `deploy/` を作り直してコミットする(`deploy:config:check` が検出する)。
- `wrangler.jsonc` に相対パスを持つキー(`main` と `$schema` 以外)を足したら、`scripts/deploy-config/core.ts` の書き換えと、`test/unit/deploy-config.test.ts` の許可キーを更新する。
- 下流のスキルとドキュメントは、`mise.toml`・runbook・コードと一致させる。`test/unit/board-setup-skill.test.ts` が、スキルと文書が参照するタスク名とパスの実在を確かめる。
- ロードマップの決定済み事項『デプロイ設定』は、このADR で置き換える。
