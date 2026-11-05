import { McpServer } from "@modelcontextprotocol/server";
import { SERVER_NAME, SERVER_VERSION } from "../info";
import { registerTools, type ToolDefinition } from "./registry";
import { ALL_TOOLS } from "./tools";

export interface McpServerDeps {
  /** テスト用。省略時は ALL_TOOLS。 */
  tools?: readonly ToolDefinition[];
}

/** リクエストごとに生成する(createMcpHandler の要件。同時リクエスト間で状態を共有しない)。 */
export function createMcpServer(deps: McpServerDeps = {}): McpServer {
  const server = new McpServer({ name: SERVER_NAME, version: SERVER_VERSION });
  registerTools(server, deps.tools ?? ALL_TOOLS);
  return server;
}
