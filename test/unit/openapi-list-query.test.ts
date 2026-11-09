/**
 * 一覧のクエリ: build*Query が出すキーは、OpenAPI(v1.9.0)の一覧の parameters の名前の部分集合であること(issue #28)。
 * 仕様に無いクエリは BOARD にエラーにならず無視される(絞り込みが効かず全件が返る)ので、送らないことを機械的に検査する。
 * 期待値は scripts/openapi-fields が仕様から生成した test/helpers/openapi-fields.ts(手で表を書かない)。
 * キャッシュ専用のキー(users の name_cont / email_cont)は API に送らないので、ここでは「出ないこと」として扱われる。
 */
import { describe, expect, it } from "vitest";
import { z } from "zod";
import { ALL_TOOLS } from "../../src/mcp/tools";
import { API_RESOURCES, type FieldDef } from "../../src/mcp/tools/api-resources";
import { SPECS } from "../../src/repository/resources";
import { OPENAPI_RESOURCES, type OpenApiResourceName } from "../helpers/openapi-fields";

const COMMON = ["updated_at_gteq", "updated_at_lteq", "include_archive_flg"] as const;

/** ツールが受ける引数それぞれに値を入れた repository のオプション(+ 共通フィルタ・ページング・response_group を全部)。 */
function everyOption(fields: readonly FieldDef[]): Record<string, unknown> {
  const opts: Record<string, unknown> = {
    updatedAtGteq: "2026-01-01 00:00:00",
    updatedAtLteq: "2026-12-31 00:00:00",
    includeArchiveFlg: true,
  };
  for (const f of fields) {
    switch (f.kind.type) {
      case "str":
        opts[f.opt] = "x";
        break;
      case "int":
        opts[f.opt] = 1;
        break;
      case "bool":
        opts[f.opt] = true;
        break;
      case "strs":
        opts[f.opt] = ["x"];
        break;
      case "ints":
        opts[f.opt] = [1];
        break;
      case "enum":
        opts[f.opt] = f.kind.values[0];
        break;
    }
  }
  return opts;
}

const queryKeys = (q: string): string[] => [...new Set(new URLSearchParams(q).keys())];

const listResources = API_RESOURCES.filter((d) => d.mode !== "document");

describe("build*Query のキーは OpenAPI の一覧のクエリ名の部分集合", () => {
  it.each(listResources.map((d) => d.resource))("%s", (resource) => {
    const spec = SPECS[resource as keyof typeof SPECS];
    if (!("buildQuery" in spec)) throw new Error(`no buildQuery: ${resource}`);
    const def = API_RESOURCES.find((d) => d.resource === resource);
    const query = (spec.buildQuery as (o: object, p: number, pp: number) => string)(
      everyOption(def?.fields ?? []),
      1,
      100,
    );
    const allowed = OPENAPI_RESOURCES[resource as OpenApiResourceName]
      .listQuery as readonly string[];
    expect(
      queryKeys(query).filter((k) => !allowed.includes(k)),
      query,
    ).toEqual([]);
  });

  it("22 リソースのうち一覧のあるものを全部見ている(18)", () => {
    expect(listResources).toHaveLength(18);
    expect(
      Object.entries(OPENAPI_RESOURCES)
        .filter(([, r]) => r.listPath !== null)
        .map(([n]) => n)
        .sort(),
    ).toEqual(listResources.map((d) => d.resource).sort());
  });
});

describe("list ツールの共通フィルタの引数は、仕様の一覧のクエリにあるリソースにだけ付く", () => {
  const byName = new Map(ALL_TOOLS.map((t) => [t.name, t]));
  it.each(listResources.map((d) => d.resource))("%s", (resource) => {
    const tool = byName.get(`list_${resource}`);
    const props = Object.keys(
      (z.toJSONSchema(tool?.inputSchema as z.ZodObject) as { properties: Record<string, unknown> })
        .properties,
    );
    const spec = OPENAPI_RESOURCES[resource as OpenApiResourceName].listQuery as readonly string[];
    expect(props.filter((p) => (COMMON as readonly string[]).includes(p)).sort()).toEqual(
      COMMON.filter((c) => spec.includes(c)).sort(),
    );
  });
});
