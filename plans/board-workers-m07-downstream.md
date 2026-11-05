# M07: 下流リポジトリのデプロイ構成と、セットアップのスキル

ユーザー決定(2026-11-05): 『youyo/rovo を参考に、下流のセットアップをする際のスキルも作成して』。
これにより、ロードマップの決定済み事項『デプロイ設定: logbutler の upstream / 下流モデルは持ち込まない』を覆す(ADR 0005)。

手本: rovo(`docs/deploy.md` の 4・5、`docs/deploy-ci.md`、`deploy.example/`、`scripts/deploy-config*.mjs`、`scripts/setup.sh`、`scripts/test-setup.sh`)、logbutler(`.claude/skills/logbutler-setup/SKILL.md`)。

## 完了条件

- `mise run lint` / `mise run test` / `mise run deploy:check` が通る
- M06 の `BOARD_*` 環境変数方式(`deploy:render`・`wrangler.deploy.jsonc`)が残っていない(`BOARD_LIVE_*` は残す)
- 実在しない値・秘密をコミット対象に書いていない

## 成果物

| 区分 | 内容 |
|---|---|
| 設定 | `board.config.example.json`(プレースホルダ)、`.gitignore` に `board.config.json` と `deploy/` |
| 生成器 | `scripts/deploy-config/core.ts`(M06 の `render-wrangler-config/core.ts` の後継。検証・生成・補助)、`main.ts`(`generate` / `check` / `validate` / `get` / `set-d1` / `find-d1` / `healthz-report`) |
| スクリプト | `scripts/setup.sh`(`mise run setup`)、`scripts/deploy.sh`(`mise run deploy`) |
| mise | `setup` / `deploy:config` / `deploy:config:validate` / `deploy:config:check` / `deploy:check`(example → dry-run)/ `deploy`。`deploy:render` は削除 |
| CI | `deploy.example/deploy.workflow.yml`(下流の雛形)、`docs/deploy-ci.md`、`ci.yml` に upstream 専用ジョブ(`github.repository == 'youyo/board'`) |
| スキル | `.claude/skills/board-setup/SKILL.md`、`.agents/skills/board-setup`(symlink) |
| 文書 | `CLAUDE.md`、`README.md`、`docs/runbook.md`(書き直し)、`docs/adr/0005-upstream-downstream-deploy.md`、ロードマップ |

## テスト

| 対象 | テスト |
|---|---|
| 生成・検証・補助 | `test/unit/deploy-config.test.ts`(M06 の `render-wrangler-config.test.ts` を改修) |
| 生成 → dry-run → check の通し | `test/node-e2e/deploy-config.test.ts`(同 node-e2e の改修) |
| setup / deploy.sh | `test/node-e2e/setup.test.ts`(wrangler・mise・curl を偽物に差し替える。rovo の `test-setup.sh` に倣う) |
| スキル・文書の参照 | `test/unit/board-setup-skill.test.ts`(タスク名・パス・runbook の節の実在)、`test/node-e2e/skill-links.test.ts`(symlink) |

M06 のテストの変更は、仕様変更(環境変数方式の廃止)によるもの。入力が `BOARD_*` の環境変数から `board.config.json` に変わったので、同じ振る舞い(検証・置換・強制・値を出さない)を新しい入力で書き直した。

## 判断

- **rovo と違い `board.config.json` / `deploy/` を gitignore する**(ユーザー指示)。下流は初回だけ `git add -f`。upstream 専用 CI が追跡を点検する。
- **setup のテストは vitest の node-e2e に置いた**(rovo は bash の `test-setup.sh`)。`mise run test` に自然に含まれ、アサーションを TypeScript で書ける。
- **D1 の id は `wrangler d1 list --json` から取る**(`d1 create` の出力を解析しない)。中断後の再実行で同名の D1 があれば作らない。
- **secret の値は setup を通らない**。`wrangler secret put` が端末から直接受ける。非対話なら飛ばして案内し、失敗で終わる。
- Terraform は持ち込まない(ADR 0005)。

## 未検証(実機)

- `setup` の通し(実アカウントの `wrangler d1 create` / `deploy` / `secret put`)
- カスタムドメイン登録に要る API トークンの最小権限(`docs/deploy-ci.md`)
- 下流 CI の雛形全体(実際の GitHub Actions では流していない)
