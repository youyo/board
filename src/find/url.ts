/** BOARD の UI の URL(Go 版 `url.go`)。id=0 またはベースが空なら空文字(= URL を付けない)。 */

const url = (base: string, path: string, id: number) =>
  id === 0 || base === "" ? "" : `${base}/${path}/${id}/edit`;

export const projectUrl = (base: string, id: number) => url(base, "projects", id);
export const documentUrl = (base: string, id: number) => url(base, "documents", id);
export const clientUrl = (base: string, id: number) => url(base, "clients", id);
/** BOARD の UI では仕入先は /payees。 */
export const vendorUrl = (base: string, id: number) => url(base, "payees", id);

/** API のオリジンから UI のベースを導く: `://api.` を `://` に置換する(Go の app.go と同じ規則)。 */
export function uiBaseUrlFromApiBase(apiBase: string): string {
  return apiBase.replace(/\/+$/, "").replace("://api.", "://");
}

/**
 * UI のベース URL。`BOARD_UI_BASE_URL` があればそれ(末尾の / は除く)、なければ API のオリジンから導く。
 * 空文字・空白だけは未設定として扱う。
 */
export function resolveUiBaseUrl(env: {
  BOARD_BASE_URL: string;
  BOARD_UI_BASE_URL?: string;
}): string {
  const override = env.BOARD_UI_BASE_URL?.trim();
  return override ? override.replace(/\/+$/, "") : uiBaseUrlFromApiBase(env.BOARD_BASE_URL);
}
