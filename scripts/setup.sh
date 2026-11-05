#!/usr/bin/env bash
# mise run setup の本体。初回のデプロイを 1 本にまとめる(既存の部品の薄い組み合わせ)。何度でも再実行できる。
#   使い方: scripts/setup.sh [--yes]
#     --yes  D1 の作成と board.config.json への書き戻しの確認を省く
#   環境変数: BOARD_CONFIG(既定 board.config.json)/ BOARD_DEPLOY_DIR(既定 deploy)
#     BOARD_SETUP_ASSUME_TTY / BOARD_SETUP_HEALTHZ_TRIES / BOARD_SETUP_HEALTHZ_SLEEP はテスト用(対話端末とみなす / /healthz の再試行回数と待ち秒)
#
# 流れ: 前提の確認 → D1 がなければ作成して id を board.config.json に書き戻す → deploy:config → deploy → 未登録の secret だけ登録 → /healthz → 画面の作業の案内
#   前提に 1 つでも欠けていたら、何も実行しない(wrangler の変更系を呼ばない)。
#   secret の値はこのスクリプトを通らない: wrangler secret put の隠した入力を端末から直接受ける。引数・env・ファイル・ログに載せない。
#   Access アプリ・Managed OAuth・Portal は画面の作業。docs/runbook.md の該当節を案内して終わる。
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
die() { echo "エラー: $*" >&2; exit 1; }

yes=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --yes) yes=1; shift ;;
    *) die "未知のオプション: $1(使えるのは --yes)" ;;
  esac
done

cd "$root"
cfg="${BOARD_CONFIG:-board.config.json}"
dir="${BOARD_DEPLOY_DIR:-deploy}"
cli() { node "$root/scripts/deploy-config/main.ts" "$@"; }
wr() { pnpm exec wrangler "$@"; }
interactive() { [ -t 0 ] || [ -n "${BOARD_SETUP_ASSUME_TTY:-}" ]; }

confirm() { # confirm <質問>
  [ "$yes" -eq 1 ] && return 0
  interactive || die "確認が要る($1)。--yes を付けるか、端末で実行する"
  printf '%s [y/N] ' "$1" >&2
  local ans=""
  read -r ans || ans=""
  case "$ans" in y | Y | yes | YES) return 0 ;; *) echo "中止した(何も変更していない部分もある。再実行できる)" >&2; return 1 ;; esac
}

# ---- 1. 前提の確認(何かを変更する前に) ------------------------------------------
echo "==> 前提を確認する"
missing=0
need_fail() { echo "  NG $*" >&2; missing=1; }
tools_ok=1
for t in mise pnpm node; do
  command -v "$t" > /dev/null 2>&1 || { need_fail "$t が無い(https://mise.jdx.dev。mise install)"; tools_ok=0; }
done
if [ "$tools_ok" -eq 1 ]; then
  who="$(wr whoami 2>&1 < /dev/null)" || who="not authenticated"
  case "$who" in *"not authenticated"* | *"Not logged in"* | *"not logged in"*) need_fail "wrangler にログインしていない(pnpm exec wrangler login)" ;; esac
  # 設定の検証。エラーは項目名だけ(値は出さない)。D1 の id は未作成でもよい(次で作る)
  cli validate --pending-d1 --config "$cfg" || { need_fail "$cfg が不正、または無い(board.config.example.json をコピーして値を入れる。docs/runbook.md の 1)"; }
fi
[ "$missing" -eq 0 ] || exit 1
workername="$(cli get workerName --config "$cfg")"
domain="$(cli get customDomain --config "$cfg")"
echo "  OK"

# ---- 2. D1 ---------------------------------------------------------------------
d1id="$(cli get d1.databaseId --config "$cfg")"
d1name="$(cli get d1.databaseName --config "$cfg")"
if [ -z "$d1id" ]; then
  echo "==> D1 '$d1name'"
  list="$(wr d1 list --json < /dev/null)" || die "wrangler d1 list に失敗した"
  d1id="$(printf '%s' "$list" | cli find-d1 "$d1name" || true)"
  if [ -n "$d1id" ]; then
    echo "  同名の D1 が既にある(前回の中断の続き)。作成は飛ばす"
    confirm "既存の D1 '$d1name' の id を $cfg に書き戻す?" || exit 1
  else
    confirm "D1 '$d1name' を作成し、得た id を $cfg に書き戻す?" || exit 1
    wr d1 create "$d1name" < /dev/null || die "wrangler d1 create に失敗した"
    list="$(wr d1 list --json < /dev/null)" || die "wrangler d1 list に失敗した"
    d1id="$(printf '%s' "$list" | cli find-d1 "$d1name" || true)"
    [ -n "$d1id" ] || die "作成した D1 '$d1name' の id を取得できなかった(wrangler d1 list --json で確かめ、$cfg の d1.databaseId に手で書く)"
  fi
  cli set-d1 "$d1id" --config "$cfg"
