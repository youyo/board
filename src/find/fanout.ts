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

/**
 * 書類の fanout(client_name / project_name)が対象にする書類(候補)の総数の上限。
 * 書類は 1 件ずつ取る(1 件 = BOARD への 1 呼び出し)ので、大量にあると締め切りとレート制限を使い切る。
 * 超えたら書類を取りに行かず、`project_id` か `limit` での絞り込みを求める。
 */
export const FIND_DOCUMENT_CANDIDATE_CAP = 200;

export function errTooManyDocuments(field: string, value: string): ArgumentError {
  return new ArgumentError(
    `${field} "${value}" matched more than ${FIND_DOCUMENT_CANDIDATE_CAP} documents (cap=${FIND_DOCUMENT_CANDIDATE_CAP}); narrow with project_id, or set limit to cap the number of documents returned`,
  );
}
