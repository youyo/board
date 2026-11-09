// 生成スクリプト(mise run openapi:fields)を実プロセスとして起動し、生成物と終了コードを確かめる。
// 実物の仕様はリポジトリに置かないので、最小の仕様(test/helpers/mini-openapi.ts)を一時ディレクトリに書いて渡し、出力先も --root で一時ディレクトリにする。
import { execFile } from "node:child_process";
import { mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { promisify } from "node:util";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { miniSpec } from "../helpers/mini-openapi";
import { childTmpEnv, makeWorkDir } from "./tmp";

const run = promisify(execFile);
const REPO = resolve(import.meta.dirname, "../..");
const MAIN = join(REPO, "scripts/openapi-fields/main.ts");

let work = "";
beforeEach(async () => {
  work = await makeWorkDir("openapi-fields-");
  await mkdir(join(work, "out/test/helpers"), { recursive: true });
  await mkdir(join(work, "out/src/board"), { recursive: true });
  await writeFile(join(work, "spec.json"), JSON.stringify(miniSpec()));
});
afterEach(async () => {
  await rm(work, { recursive: true, force: true });
});

async function main(args: string[]): Promise<{ code: number; stdout: string; stderr: string }> {
  try {
    const { stdout, stderr } = await run(process.execPath, [MAIN, ...args], {
      cwd: REPO,
      env: { PATH: process.env.PATH, HOME: process.env.HOME, ...childTmpEnv() },
    });
    return { code: 0, stdout, stderr };
  } catch (e) {
    const err = e as { code?: number; stdout?: string; stderr?: string };
    return { code: err.code ?? 1, stdout: err.stdout ?? "", stderr: err.stderr ?? "" };
  }
}

describe("openapi-fields の入口", () => {
  it("生成して、再実行しても同じ内容。--check は最新なら 0、手で書き換えられていたら 1", async () => {
    const root = join(work, "out");
    const gen = await main([join(work, "spec.json"), "--root", root]);
    expect(gen.code, gen.stderr).toBe(0);
    const fields = join(root, "test/helpers/openapi-fields.ts");
    const support = join(root, "src/board/openapi-support.ts");
    const first = await readFile(fields, "utf8");
    expect(first).toContain('OPENAPI_SPEC_VERSION = "9.9.9"');
    expect(await readFile(support, "utf8")).toContain("LIST_FILTER_SUPPORT");

    expect((await main([join(work, "spec.json"), "--root", root, "--check"])).code).toBe(0);
    await main([join(work, "spec.json"), "--root", root]);
    expect(await readFile(fields, "utf8")).toBe(first); // 再実行で差分なし

    await writeFile(fields, `${first}// 手で編集\n`);
    const stale = await main([join(work, "spec.json"), "--root", root, "--check"]);
    expect(stale.code).toBe(1);
    expect(stale.stderr).toContain("stale");
  });

  it("引数なしは使い方を出して 2、壊れた仕様は 0 以外で終わり何も書かない", async () => {
    const usage = await main([]);
    expect(usage.code).toBe(2);
    expect(usage.stderr).toContain("usage");

    await writeFile(join(work, "bad.json"), JSON.stringify({ info: { version: "1" }, paths: {} }));
    const bad = await main([join(work, "bad.json"), "--root", join(work, "out")]);
    expect(bad.code).not.toBe(0);
    await expect(
      readFile(join(work, "out/test/helpers/openapi-fields.ts"), "utf8"),
    ).rejects.toThrow();
  });
});
