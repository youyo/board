import { describe, expect, it } from "vitest";
import { createMcpServer } from "../../src/mcp/server";
import { ALL_TOOLS } from "../../src/mcp/tools";
import { firstText, rpcTo } from "../helpers/mcp";

describe("server_info", () => {
  it("ALL_TOOLS の先頭は server_info(api 層の 40 個 + find 層の 11 個 + cache 系の 2 個が続く。M04・M05・M06)", () => {
    expect(ALL_TOOLS[0]?.name).toBe("server_info");
    expect(ALL_TOOLS).toHaveLength(54);
  });

  it("名前とバージョンだけを返す", async () => {
    const res = await rpcTo(() => createMcpServer(), "tools/call", {
      name: "server_info",
      arguments: {},
    });
    expect(res.result?.isError).not.toBe(true);
    expect(JSON.parse(firstText(res))).toEqual({ name: "board", version: "1.3.0" });
  });

  it("読み取り専用の注釈と max_response_tokens 引数を持つ", async () => {
    const res = await rpcTo(() => createMcpServer(), "tools/list");
    const tool = res.result?.tools?.find((t) => t.name === "server_info");
    expect(tool?.annotations?.readOnlyHint).toBe(true);
    expect(Object.keys(tool?.inputSchema?.properties ?? {})).toEqual(["max_response_tokens"]);
  });
});
