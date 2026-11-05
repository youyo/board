import type { BoardClient } from "../client";
import {
  baseQuery,
  type CommonListOptions,
  getResource,
  type ItemResult,
  type ListPaging,
  type ListResult,
  listResource,
} from "./common";

/** 案件の原価。GET /v1/project_costs の応答 1 要素(Go の ProjectCostEntity)。 */
export interface ProjectCostEntity {
  id: number;
  project_id: number;
  /** 支払い内容の説明 */
  description: string;
  /** 金額(整数) */
  cost: number;
  invoice_date: string | null;
  payment_date: string | null;
  updated_at: string;
  created_at: string;
  [key: string]: unknown;
}

export interface ProjectCostListOptions extends CommonListOptions {
  /** 案件 ID の完全一致(project_id_eq) */
  projectIdEq?: number;
}

export function buildProjectCostsQuery(
  opts: ProjectCostListOptions,
  page: number,
  perPage: number,
): string {
  return baseQuery(opts, page, perPage).intEq("project_id", opts.projectIdEq).encode();
}

export function listProjectCosts(
  client: BoardClient,
  opts: ProjectCostListOptions = {},
  paging?: ListPaging,
): Promise<ListResult<ProjectCostEntity>> {
  return listResource<ProjectCostEntity, ProjectCostListOptions>(
    client,
    "/v1/project_costs",
    buildProjectCostsQuery,
    opts,
    paging,
  );
}

export function getProjectCost(
  client: BoardClient,
  id: number,
): Promise<ItemResult<ProjectCostEntity>> {
  return getResource<ProjectCostEntity>(client, "/v1/project_costs", id);
}
