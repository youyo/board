/**
 * scripts/openapi-fields/core.ts(OpenAPI → 応答キー・クエリ名の抽出)の単体。
 * 仕様の JSON はリポジトリに置かないので、$ref / allOf / 配列 / 入れ子 / 一覧と単体取得の和集合を含む小さな仕様で確かめる。
 */
import { describe, expect, it } from "vitest";
import {
  extract,
  RESOURCE_PATHS,
  renderSupport,
  renderTestFields,
} from "../../scripts/openapi-fields/core";
import { RESOURCES } from "../../src/board/resources";
import { miniSpec } from "../helpers/mini-openapi";

describe("extract", () => {
  it("リソースの表は src の RESOURCES(22 リソース・パス)と一致する", () => {
    expect(Object.keys(RESOURCE_PATHS)).toEqual(RESOURCES.map((r) => r.name));
    for (const r of RESOURCES) {
      const p = RESOURCE_PATHS[r.name];
      // list のあるリソースは list のパス、無い(書類)ものは単体取得のパスが `/v1` + 仕様のパスに一致する
      const spec = p?.list ?? p?.get?.replace(/\/\{id\}$/, "");
      expect(`/v1${spec}`, r.name).toBe(r.path);
    }
  });

  it("一覧と単体取得の応答キーの和集合・入れ子のキー・一覧のクエリ名(query の parameters だけ。`x[]` はそのまま)", () => {
    const e = extract(miniSpec());
    expect(e.version).toBe("9.9.9");
    const c = e.resources.clients;
    expect(c?.responseKeys).toEqual(["id", "name", "memo_get_only", "child", "rows"]);
    expect(c?.nestedKeys).toEqual({ child: ["id", "label"], rows: ["cost"] });
    expect(c?.listQuery).toEqual(["name_cont", "updated_at_gteq", "tags[]"]);
    // 書類は一覧が無い(単体取得だけ)
    expect(e.resources.estimates).toMatchObject({ listPath: null, listQuery: [] });
    expect(e.resources.estimates?.responseKeys).toContain("memo_get_only");
    // 単体取得が無いもの(users)は一覧だけ
    expect(e.resources.users).toMatchObject({ getPath: null, responseKeys: ["id", "name"] });
  });

  it("parameters の $ref をたどる", () => {
    const spec = miniSpec({
      "/groups": {
        get: {
          parameters: [{ $ref: "#/components/parameters/P" }],
          responses: {
            "200": {
              content: {
                "application/json": {
                  schema: { type: "array", items: { properties: { id: {} } } },
                },
              },
            },
          },
        },
      },
    }) as { components: Record<string, unknown> };
    spec.components.parameters = { P: { name: "include_archive_flg", in: "query", schema: {} } };
    expect(extract(spec).resources.groups?.listQuery).toEqual(["include_archive_flg"]);
  });

  it("仕様に GET が無い・200 のスキーマが無い・バージョンが無いときは、黙って空にせず失敗する", () => {
    const noGet = miniSpec();
    delete (noGet.paths as Record<string, unknown>)["/users"];
    expect(() => extract(noGet)).toThrow(/GET \/users is missing/);
    expect(() => extract({ ...miniSpec(), info: {} })).toThrow(/info.version/);
    expect(() =>
      extract(miniSpec({ "/users": { get: { parameters: [], responses: {} } } })),
    ).toThrow(/no 200/);
  });
});

describe("render", () => {
  it("バージョンと取得元を先頭に書き、共通フィルタの有無は listQuery から導く", () => {
    const e = extract(miniSpec());
    const t = renderTestFields(e);
    expect(t.split("\n").slice(0, 3).join("\n")).toContain("v9.9.9");
    expect(t).toContain("https://developers.the-board.jp/doc/board_openapi.json");
    expect(t).toContain('OPENAPI_SPEC_VERSION = "9.9.9"');
    const s = renderSupport(e);
    expect(s).toContain(
      "clients: { updatedAtGteq: true, updatedAtLteq: false, includeArchiveFlg: false },",
    );
    expect(s).not.toContain("estimates:"); // 一覧の無い書類は載せない
  });

  it("同じ入力からは同じ出力(再実行しても差分が出ない)", () => {
    const e = extract(miniSpec());
    expect(renderTestFields(e)).toBe(renderTestFields(extract(miniSpec())));
    expect(renderSupport(e)).toBe(renderSupport(extract(miniSpec())));
  });
});
