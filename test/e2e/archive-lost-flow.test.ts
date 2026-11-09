import { env } from "cloudflare:workers";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { clearJwksCache } from "../../src/auth/access-jwt";
import { clearBoardClientCache } from "../../src/board/client";
import { createTestKey, jwksResponse, signAccessJwt, type TestKey } from "../helpers/access";
import { BASE, pagedHandler } from "../helpers/board-list";
import { resetCache } from "../helpers/d1";
import { firstText, readRpc } from "../helpers/mcp";
import { jsonResponse, mockOutboundFetch } from "../helpers/mock-fetch";
import { mcpCall } from "../helpers/worker";

// issue #35: アーカイブ済み・失注の案件を、refresh(full)でもキャッシュに入れ、名前解決にも出す。
// Worker 全体(Access の JWT 検証 → /mcp → ツール → repository → D1 / 本物の BOARD クライアント)を通す。BOARD API は outbound fetch のモック。
// モックの BOARD は、実 API と同じく include_archive_flg / include_lost_flg が無いとアーカイブ済み・失注を返さない。

const TEAM = "board-test.cloudflareaccess.com";
const CERTS = `GET https://${TEAM}/cdn-cgi/access/certs`;

let key: TestKey;
let jwt: string;

beforeEach(async () => {
  clearJwksCache();
  clearBoardClientCache();
  await resetCache(env.DB);
  key = await createTestKey("kid-1");
  jwt = await signAccessJwt(key);
});
afterEach(() => {
  vi.restoreAllMocks();
  clearJwksCache();
  clearBoardClientCache();
});

async function tool(name: string, args: Record<string, unknown>, id = 1) {
  const res = await readRpc(await mcpCall(jwt, "tools/call", { name, arguments: args }, id));
  const text = firstText(res);
  return { isError: res.result?.isError === true, text, json: JSON.parse(text) };
}

const ref = (id: number, name: string) => ({ id, name, name_disp: name, custom_no: "" });

/** 通常 3 件 + アーカイブ済み 2 件(include_archive_flg=1 のとき)+ 失注 1 件(include_lost_flg=1 のとき)。 */
const NORMAL = [1, 2, 3].map((id) => ({ id, name: `案件${id}`, client: ref(7, "顧客A") }));
const ARCHIVED = [4, 5].map((id) => ({ id, name: `案件${id}`, client: ref(7, "顧客A") }));
const LOST = [{ id: 6, name: "案件6", client: ref(7, "顧客A") }];

function projectsHandler() {
  return pagedHandler(
    () => [],
    (_page, q) => {
      const rows = [
        ...NORMAL,
        ...(q.get("include_archive_flg") === "1" ? ARCHIVED : []),
        ...(q.get("include_lost_flg") === "1" ? LOST : []),
      ];
      return new Response(JSON.stringify(rows), {
        headers: { "Content-Type": "application/json", "X-Total-Count": String(rows.length) },
      });
    },
  );
}

describe("E2E: アーカイブ・失注を含めたキャッシュ", () => {
  it("list_projects refresh=full を complete まで回すと、cache_status の projects の row_count がアーカイブ・失注込みの件数になる", async () => {
    mockOutboundFetch({
      [CERTS]: () => jwksResponse(key),
      [`GET ${BASE}/v1/projects`]: projectsHandler(),
    });
    const r = await tool("list_projects", { refresh: "full" });
    expect(r.isError, r.text).toBe(false);
    expect(r.json.items).toHaveLength(6);
    expect(r.json.cache[0]).toMatchObject({ resource: "projects", refresh_state: "complete" });

    const status = await tool("cache_status", {}, 2);
    const projects = status.json.resources.find(
      (x: { resource: string }) => x.resource === "projects",
    );
    expect(projects).toMatchObject({ refresh_state: "complete", row_count: 6 });
  });

  it("find_invoices project_name + client_name: アーカイブ済み・失注の同名案件を含めて曖昧エラー(候補 2 件)になる", async () => {
    const same = [
      { id: 10, name: "保守案件", client: ref(7, "顧客A") },
      { id: 11, name: "保守案件", client: ref(7, "顧客A") },
    ];
    mockOutboundFetch({
      [CERTS]: () => jwksResponse(key),
      [`GET ${BASE}/v1/clients`]: pagedHandler(() => [{ id: 7, name: "顧客A" }]),
      [`GET ${BASE}/v1/projects`]: pagedHandler(
        () => [],
        (_page, q) => {
          const all = q.get("include_archive_flg") === "1" && q.get("include_lost_flg") === "1";
          return jsonResponse(all ? same : same.slice(0, 1));
        },
      ),
    });
    const r = await tool("find_invoices", { project_name: "保守案件", client_name: "顧客A" });
    expect(r.isError, r.text).toBe(true);
    expect(r.json.error.code).toBe("argument_error");
    expect(r.json.error.message).toContain("2 hits");
    expect(r.json.error.message).toContain("id=10");
    expect(r.json.error.message).toContain("id=11");
  });
});
