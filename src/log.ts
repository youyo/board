/**
 * 構造化ログ(JSON 1 行)。Workers Logs で request_id や event を検索できるようにする。
 * JWT・BOARD の API キー / トークンは渡さないこと(呼び出し側の責任。値は伏せてから渡す)。
 */
export type LogLevel = "info" | "warn" | "error";

/** リクエストの相関 ID。cf-ray があればそれ、なければ生成する。 */
export function requestIdOf(request: Request): string {
  return request.headers.get("cf-ray") || crypto.randomUUID();
}

export function logEvent(
  level: LogLevel,
  event: string,
  requestId: string | undefined,
  fields: Record<string, unknown> = {},
): void {
  const line = JSON.stringify({
    level,
    event,
    ...(requestId ? { request_id: requestId } : {}),
    ...fields,
  });
  if (level === "error") console.error(line);
  else if (level === "warn") console.warn(line);
  else console.info(line);
}
