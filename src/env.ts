/** Worker の束縛・環境変数。wrangler.jsonc と .dev.vars.example に対応する。 */
export interface Env {
  /** キャッシュ用の D1 (M03 で使う)。M01 では設定チェックの対象にするだけ。 */
  DB: D1Database;
  /** Cloudflare Access のチームドメイン (例 your-team.cloudflareaccess.com)。JWT の iss と JWKS の取得先。 */
  ACCESS_TEAM_DOMAIN: string;
  /** Access アプリケーションの AUD タグ。JWT の aud と照合する。 */
  ACCESS_AUD: string;
  /** BOARD API のベース URL (オリジンのみ。例 https://api.the-board.jp)。 */
  BOARD_BASE_URL: string;
  /** BOARD の API キー (secret)。全利用者で共有する。 */
  BOARD_API_KEY: string;
  /** BOARD の API トークン (secret)。全利用者で共有する。 */
  BOARD_API_TOKEN: string;
}

type BoardEnv = Env;

// cloudflare:workers の `env` と `Cloudflare.Env` にも同じ型を反映する。
declare global {
  namespace Cloudflare {
    interface Env extends BoardEnv {}
  }
}
