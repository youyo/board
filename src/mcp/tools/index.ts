import type { ToolDefinition } from "../registry";
import { API_TOOLS } from "./api";
import { CACHE_TOOLS } from "./cache";
import { FIND_TOOLS } from "./find";
import { serverInfo } from "./server-info";

/** 公開するツールの全体。ツールを足したら tools/list のスナップショット(mise run test:update-snapshots)を更新する。 */
export const ALL_TOOLS: readonly ToolDefinition[] = [
  serverInfo,
  ...API_TOOLS,
  ...FIND_TOOLS,
  ...CACHE_TOOLS,
];
