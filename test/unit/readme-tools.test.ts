import { describe, expect, it } from "vitest";
import readmeEn from "../../README.md?raw";
import readmeJa from "../../README_ja.md?raw";
import { ALL_TOOLS } from "../../src/mcp/tools";

/** ツール名からグループを決める(README の表の『グループ』列)。 */
function groupOf(name: string): string {
  for (const prefix of ["list", "get", "find", "cache"]) {
    if (name.startsWith(`${prefix}_`)) return prefix;
  }
  return "server";
}

/** README のツール一覧(グループごとの表)が、実際に登録されているツールと一致していること。 */
const READMES = [
  { file: "README.md", text: readmeEn, total: (n: number) => `${n} tools` },
  { file: "README_ja.md", text: readmeJa, total: (n: number) => `${n} 個` },
];

describe.each(READMES)("$file のツール一覧", ({ file, text: readme, total }) => {
  const byGroup = new Map<string, string[]>();
  for (const tool of ALL_TOOLS) {
    const list = byGroup.get(groupOf(tool.name)) ?? [];
    list.push(tool.name);
    byGroup.set(groupOf(tool.name), list);
  }

  it("総数が本文に書かれている", () => {
    expect(readme).toContain(total(ALL_TOOLS.length));
  });

  it.each([...byGroup])("グループ %s の行に、件数と全ツール名がある", (group, names) => {
    const row = readme.split("\n").find((l) => l.startsWith(`| \`${group}\` |`));
    expect(row, `${file} にグループ ${group} の行がない`).toBeDefined();
    const cells = (row ?? "").split("|").map((c) => c.trim());
    expect(cells[2]).toBe(String(names.length));
    for (const name of names) expect(row).toContain(`\`${name}\``);
  });

  it("相互リンクがある", () => {
    const other = file === "README.md" ? "README_ja.md" : "README.md";
    expect(readme.split("\n").slice(0, 3).join("\n")).toContain(`](${other})`);
  });

  it("表に載っているツール名が、実在しないものを含まない", () => {
    const real = new Set(ALL_TOOLS.map((t) => t.name));
    const rows = readme.split("\n").filter((l) => /^\| `[a-z_]+` \| \d+ \|/.test(l));
    expect(rows.length).toBe(byGroup.size);
    for (const row of rows) {
      const cell = row.split("|")[3] ?? "";
      for (const m of cell.matchAll(/`([a-z_0-9]+)`/g)) {
        expect(real.has(m[1] as string), `${file} の ${m[1]} は登録されていない`).toBe(true);
      }
    }
  });
});
