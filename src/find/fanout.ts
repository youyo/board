import { ArgumentError } from "../errors";

/**
 * fanout 検索の名前解決(client_name / project_name)が一度に許容するマッチ件数の上限(Go の `fanoutResolveCap`)。
 * 超えたらエラーで絞り込みを求める: 後段で書類を 1 件ずつ取るので、大量にマッチすると BOARD のレート制限と締め切りを使い切る。
 */
export const FANOUT_RESOLVE_CAP = 5;

export function errFanoutTooMany(field: string, value: string, n: number): ArgumentError {
  return new ArgumentError(
    `${field} "${value}" matched ${n} entities (cap=${FANOUT_RESOLVE_CAP}); narrow the query or specify a more specific id/project_id to reduce fanout`,
  );
}
