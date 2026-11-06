import { McpServer } from "@modelcontextprotocol/server";
import { createBoardClientFromEnv } from "../board/client";
import type { Env } from "../env";
import { resolveUiBaseUrl } from "../find/url";
import { SERVER_NAME, SERVER_VERSION } from "../info";
import { createRepositories, type Repositories } from "../repository";
import { createDocumentIndex, type DocumentIndex } from "../repository/document-index";
import { registerTools, type ToolDefinition } from "./registry";
import { ALL_TOOLS } from "./tools";

export interface McpServerDeps {
  /** テスト用。省略時は ALL_TOOLS。 */
  tools?: readonly ToolDefinition[];
  /** Worker の束縛・secret。repository(D1 + BOARD クライアント)を作るのに使う。 */
  env?: Pick<
    Env,
    "DB" | "BOARD_BASE_URL" | "BOARD_UI_BASE_URL" | "BOARD_API_KEY" | "BOARD_API_TOKEN"
  >;
  /** テスト用: D1 の取得口を差し替える(`env` より優先)。cache 系ツールが使う。 */
  db?: () => D1Database;
  /** テスト用: repository の取得口を差し替える(`env` より優先)。 */
  repositories?: () => Repositories;
  /** テスト用: 書類 → 案件の逆引きの取得口を差し替える(`env` より優先)。 */
  documentIndex?: () => DocumentIndex;
  /** テスト用: BOARD の UI のベース URL(省略時は `env.BOARD_UI_BASE_URL`、なければ `env.BOARD_BASE_URL` から導く)。 */
  uiBaseUrl?: string;
  /** テスト用: 現在時刻。 */
  now?: () => Date;
  /** ログの相関 ID(cf-ray など)。 */
  requestId?: string;
  /** 操作した主体(Access の JWT の email、service token なら Client ID)。書き込み系の操作のログにだけ出す。 */
  actor?: string;
}

/** リクエストごとに生成する(createMcpHandler の要件。同時リクエスト間で状態を共有しない)。 */
export function createMcpServer(deps: McpServerDeps = {}): McpServer {
  const server = new McpServer({ name: SERVER_NAME, version: SERVER_VERSION });
  registerTools(server, deps.tools ?? ALL_TOOLS, {
    now: deps.now,
    requestId: deps.requestId,
    actor: deps.actor,
    db: deps.db ?? (deps.env ? dbFromEnv(deps.env) : undefined),
    repositories: deps.repositories ?? repositoriesFromEnv(deps.env),
    documentIndex: deps.documentIndex ?? documentIndexFromEnv(deps.env),
    uiBaseUrl: deps.uiBaseUrl ?? (deps.env ? resolveUiBaseUrl(deps.env) : ""),
  });
  return server;
}

function dbFromEnv(env: { DB: D1Database }): () => D1Database {
  return () => env.DB;
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

function documentIndexFromEnv(env: McpServerDeps["env"]): (() => DocumentIndex) | undefined {
  if (!env) return undefined;
  let index: DocumentIndex | undefined;
  return () => {
    index ??= createDocumentIndex({ db: env.DB, client: createBoardClientFromEnv(env) });
    return index;
  };
}
