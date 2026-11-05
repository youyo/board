#!/usr/bin/env bash
# mise run deploy の本体。deploy/wrangler.jsonc(deploy:config の生成物)で wrangler deploy する。
#   環境変数: BOARD_DEPLOY_DIR(既定 deploy。リポジトリ直下から実行する: mise run deploy / setup.sh は直下で呼ぶ)
# upstream の wrangler.jsonc はダミー値なので、誤ってデプロイしないよう、
#   deploy/wrangler.jsonc が無い、または <...> のプレースホルダが残っている(コメント行は除く)ときは、何も実行せずに失敗する。
set -euo pipefail

die() { echo "エラー: $*" >&2; exit 1; }

dir="${BOARD_DEPLOY_DIR:-deploy}"
cfg="$dir/wrangler.jsonc"
[ -f "$cfg" ] || die "$cfg が無い(先に board.config.json を作り、mise run deploy:config を実行する。docs/runbook.md)"

# コメント行を除いて、<...> が残る行を探す(値は表示せず、行番号だけ)
left="$(grep -nv '^[[:space:]]*//' "$cfg" | grep -E '<[^>]*>' | cut -d: -f1 | tr '\n' ' ' || true)"
[ -z "$left" ] || die "$cfg に <...> のプレースホルダが残っている(行: $left)。board.config.json を直して mise run deploy:config をやり直す"

exec pnpm exec wrangler deploy -c "$cfg"
