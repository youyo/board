/** ステータスによる後段の絞り込み(Go 版 `filter.go`)。 */
import type { ProjectEntity } from "../board/resources";

/** 受注または納品のステータス名のどちらかが statuses に含まれる案件(OR)。statuses が空なら何もしない。 */
export function filterProjectsByStatuses(
  projects: ProjectEntity[],
  statuses: readonly string[],
): ProjectEntity[] {
  if (statuses.length === 0) return projects;
  const set = new Set(statuses);
  return projects.filter((p) => set.has(p.order_status_name) || set.has(p.delivery_status_name));
}

export function filterProjectsByStatus(projects: ProjectEntity[], status: string): ProjectEntity[] {
  return status === "" ? projects : filterProjectsByStatuses(projects, [status]);
}

/** statuses が空なら何もしない。 */
export function filterByStatuses<T>(
  items: T[],
  getStatus: (item: T) => string,
  statuses: readonly string[],
): T[] {
  if (statuses.length === 0) return items;
  const set = new Set(statuses);
  return items.filter((it) => set.has(getStatus(it)));
}
