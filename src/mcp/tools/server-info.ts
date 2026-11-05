import { z } from "zod";
import { SERVER_NAME, SERVER_VERSION } from "../../info";
import { defineTool } from "../registry";

/** 疎通確認用。名前とバージョンだけを返す(設定値・秘密は返さない)。 */
export const serverInfo = defineTool({
  name: "server_info",
  title: "サーバー情報",
  description: "Returns the name and version of this MCP server. Use it to check connectivity.",
  category: "ReadOnly",
  inputSchema: z.object({}),
  handler: async () => ({ name: SERVER_NAME, version: SERVER_VERSION }),
});
