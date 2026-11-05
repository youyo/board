/** キャッシュが保存する時刻は ISO 8601 UTC(秒まで)。文字列の大小比較がそのまま時刻の前後になる。 */
export function isoSeconds(ms: number): string {
  return new Date(ms).toISOString().replace(/\.\d{3}Z$/, "Z");
}

export type Clock = () => number;
export const systemClock: Clock = () => Date.now();
