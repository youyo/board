import type { ToolDefinition } from "../registry";
import { serverInfo } from "./server-info";

/** 公開するツールの全体。ツールを足したら tools/list のスナップショット(mise run test:update-snapshots)を更新する。 */
export const ALL_TOOLS: readonly ToolDefinition[] = [serverInfo];
