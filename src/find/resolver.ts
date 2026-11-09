/**
 * 名前 → ID の解決(Go 版 `resolver.go`)。
 * 部分一致が 0 件ならエラー、1 件ならその ID、複数なら候補(最大 5 件)を載せたエラー。先頭を黙って採用しない。
 * 解決の読み取りには refresh を付けない(Go が `ReadOptions{}` を渡しているのと同じ)。
 */
import type { ProjectEntity } from "../board/resources";
import { ArgumentError, BoardError } from "../errors";
import type { FindSession } from "./session";

/** 曖昧なときのエラー文に載せる候補の上限。 */
export const RESOLVER_CANDIDATES_MAX = 5;

export function formatCandidates(
  kind: "client" | "vendor" | "project",
  name: string,
  list: ReadonlyArray<{ id: number; name: string }>,
): string {
  const n = Math.min(list.length, RESOLVER_CANDIDATES_MAX);
  const lines = [
    `multiple ${kind}s match name ${JSON.stringify(name)} (${list.length} hits); use id to disambiguate:`,
  ];
  for (const c of list.slice(0, n)) lines.push(`  - id=${c.id} name=${JSON.stringify(c.name)}`);
  if (list.length > n) lines.push(`  ... (${list.length - n} more)`);
  return lines.join("\n");
}

async function resolve(s: FindSession, kind: "client" | "vendor", name: string): Promise<number> {
  if (name === "") throw new ArgumentError(`${kind} name is empty`);
  const list =
    kind === "client"
      ? await s.search("clients", { nameCont: name }, { noRefresh: true })
      : await s.search("vendors", { nameCont: name }, { noRefresh: true });
  const first = list[0];
  if (!first) {
    throw new BoardError("not_found", `no ${kind} matches name ${JSON.stringify(name)}`, false);
  }
  if (list.length === 1) return first.id;
  throw new ArgumentError(formatCandidates(kind, name, list));
}

export const resolveClientByName = (s: FindSession, name: string) => resolve(s, "client", name);
export const resolveVendorByName = (s: FindSession, name: string) => resolve(s, "vendor", name);

/**
 * 案件名 → 案件(部分一致)。顧客の絞り込み(clientId)があれば AND で掛ける。
 * 0 件は not_found、複数は候補つきの argument_error。請求書の絞り込みには案件番号も要るので、ID ではなくエンティティを返す。
 */
export async function resolveProjectByName(
  s: FindSession,
  name: string,
  clientId?: number,
): Promise<ProjectEntity> {
  if (name === "") throw new ArgumentError("project name is empty");
  const list = await s.search(
    "projects",
    { nameCont: name, ...(clientId ? { clientIdEq: clientId } : {}) },
    { noRefresh: true },
  );
  const first = list[0];
  if (!first) {
    throw new BoardError("not_found", `no project matches name ${JSON.stringify(name)}`, false);
  }
  if (list.length === 1) return first;
  throw new ArgumentError(formatCandidates("project", name, list));
}
