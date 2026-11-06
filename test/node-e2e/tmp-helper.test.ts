// node-e2e の作業ディレクトリ(サンドボックスでも書ける場所)を決めるヘルパーの確認。
import { rm, stat } from "node:fs/promises";
import { join, resolve } from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { childTmpEnv, makeWorkDir, tmpBase } from "./tmp";

const REPO = resolve(import.meta.dirname, "../..");
const saved = process.env.BOARD_TEST_TMPDIR;
afterEach(() => {
  if (saved === undefined) delete process.env.BOARD_TEST_TMPDIR;
  else process.env.BOARD_TEST_TMPDIR = saved;
});

describe("node-e2e の一時ディレクトリ", () => {
  it("BOARD_TEST_TMPDIR があればその下に作る", async () => {
    process.env.BOARD_TEST_TMPDIR = join(REPO, ".tmp", "helper-test-override");
    const d = await makeWorkDir("x-");
    try {
      expect(d.startsWith(`${join(REPO, ".tmp", "helper-test-override")}/x-`)).toBe(true);
      expect((await stat(d)).isDirectory()).toBe(true);
    } finally {
      await rm(process.env.BOARD_TEST_TMPDIR, { recursive: true, force: true });
    }
  });

  it("指定がなければ <repo>/.tmp/node-e2e の下に作る", async () => {
    delete process.env.BOARD_TEST_TMPDIR;
    expect(tmpBase()).toBe(join(REPO, ".tmp", "node-e2e"));
    const d = await makeWorkDir("y-");
    try {
      expect(d.startsWith(`${join(REPO, ".tmp", "node-e2e")}/y-`)).toBe(true);
    } finally {
      await rm(d, { recursive: true, force: true });
    }
  });

  it("子プロセスの TMPDIR はその場所になる", () => {
    process.env.BOARD_TEST_TMPDIR = join(REPO, ".tmp", "helper-test-child");
    expect(childTmpEnv().TMPDIR).toBe(join(REPO, ".tmp", "helper-test-child"));
    delete process.env.BOARD_TEST_TMPDIR;
    expect(childTmpEnv().TMPDIR).toBe(join(REPO, ".tmp", "node-e2e"));
  });
});
