/** scripts/openapi-fields のテスト用の、22 リソースぶんの最小の OpenAPI 仕様(実物の仕様は置かない)。 */
import { RESOURCE_PATHS } from "../../scripts/openapi-fields/core";

/** 22 リソースぶんの最小の仕様を作る。overrides でリソースごとに差し替える。 */
export function miniSpec(overrides: Record<string, unknown> = {}) {
  const paths: Record<string, unknown> = {};
  for (const [name, p] of Object.entries(RESOURCE_PATHS)) {
    if (p.list) {
      paths[p.list] = {
        get: {
          parameters: [
            { name: "name_cont", in: "query", schema: { type: "string" } },
            { name: "updated_at_gteq", in: "query", schema: { type: "string" } },
            { name: "tags[]", in: "query", schema: { type: "string" } },
            { name: "x_header", in: "header", schema: { type: "string" } },
          ],
          responses: {
            "200": {
              content: {
                "application/json": {
                  schema: {
                    type: "array",
                    items: { properties: { id: { type: "integer" }, name: { type: "string" } } },
                  },
                },
              },
            },
          },
        },
      };
    }
    if (p.get) {
      paths[p.get] = {
        get: {
          responses: {
            "200": {
              content: {
                "application/json": {
                  schema: {
                    allOf: [
                      { properties: { id: { type: "integer" } } },
                      { $ref: "#/components/schemas/Extra" },
                    ],
                  },
                },
              },
            },
          },
        },
      };
    }
    void name;
  }
  return {
    info: { version: "9.9.9" },
    paths: { ...paths, ...overrides },
    components: {
      schemas: {
        Extra: {
          properties: {
            memo_get_only: { type: "string" },
            child: { $ref: "#/components/schemas/Child" },
            rows: { type: "array", items: { properties: { cost: { type: "integer" } } } },
          },
        },
        Child: {
          type: "object",
          properties: { id: { type: "integer" }, label: { type: "string" } },
        },
      },
    },
  };
}
