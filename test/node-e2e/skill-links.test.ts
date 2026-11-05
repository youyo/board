// Codex 用の .agents/skills/board-setup が .claude/skills/board-setup への symlink であることを確かめる(logbutler と同じ)。
import { lstat, readFile, readlink } from "node:fs/promises";
import { join, resolve } from "node:path";
import { describe, expect, it } from "vitest";

const REPO = resolve(import.meta.dirname, "../..");

describe("スキルの symlink", () => {
  it(".agents/skills/board-setup は ../../.claude/skills/board-setup への symlink", async () => {
    const link = join(REPO, ".agents/skills/board-setup");
    expect((await lstat(link)).isSymbolicLink()).toBe(true);
    expect(await readlink(link)).toBe("../../.claude/skills/board-setup");
  });

  it("symlink 越しに SKILL.md が読める", async () => {
    const viaLink = await readFile(join(REPO, ".agents/skills/board-setup/SKILL.md"), "utf8");
    const direct = await readFile(join(REPO, ".claude/skills/board-setup/SKILL.md"), "utf8");
    expect(viaLink).toBe(direct);
  });
});
