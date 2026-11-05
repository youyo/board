import { describe, expect, it } from "vitest";
import { createMcpServer } from "../../src/mcp/server";
import { rpcTo } from "../helpers/mcp";

describe("tools/list スナップショット", () => {
  it("ツール定義(名前・説明・入力スキーマ・注釈)が変わったら意図して更新する: mise run test:update-snapshots", async () => {
    const res = await rpcTo(() => createMcpServer(), "tools/list");
    const tools = [...(res.result?.tools ?? [])].sort((a, b) => a.name.localeCompare(b.name));
    await expect(`${JSON.stringify(tools, null, 2)}\n`).toMatchFileSnapshot(
      "../snapshots/tools_list.json",
    );
  });
});
