// `mise run openapi:fields -- <board_openapi.json のパス> [--check] [--root <dir>]` の入口(Node 24 の型ストリップで直接実行する)。
// BOARD 公式の OpenAPI 仕様(JSON)から、テスト用の test/helpers/openapi-fields.ts と、Worker が使う
// src/board/openapi-support.ts を生成する。--check は書かずに、生成物が最新かだけを確かめる(差分があれば終了コード 1)。
// --root は出力先のルート(既定はこのリポジトリ。テスト用)。
// 仕様の JSON は信頼できないデータとして読むだけで、リポジトリには置かない。
import { readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { extract, renderSupport, renderTestFields } from "./core.ts";

const args = process.argv.slice(2);
const check = args.includes("--check");
const rootIdx = args.indexOf("--root");
const rootArg = rootIdx >= 0 ? args[rootIdx + 1] : undefined;
const input = args.find((a, i) => !a.startsWith("--") && (rootIdx < 0 || i !== rootIdx + 1));
if (!input || (rootIdx >= 0 && !rootArg)) {
  console.error(
    "usage: node scripts/openapi-fields/main.ts <board_openapi.json> [--check] [--root <dir>]",
  );
  process.exit(2);
}
const ROOT = rootArg ? resolve(rootArg) : resolve(import.meta.dirname, "../..");
const OUT_TEST = resolve(ROOT, "test/helpers/openapi-fields.ts");
const OUT_SUPPORT = resolve(ROOT, "src/board/openapi-support.ts");

const extracted = extract(JSON.parse(readFileSync(resolve(input), "utf8")));
const outputs: Array<[string, string]> = [
  [OUT_TEST, renderTestFields(extracted)],
  [OUT_SUPPORT, renderSupport(extracted)],
];

let stale = false;
for (const [path, body] of outputs) {
  if (check) {
    let current = "";
    try {
      current = readFileSync(path, "utf8");
    } catch {}
    if (current !== body) {
      console.error(`stale: ${path}`);
      stale = true;
    }
  } else {
    writeFileSync(path, body);
    console.log(`wrote ${path}`);
  }
}
process.exit(stale ? 1 : 0);
