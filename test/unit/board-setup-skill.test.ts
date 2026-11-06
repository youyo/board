// スキル(board-setup)と下流向けの文書が参照する mise タスク名・ファイルのパス・runbook の節が実在することを確かめる。
// 存在しないタスク名やファイルを案内すると、下流のエージェントと運用者が詰まる。
import { describe, expect, it } from "vitest";
import skill from "../../.claude/skills/board-setup/SKILL.md?raw";
import claudeMd from "../../CLAUDE.md?raw";
import workflow from "../../deploy.example/deploy.workflow.yml?raw";
import deployCi from "../../docs/deploy-ci.md?raw";
import runbook from "../../docs/runbook.md?raw";
import miseToml from "../../mise.toml?raw";
import readme from "../../README.md?raw";
import readmeJa from "../../README_ja.md?raw";

// 存在確認だけなので中身は読まない(キーのパスだけを使う)。
const FILES = import.meta.glob(
  [
    "../../docs/**",
    "../../scripts/**",
    "../../plans/*",
    "../../deploy.example/*",
    "../../.claude/skills/**",
    "../../.github/workflows/*",
    "../../*.md",
    "../../*.json",
    "../../*.jsonc",
    "../../mise.toml",
    "../../.gitignore",
  ],
  { query: "?url", import: "default" },
);
const EXISTING = new Set(Object.keys(FILES).map((k) => k.replace(/^\.\.\/\.\.\//, "")));

const DOCS: Record<string, string> = {
  ".claude/skills/board-setup/SKILL.md": skill,
  "docs/runbook.md": runbook,
  "docs/deploy-ci.md": deployCi,
  "README.md": readme,
  "README_ja.md": readmeJa,
  "CLAUDE.md": claudeMd,
  "deploy.example/deploy.workflow.yml": workflow,
};

function miseTasks(): Set<string> {
  const names = new Set<string>();
  for (const m of miseToml.matchAll(/^\[tasks\.("([^"]+)"|([A-Za-z0-9_-]+))\]/gm)) {
    names.add((m[2] ?? m[3]) as string);
  }
  return names;
}

describe("board-setup スキルの形", () => {
  it("frontmatter: name がディレクトリ名と同じで、description に TRIGGER / DO NOT TRIGGER がある", () => {
    const fm = /^---\nname: (.+)\ndescription: (.+)\n---\n/.exec(skill);
    expect(fm).not.toBeNull();
    expect(fm?.[1]).toBe("board-setup");
    expect(fm?.[2]).toContain("TRIGGER when");
    expect(fm?.[2]).toContain("DO NOT TRIGGER when");
  });

  it("扱う場面と切り分けの症状を網羅している", () => {
    const desc = skill.split("\n")[2] as string;
    for (const word of [
      "新規セットアップ",
      "失敗・中断からの再開",
      "稼働中の設定変更",
      "upstream の更新の取り込み",
      "障害の切り分け",
      "/healthz が 503",
      "401 / 403",
      "refresh_in_progress",
      "rate_limited",
      "project_unresolved",
      "D1 のクエリ上限",
    ]) {
      expect(desc, word).toContain(word);
    }
    for (const word of ["締め切り", "Free プラン", "refresh=full", "service token", "Portal"]) {
      expect(skill, word).toContain(word);
    }
  });

  it("秘密の扱いと実行の境界が書かれている", () => {
    expect(skill).toContain("BOARD_API_KEY");
    expect(skill).toContain("隠した入力");
    expect(skill).toContain("コミットと push");
  });
});

describe("文書が参照する mise タスクは実在する", () => {
  const tasks = miseTasks();
  it("mise.toml からタスク名を取れる", () => {
    expect(tasks.has("setup")).toBe(true);
    expect(tasks.has("deploy:config:check")).toBe(true);
  });

  for (const [file, text] of Object.entries(DOCS)) {
    it(file, () => {
      const used = new Set<string>();
      for (const m of text.matchAll(/mise run ([a-z][a-z0-9:-]*)/g)) used.add(m[1] as string);
      expect([...used].filter((t) => !tasks.has(t))).toEqual([]);
    });
  }

  it("廃止した deploy:render と BOARD_* 環境変数方式を、どの文書も案内しない", () => {
    for (const [file, text] of Object.entries(DOCS)) {
      expect(text, file).not.toContain("deploy:render");
      expect(text, file).not.toContain("wrangler.deploy.jsonc");
      expect(text, file).not.toMatch(/BOARD_(WORKER_NAME|D1_|ACCESS_|CUSTOM_DOMAIN)/);
    }
  });
});

describe("文書が参照するパスは実在する", () => {
  const PATH_RE =
    /`((?:docs|scripts|plans|deploy\.example|\.claude|\.github)\/[A-Za-z0-9_./-]*[A-Za-z0-9_])`/g;
  // 生成物・利用者が作るもの・ディレクトリの接頭辞は存在確認の対象外
  const SKIP = new Set(["deploy/wrangler.jsonc", ".github/workflows/deploy.yml"]);

  for (const [file, text] of Object.entries(DOCS)) {
    it(file, () => {
      const missing: string[] = [];
      for (const m of text.matchAll(PATH_RE)) {
        const p = m[1] as string;
        if (SKIP.has(p)) continue;
        const exists =
          EXISTING.has(p) || [...EXISTING].some((e) => e.startsWith(`${p}/`)) || p.endsWith("/");
        if (!exists) missing.push(p);
      }
      expect(missing).toEqual([]);
    });
  }

  it("スキルが名前で挙げる主要なファイルが実在する", () => {
    for (const p of [
      "docs/runbook.md",
      "docs/deploy-ci.md",
      "docs/adr/0005-upstream-downstream-deploy.md",
      "docs/adr/0001-shared-key-and-access-managed-oauth.md",
      "docs/adr/0002-d1-cache-and-resumable-refresh.md",
      "docs/manual-verification.md",
      "scripts/setup.sh",
      "scripts/deploy.sh",
      "scripts/deploy-config/core.ts",
      "deploy.example/deploy.workflow.yml",
      "board.config.example.json",
    ]) {
      expect(EXISTING.has(p), p).toBe(true);
    }
  });
});

describe("runbook の節番号の参照は実在する", () => {
  const sections = new Set([...runbook.matchAll(/^## (\d+)\. /gm)].map((m) => m[1] as string));

  it("runbook に 0〜10 の節がある", () => {
    for (let i = 0; i <= 10; i++) expect(sections.has(String(i)), `## ${i}.`).toBe(true);
  });

  it("スキルの『runbook の N』『docs/runbook.md の N』は存在する節を指す", () => {
    const refs = [...skill.matchAll(/(?:runbook(?:\.md)?|`docs\/runbook\.md`) の (\d+)/g)].map(
      (m) => m[1] as string,
    );
    expect(refs.length).toBeGreaterThan(5);
    expect(refs.filter((n) => !sections.has(n))).toEqual([]);
  });
});

describe("下流 CI の雛形", () => {
  it("lint → test → deploy:config:check → deploy の順で、アクションは SHA で固定している", () => {
    const order = [
      "mise run lint",
      "mise run test",
      "mise run deploy:config:check",
      "mise run deploy",
    ];
    let at = -1;
    for (const step of order) {
      const i = workflow.indexOf(`run: ${step}`, at + 1);
      expect(i, step).toBeGreaterThan(at);
      at = i;
    }
    const uses = [...workflow.matchAll(/uses: (\S+)/g)].map((m) => m[1] as string);
    expect(uses.length).toBeGreaterThan(0);
    for (const u of uses) expect(u, u).toMatch(/@[0-9a-f]{40}$/);
  });

  it("upstream の ci.yml と同じ SHA", async () => {
    const ci = (await import("../../.github/workflows/ci.yml?raw")).default;
    const shas = (t: string) =>
      new Set([...t.matchAll(/uses: (\S+@[0-9a-f]{40})/g)].map((m) => m[1]));
    for (const s of shas(workflow)) expect(shas(ci).has(s), s).toBe(true);
  });

  it("トークンは GitHub Secrets から読み、デプロイのステップだけに渡す", () => {
    expect(workflow).toMatch(/\$\{\{ secrets\.CLOUDFLARE_API_TOKEN \}\}/);
    expect(workflow.match(/CLOUDFLARE_API_TOKEN: /g)?.length).toBe(1);
  });
});

describe("upstream 専用の CI", () => {
  it("ci.yml の upstream 専用ジョブは github.repository == 'youyo/board' で限定している", async () => {
    const ci = (await import("../../.github/workflows/ci.yml?raw")).default;
    expect(ci).toContain("if: github.repository == 'youyo/board'");
  });

  it(".gitignore に board.config.json と deploy/ が入っている", async () => {
    const ignore = (await import("../../.gitignore?raw")).default.split("\n");
    expect(ignore).toContain("board.config.json");
    expect(ignore).toContain("deploy/");
  });
});
