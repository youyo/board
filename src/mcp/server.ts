import { McpServer } from "@modelcontextprotocol/server";
import { createBoardClientFromEnv } from "../board/client";
import type { Env } from "../env";
import { SERVER_NAME, SERVER_VERSION } from "../info";
import { createRepositories, type Repositories } from "../repository";
import { registerTools, type ToolDefinition } from "./registry";
import { ALL_TOOLS } from "./tools";

export interface McpServerDeps {
  /** テスト用。省略時は ALL_TOOLS。 */
  tools?: readonly ToolDefinition[];
  /** Worker の束縛・secret。repository(D1 + BOARD クライアント)を作るのに使う。 */
  env?: Pick<Env, "DB" | "BOARD_BASE_URL" | "BOARD_API_KEY" | "BOARD_API_TOKEN">;
  /** テスト用: repository の取得口を差し替える(`env` より優先)。 */
  repositories?: () => Repositories;
  /** テスト用: 現在時刻。 */
  now?: () => Date;
}

/** リクエストごとに生成する(createMcpHandler の要件。同時リクエスト間で状態を共有しない)。 */
export function createMcpServer(deps: McpServerDeps = {}): McpServer {
  const server = new McpServer({ name: SERVER_NAME, version: SERVER_VERSION });
  registerTools(server, deps.tools ?? ALL_TOOLS, {
    now: deps.now,
    repositories: deps.repositories ?? repositoriesFromEnv(deps.env),
  });
  return server;
}

/** 最初に使われたときに 1 度だけ作る。BOARD クライアントは isolate 内で共有される(createBoardClientFromEnv)。 */
function repositoriesFromEnv(env: McpServerDeps["env"]): (() => Repositories) | undefined {
  if (!env) return undefined;
  let repos: Repositories | undefined;
  return () => {
    repos ??= createRepositories({ db: env.DB, client: createBoardClientFromEnv(env) });
    return repos;
  };
}
