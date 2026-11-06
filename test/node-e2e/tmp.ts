// node-e2e の作業ディレクトリ。システムの一時ディレクトリに書けない環境(サンドボックス)でも動くよう、
// 既定はリポジトリ内の .tmp/node-e2e/(gitignore 済み)。BOARD_TEST_TMPDIR で変えられる。
import { mkdirSync } from "node:fs";
import { mkdir, mkdtemp } from "node:fs/promises";
import { join, resolve } from "node:path";

const REPO = resolve(import.meta.dirname, "../..");

export function tmpBase(): string {
  const v = process.env.BOARD_TEST_TMPDIR;
  return v ? resolve(REPO, v) : join(REPO, ".tmp", "node-e2e");
}

export async function makeWorkDir(prefix: string): Promise<string> {
  const base = tmpBase();
  await mkdir(base, { recursive: true });
  return mkdtemp(join(base, prefix));
}

/** 子プロセスの env に足す。TMPDIR を作業場所へ向ける(setup.sh の mktemp など)。 */
export function childTmpEnv(): { TMPDIR: string } {
  const base = tmpBase();
  mkdirSync(base, { recursive: true });
  return { TMPDIR: base };
}
