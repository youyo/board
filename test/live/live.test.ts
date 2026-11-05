/**
 * 実環境の確認(`mise run test:live`)。デプロイ済みの Worker に Access の service token で接続し、
 * 読み取り系の代表ツールを呼んで、応答時間・full refresh の回数・応答サイズを計測する。
 * `mise run test` と CI には含めない。BOARD の API は rate limit(3000/日、3/秒)があるので、呼び出しは直列で最小限にする。
 *
 * full refresh は実際にキャッシュ(D1)を育てる(runbook の『キャッシュの初回構築』と同じ操作)。BOARD への書き込みはしない。
 */
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { LiveClient, loadLiveConfig, summarize, type ToolCall } from "./helpers/live";

const loaded = loadLiveConfig(process.env);
if (!loaded.ok) {
  console.warn(`[live] 全体を skip します。${loaded.reason}`);
}

/** 1 回の full refresh を何回まで繰り返すか(これを超えたら失敗)。 */
const MAX_REFRESH_CALLS = 40;
/** BOARD の 1 ページの件数(src/board/paging.ts の DEFAULT_PER_PAGE)。1 回の呼び出しで進んだページ数の見積もりに使う。 */
const PER_PAGE = 100;

const report: string[] = [];
const log = (line: string) => report.push(line);
const kb = (bytes: number) => `${(bytes / 1024).toFixed(1)} KiB`;
const sec = (ms: number) => `${(ms / 1000).toFixed(2)}s`;

describe.skipIf(!loaded.ok)("live: デプロイ済みの Worker", () => {
  let client: LiveClient;

  beforeAll(() => {
    if (!loaded.ok) return;
    client = new LiveClient(loaded.config);
  });

  afterAll(() => {
    if (report.length > 0)
      console.log(`\n[live] 計測結果\n${report.map((l) => `  ${l}`).join("\n")}\n`);
  });

  const ok = (name: string, r: ToolCall) => {
    expect(r.isError, `${name}: ${r.text.slice(0, 300)}`).toBe(false);
    return r.json;
  };

  async function rowCount(resource: string): Promise<number> {
    const s = ok("cache_status", await client.call("cache_status"));
    return s.resources.find((x: { resource: string }) => x.resource === resource)?.row_count ?? 0;
  }

  it("server_info: 認証を通って名前が返る", async () => {
    const r = await client.call("server_info");
    expect(ok("server_info", r).name).toBe("board");
    log(`server_info: ${sec(r.ms)} / ${kb(r.bytes)}`);
  });

  it("cache_status: 22 リソースの状態が返る", async () => {
    const r = await client.call("cache_status");
    const json = ok("cache_status", r);
    expect(json.resources).toHaveLength(22);
    expect(json.document_scans).toHaveLength(4);
    log(`cache_status: ${sec(r.ms)} / ${kb(r.bytes)}`);
    for (const x of json.resources) {
      if (x.row_count > 0 || x.refresh_state !== "not_synced") {
        log(
          `  ${x.resource}: ${x.refresh_state}, ${x.row_count} 件, cached_at=${x.cached_at ?? "-"}`,
        );
      }
    }
  });

  const reads: Array<[string, Record<string, unknown>]> = [
    ["list_users", { limit: 5 }],
    ["list_clients", { limit: 5 }],
    ["find_projects", { limit: 5 }],
  ];
  it.each(reads)("%s(キャッシュの読み取り)", async (name, args) => {
    const r = await client.call(name, args);
    const json = ok(name, r);
    expect(Array.isArray(json.items)).toBe(true);
    expect(json.items.length).toBeLessThanOrEqual(5);
    log(`${name} ${JSON.stringify(args)}: ${sec(r.ms)} / ${kb(r.bytes)} / ${json.items.length} 件`);
  });

  /**
   * refresh=full を complete になるまで繰り返す。1 回の呼び出しで進んだページ数は cache_status の row_count の増分から見積もる。
   * 別の refresh が動いているとき(refresh_in_progress)は待って続ける。
   */
  const fulls: Array<[string, string]> = [
    ["list_users", "users"],
    ["list_clients", "clients"],
    ["find_projects", "projects"],
  ];
  it.each(fulls)(
    "%s の refresh=full が complete になるまでの回数と 1 ページあたりの時間",
    async (tool, resource) => {
      const times: number[] = [];
      const sizes: number[] = [];
      let calls = 0;
      let pages = 0;
      let before = await rowCount(resource);
      let state = "";
      while (calls < MAX_REFRESH_CALLS) {
        const r = await client.call(tool, { limit: 1, refresh: "full" });
        if (r.isError) {
          const e = r.json?.error;
          if (e?.code === "refresh_in_progress") {
            await new Promise((res) => setTimeout(res, (e.retry_after_seconds ?? 30) * 1000));
            continue;
          }
          expect.fail(`${tool}: ${e?.code ?? "?"}: ${String(e?.message ?? r.text).slice(0, 300)}`);
        }
        calls++;
        times.push(r.ms);
        sizes.push(r.bytes);
        const info = (r.json.cache as Array<{ resource: string; refresh_state?: string }>).find(
          (c) => c.resource === resource,
        );
        state = info?.refresh_state ?? "";
        if (state === "complete") break;
        const after = await rowCount(resource);
        pages += Math.max(1, Math.ceil(Math.max(0, after - before) / PER_PAGE));
        before = after;
      }
      expect(state, `${tool}: ${MAX_REFRESH_CALLS} 回で complete にならなかった`).toBe("complete");
      const rows = await rowCount(resource);
      const totalPages = Math.max(1, Math.ceil(rows / PER_PAGE));
      const t = summarize(times);
      const s = summarize(sizes);
      log(
        `${tool} refresh=full(${resource}): ${calls} 回で complete / ${rows} 件(約 ${totalPages} ページ) / ` +
          `1 回あたり 最小 ${sec(t.min)} 中央 ${sec(t.median)} 最大 ${sec(t.max)} / 合計 ${sec(t.total)} / ` +
          `1 ページあたり約 ${sec(t.total / totalPages)} / 応答 最大 ${kb(s.max)}` +
          (pages > 0 ? ` / 途中で進んだページ数の見積もり ${pages}` : ""),
      );
    },
  );

  it("完了後の cache_status: 取得したリソースは complete で、ロックは残っていない", async () => {
    const json = ok("cache_status", await client.call("cache_status"));
    for (const name of ["users", "clients", "projects"]) {
      const x = json.resources.find((r: { resource: string }) => r.resource === name);
      expect(x.refresh_state, name).toBe("complete");
      expect(x.locked, name).toBe(false);
    }
  });

  it("完了後の find_projects limit=5: 応答サイズ", async () => {
    const r = await client.call("find_projects", { limit: 5 });
    const json = ok("find_projects", r);
    expect(json.items.length).toBeGreaterThan(0);
    log(`find_projects {limit:5}(キャッシュ済み): ${sec(r.ms)} / ${kb(r.bytes)}`);
  });
});
