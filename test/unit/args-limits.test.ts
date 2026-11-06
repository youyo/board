/** 入力の上限(文字列の長さ・配列の件数)。超えたら argument_error で、BOARD にも D1 にも触れない。 */
import { beforeEach, describe, expect, it } from "vitest";
import { MAX_ARRAY_ITEMS, MAX_STRING_LENGTH } from "../../src/mcp/args";
import { BASE, callApiTool } from "../helpers/api-tool";
import { pagedHandler } from "../helpers/board-list";
import { resetCache } from "../helpers/d1";

beforeEach(async () => {
  await resetCache();
});

const ok = { [`GET ${BASE}/v1/clients`]: pagedHandler(() => [{ id: 1 }]) };

describe("入力の上限", () => {
  it("上限は常識的な範囲の定数", () => {
    expect(MAX_STRING_LENGTH).toBe(1000);
    expect(MAX_ARRAY_ITEMS).toBe(100);
  });

  it("文字列: 上限ちょうどは通り、超えたら argument_error(通信しない)", async () => {
    const fine = await callApiTool(
      "list_clients",
      { name_cont: "a".repeat(MAX_STRING_LENGTH) },
      ok,
    );
    expect(fine.isError).toBe(false);

    const r = await callApiTool(
      "list_clients",
      { name_cont: "a".repeat(MAX_STRING_LENGTH + 1) },
      ok,
    );
    expect(r.isError).toBe(true);
    expect(r.json.error.code).toBe("argument_error");
    expect(r.calls).toHaveLength(0);
  });

  it("文字列の配列: 件数の上限と、要素の長さの上限", async () => {
    const tags = (n: number) => Array.from({ length: n }, (_, i) => `t${i}`);
    expect((await callApiTool("list_clients", { tags: tags(MAX_ARRAY_ITEMS) }, ok)).isError).toBe(
      false,
    );

    const many = await callApiTool("list_clients", { tags: tags(MAX_ARRAY_ITEMS + 1) }, ok);
    expect(many.json.error.code).toBe("argument_error");
    const long = await callApiTool(
      "list_clients",
      { tags: ["x".repeat(MAX_STRING_LENGTH + 1)] },
      ok,
    );
    expect(long.json.error.code).toBe("argument_error");
    expect(many.calls).toHaveLength(0);
  });

  it("整数の配列の件数の上限", async () => {
    const key = { [`GET ${BASE}/v1/projects`]: pagedHandler(() => []) };
    const ids = (n: number) => Array.from({ length: n }, (_, i) => i + 1);
    const fine = await callApiTool("list_projects", { order_status_in: ids(MAX_ARRAY_ITEMS) }, key);
    const over = await callApiTool(
      "list_projects",
      { order_status_in: ids(MAX_ARRAY_ITEMS + 1) },
      key,
    );
    expect(fine.isError).toBe(false);
    expect(over.json.error.code).toBe("argument_error");
  });

  it("cache_clear の resources も件数と長さの上限を持つ", async () => {
    const over = await callApiTool("cache_clear", {
      resources: Array.from({ length: MAX_ARRAY_ITEMS + 1 }, () => "clients"),
    });
    expect(over.json.error.code).toBe("argument_error");
    expect(over.json.error.message).toContain("100"); // 上限の件数を伝える(重複などの別の理由ではない)
    const long = await callApiTool("cache_clear", {
      resources: ["c".repeat(MAX_STRING_LENGTH + 1)],
    });
    expect(long.json.error.code).toBe("argument_error");
    expect(long.json.error.message).toContain("1000");
  });

  it("find_* の文字列引数にも上限がある", async () => {
    const r = await callApiTool("find_clients", { name: "a".repeat(MAX_STRING_LENGTH + 1) });
    expect(r.isError).toBe(true);
    expect(r.json.error.code).toBe("argument_error");
    expect(r.json.error.message).toContain("1000");
    expect(r.calls).toHaveLength(0);
  });
});