fi

# ---- 3. 設定の生成 → デプロイ ----------------------------------------------------
echo "==> deploy:config($dir/wrangler.jsonc を生成する)"
cli generate --config "$cfg" --dir "$dir"

echo "==> deploy"
BOARD_DEPLOY_DIR="$dir" bash "$root/scripts/deploy.sh"

# ---- 4. secrets(未登録のものだけ) -------------------------------------------------
echo "==> secrets(未登録のものだけ登録する。値は隠した入力で、このスクリプトは受け取らない)"
list="$(wr secret list -c "$dir/wrangler.jsonc" --format json < /dev/null)" || die "wrangler secret list に失敗した(未登録とはみなさない。Worker のデプロイと wrangler login を確かめる)"
todo=""
for name in BOARD_API_KEY BOARD_API_TOKEN; do
  if printf '%s' "$list" | grep -Eq "\"name\"[[:space:]]*:[[:space:]]*\"$name\""; then
    echo "  $name: 登録済み(上書きしない)"
  else
    todo="$todo $name"
  fi
done
secrets_pending=""
for name in $todo; do
  if interactive; then
    echo "  $name を登録する(入力は表示されない)"
    wr secret put "$name" -c "$dir/wrangler.jsonc" || die "$name の登録に失敗した"
  else
    secrets_pending="$secrets_pending $name"
  fi
done

# ---- 5. /healthz ---------------------------------------------------------------
echo "==> https://$domain/healthz を確認する"
tries="${BOARD_SETUP_HEALTHZ_TRIES:-6}"
nap="${BOARD_SETUP_HEALTHZ_SLEEP:-5}"
body="$(mktemp)"
trap 'rm -f "$body"' EXIT
health="unreachable"
i=0
while [ "$i" -lt "$tries" ]; do
  i=$((i + 1))
  code="$(curl -sS -m 10 -o "$body" -w '%{http_code}' "https://$domain/healthz" 2> /dev/null)" || code="000"
  if [ "$code" = "200" ]; then health="ok"; break; fi
  if [ "$code" = "503" ]; then health="misconfigured"; break; fi
  [ "$i" -lt "$tries" ] && sleep "$nap"
done
case "$health" in
  ok) echo "  OK(設定はそろっている)" ;;
  misconfigured)
    echo "  NG 503: 設定が足りない(項目名のみ)" >&2
    cli healthz-report < "$body" >&2 || true
    ;;
  *) echo "  NG 到達できなかった(DNS・証明書の反映待ちかもしれない。しばらくしてもう一度 mise run setup)" >&2 ;;
esac

# ---- 6. 画面の作業の案内 ---------------------------------------------------------
cat <<MSG

==> 残りは画面の作業(docs/runbook.md)
[1] Access のアプリケーション(MCP server application)を作り、Managed OAuth を有効にする(runbook の 5)
    公開ホスト名: $domain
    作ったら AUD タグを $cfg の access.aud に入れ、mise run deploy:config && mise run deploy(runbook の 5 の 3)
[2] MCP Server Portal に https://$domain/mcp を登録する(runbook の 6)
[3] 動作確認と、キャッシュの初回構築(runbook の 7、8)
MSG
if [ -n "$secrets_pending" ]; then
  echo
  echo "未登録の secret:$secrets_pending(対話端末でないため登録を飛ばした)。端末で次を実行する:" >&2
  for name in $secrets_pending; do echo "  pnpm exec wrangler secret put $name -c $dir/wrangler.jsonc" >&2; done
  exit 1
fi
[ "$health" = "ok" ] || exit 1
echo
echo "完了($workername)。"
