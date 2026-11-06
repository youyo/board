// scripts/setup.sh(mise run setup)と scripts/deploy.sh の自己確認。
// wrangler(pnpm)・mise・curl をすべて偽物に差し替え、PATH を「偽物のディレクトリ + 最小のシステム」だけにして、
// 実際の wrangler / Cloudflare に届かないようにする。確かめること:
//   前提が欠けていれば何も実行せずに止まる / 再実行しても安全 / 秘密を出力・引数・ログに載せない / 順序
import { spawn } from "node:child_process";
import { chmod, cp, mkdir, readFile, rm, symlink, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { childTmpEnv, makeWorkDir } from "./tmp";

const REPO = resolve(import.meta.dirname, "../..");
const SENTINEL = "SECRET-SENTINEL-DO-NOT-PRINT";
const D1_ID = "0123abcd-4567-89ab-cdef-0123456789ab";

const VALID = {
  workerName: "e2e-board",
  customDomain: "board.e2e.example.org",
  d1: { databaseId: "<YOUR_D1_DATABASE_ID>", databaseName: "e2e-board-cache" },
  access: {
    teamDomain: "e2eteam.cloudflareaccess.com",
    aud: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", // gitleaks:allow テスト用の合成値
  },
};

let work: string;
let root: string;
let bin: string;
let state: string;

beforeEach(async () => {
  work = await makeWorkDir("setup-");
  root = join(work, "root");
  bin = join(work, "bin");
  state = join(work, "state");
  await mkdir(join(root, "scripts"), { recursive: true });
  await mkdir(bin);
  await mkdir(state);
  await cp(join(REPO, "scripts/deploy-config"), join(root, "scripts/deploy-config"), {
    recursive: true,
  });
  for (const f of ["setup.sh", "deploy.sh"]) {
    await cp(join(REPO, "scripts", f), join(root, "scripts", f));
  }
  await cp(join(REPO, "wrangler.jsonc"), join(root, "wrangler.jsonc"));
  await cp(join(REPO, "board.config.example.json"), join(root, "board.config.example.json"));
  await symlink(process.execPath, join(bin, "node"));

  // 偽の pnpm: 呼び出しを記録し、wrangler のサブコマンドを状態ファイルで模擬する。
  await script(
    "pnpm",
    `#!/bin/bash
echo "pnpm $*" >> "${state}/calls.log"
case "$*" in
  "exec wrangler whoami"*)
    if [ -f "${state}/not-logged-in" ]; then echo "You are not authenticated. Please run wrangler login."; else echo "You are logged in."; fi ;;
  "exec wrangler d1 list"*) cat "${state}/d1.json" 2>/dev/null || echo '[]' ;;
  "exec wrangler d1 create"*)
    echo '[{"uuid":"${D1_ID}","name":"e2e-board-cache"}]' > "${state}/d1.json"
    echo "Successfully created DB" ;;
  "exec wrangler deploy"*) cat "${state}/deploy.out" 2>/dev/null || echo "Deployed" ;;
  "exec wrangler secret list"*)
    if [ -f "${state}/secret-list-fails" ]; then echo "boom" >&2; exit 1; fi
    cat "${state}/secrets.json" 2>/dev/null || echo '[]' ;;
  "exec wrangler secret put "*)
    name="$5"
    # 値は標準入力(wrangler の隠した入力)から読む。スクリプトからは渡されない。長さだけ記録する。
    data="$(cat; echo x)"; data="\${data%x}"
    echo "put $name stdin_len=\${#data}" >> "${state}/puts.log"
    printf '%s' "$(cat "${state}/names.txt" 2>/dev/null)$name " > "${state}/names.txt"
    node -e 'const n=require("fs").readFileSync(process.argv[1],"utf8").trim().split(/\\s+/).filter(Boolean);console.log(JSON.stringify(n.map(x=>({name:x,type:"secret_text"}))))' "${state}/names.txt" > "${state}/secrets.json" ;;
esac
exit 0
`,
  );
  await script("mise", "#!/bin/bash\nexit 0\n");
  await script(
    "curl",
    `#!/bin/bash
echo "curl $*" >> "${state}/calls.log"
out=""; hdr=""; prev=""
for a in "$@"; do [ "$prev" = "-o" ] && out="$a"; [ "$prev" = "-D" ] && hdr="$a"; prev="$a"; done
[ -n "$hdr" ] && cat "${state}/healthz.headers" 2>/dev/null > "$hdr"
code="$(cat "${state}/healthz.code" 2>/dev/null || echo 200)"
body="$(cat "${state}/healthz.body" 2>/dev/null || echo '{"status":"ok"}')"
[ -n "$out" ] && printf '%s' "$body" > "$out"
printf '%s' "$code"
`,
  );
  await writeConfig(VALID);
});

afterEach(async () => {
  await rm(work, { recursive: true, force: true });
});

async function script(name: string, body: string): Promise<void> {
  await writeFile(join(bin, name), body);
  await chmod(join(bin, name), 0o755);
}

async function writeConfig(value: unknown): Promise<void> {
  await writeFile(join(root, "board.config.json"), `${JSON.stringify(value, null, 2)}\n`);
}

async function put(file: string, content: string): Promise<void> {
  await writeFile(join(state, file), content);
}

async function calls(): Promise<string[]> {
  try {
    return (await readFile(join(state, "calls.log"), "utf8")).split("\n").filter(Boolean);
  } catch {
    return [];
  }
}

async function puts(): Promise<string[]> {
  try {
    return (await readFile(join(state, "puts.log"), "utf8")).split("\n").filter(Boolean);
  } catch {
    return [];
  }
}

interface Result {
  code: number;
  out: string;
}

function sh(
  scriptName: string,
  args: string[],
  opts: { stdin?: string; env?: Record<string, string>; path?: string } = {},
): Promise<Result> {
  return new Promise((resolveP) => {
    const child = spawn("/bin/bash", [join(root, "scripts", scriptName), ...args], {
      cwd: root,
      env: {
        PATH: opts.path ?? `${bin}:/usr/bin:/bin`,
        HOME: work,
        ...childTmpEnv(),
        // 秘密の持ち込み確認用: スクリプトはこれを読まない・出力しない
        BOARD_API_KEY: SENTINEL,
        BOARD_API_TOKEN: SENTINEL,
        CLOUDFLARE_API_TOKEN: SENTINEL,
        BOARD_SETUP_ASSUME_TTY: "1",
        BOARD_SETUP_HEALTHZ_TRIES: "2",
        BOARD_SETUP_HEALTHZ_SLEEP: "0",
        ...opts.env,
      },
    });
    let out = "";
    child.stdout.on("data", (d) => {
      out += d;
    });
    child.stderr.on("data", (d) => {
      out += d;
    });
    child.on("close", (code) => resolveP({ code: code ?? 1, out }));
    child.stdin.end(opts.stdin ?? "");
  });
}

const setup = (args: string[] = ["--yes"], opts = {}) => sh("setup.sh", args, opts);

describe("安全確認(偽物に解決され、本物の wrangler に届かない)", () => {
  it("PATH 上の pnpm / mise / curl は偽物", async () => {
    for (const t of ["pnpm", "mise", "curl"]) {
      const r = await new Promise<string>((res) => {
        const c = spawn("/bin/bash", ["-c", `command -v ${t}`], {
          env: { PATH: `${bin}:/usr/bin:/bin` },
        });
        let o = "";
        c.stdout.on("data", (d) => {
          o += d;
        });
        c.on("close", () => res(o.trim()));
      });
      expect(r).toBe(join(bin, t));
    }
  });
});

describe("前提が欠けていれば、wrangler を呼ばずに止まる", () => {
  it("mise が無い", async () => {
    await rm(join(bin, "mise"));
    const r = await setup();
    expect(r.code).not.toBe(0);
    expect(r.out).toContain("mise");
    expect(await calls()).toEqual([]);
  });

  it("wrangler login していない(whoami 以外は呼ばない)", async () => {
    await put("not-logged-in", "1");
    const r = await setup();
    expect(r.code).not.toBe(0);
    expect(r.out).toContain("wrangler login");
    expect((await calls()).every((c) => c.includes("whoami"))).toBe(true);
  });

  it("board.config.json が無い(example のコピーを案内)", async () => {
    await rm(join(root, "board.config.json"));
    const r = await setup();
    expect(r.code).not.toBe(0);
    expect(r.out).toContain("board.config.example.json");
    expect((await calls()).every((c) => c.includes("whoami"))).toBe(true);
  });

  it("設定に不正な項目があれば項目名だけを出す(値は出さない)", async () => {
    await writeConfig({
      ...VALID,
      workerName: "BAD_NAME-SECRET",
      access: { ...VALID.access, aud: "<YOUR_ACCESS_AUD>" },
    });
    const r = await setup();
    expect(r.code).not.toBe(0);
    expect(r.out).toContain("workerName");
    expect(r.out).toContain("access.aud");
    expect(r.out).not.toContain("BAD_NAME-SECRET");
    expect((await calls()).every((c) => c.includes("whoami"))).toBe(true);
  });

  it("未知の引数は止まる", async () => {
    const r = await setup(["--nope"]);
    expect(r.code).not.toBe(0);
    expect(await calls()).toEqual([]);
  });
});

describe("通しの成功と再実行", () => {
  it("D1 作成 → 書き戻し → 生成 → deploy → secrets → /healthz の順に進み、案内で終わる", async () => {
    const r = await setup();
    expect(r.code).toBe(0);
    const c = (await calls()).map((x) => x.replace(/^pnpm exec wrangler /, ""));
    const idx = (prefix: string) => c.findIndex((x) => x.startsWith(prefix));
    expect(idx("whoami")).toBe(0);
    expect(idx("d1 create")).toBeGreaterThan(idx("d1 list"));
    expect(idx("deploy")).toBeGreaterThan(idx("d1 create"));
    expect(idx("secret list")).toBeGreaterThan(idx("deploy"));
    expect(idx("secret put")).toBeGreaterThan(idx("secret list"));
    expect(idx("curl")).toBeGreaterThan(idx("secret put"));
    expect(c.find((x) => x.startsWith("deploy"))).toContain("-c deploy/wrangler.jsonc");
    expect(await puts()).toEqual([
      "put BOARD_API_KEY stdin_len=0",
      "put BOARD_API_TOKEN stdin_len=0",
    ]);

    const written = JSON.parse(await readFile(join(root, "board.config.json"), "utf8"));
    expect(written.d1.databaseId).toBe(D1_ID);
    const deployed = await readFile(join(root, "deploy/wrangler.jsonc"), "utf8");
    expect(deployed).toContain(D1_ID);
    expect(deployed).not.toContain("<");
    expect(c.find((x) => x.startsWith("curl"))).toContain("https://board.e2e.example.org/healthz");
    // 画面の作業は runbook へ案内して終わる
    expect(r.out).toContain("docs/runbook.md");
    expect(r.out).toMatch(/Access/);
    expect(r.out).toMatch(/Portal/);
  });

  it("再実行しても安全(D1 を作り直さない・登録済みの secret に put しない・設定が変わらない)", async () => {
    expect((await setup()).code).toBe(0);
    const before = await readFile(join(root, "board.config.json"), "utf8");
    const genBefore = await readFile(join(root, "deploy/wrangler.jsonc"), "utf8");
    await rm(join(state, "calls.log"));
    await rm(join(state, "puts.log"));
    const r = await setup();
    expect(r.code).toBe(0);
    const c = await calls();
    expect(c.some((x) => x.includes("d1 create"))).toBe(false);
    expect(await puts()).toEqual([]);
    expect(await readFile(join(root, "board.config.json"), "utf8")).toBe(before);
    expect(await readFile(join(root, "deploy/wrangler.jsonc"), "utf8")).toBe(genBefore);
  });

  it("片方の secret だけ未登録なら、その 1 つだけを put する", async () => {
    await put("secrets.json", JSON.stringify([{ name: "BOARD_API_KEY", type: "secret_text" }]));
    await put("names.txt", "BOARD_API_KEY ");
    expect((await setup()).code).toBe(0);
    expect(await puts()).toEqual(["put BOARD_API_TOKEN stdin_len=0"]);
  });

  it("D1 が名前で既に在る(前回の中断)なら作らず、id だけ書き戻す", async () => {
    await put("d1.json", JSON.stringify([{ uuid: D1_ID, name: "e2e-board-cache" }]));
    const r = await setup();
    expect(r.code).toBe(0);
    expect((await calls()).some((x) => x.includes("d1 create"))).toBe(false);
    expect(JSON.parse(await readFile(join(root, "board.config.json"), "utf8")).d1.databaseId).toBe(
      D1_ID,
    );
  });

  it("d1.databaseId が設定済みなら D1 には触れない", async () => {
    await writeConfig({ ...VALID, d1: { ...VALID.d1, databaseId: D1_ID } });
    expect((await setup()).code).toBe(0);
    expect((await calls()).some((x) => x.includes("d1 "))).toBe(false);
  });
});

describe("確認(--yes なし)", () => {
  it("n と答えたら D1 を作らず、設定も変えずに止まる", async () => {
    const before = await readFile(join(root, "board.config.json"), "utf8");
    const r = await setup([], { stdin: "n\n" });
    expect(r.code).not.toBe(0);
    expect((await calls()).some((x) => x.includes("d1 create") || x.includes("deploy"))).toBe(
      false,
    );
    expect(await readFile(join(root, "board.config.json"), "utf8")).toBe(before);
  });

  it("y と答えれば進む", async () => {
    const r = await setup([], { stdin: "y\n" });
    expect(r.code).toBe(0);
    expect((await calls()).some((x) => x.includes("d1 create"))).toBe(true);
  });

  it("対話端末でなく --yes も無ければ、確認できないので止まる", async () => {
    const r = await setup([], { env: { BOARD_SETUP_ASSUME_TTY: "" } });
    expect(r.code).not.toBe(0);
    expect((await calls()).some((x) => x.includes("d1 create"))).toBe(false);
  });
});

describe("secrets", () => {
  it("secret list が読めなければ『無い』とみなさず、put しない", async () => {
    await put("secret-list-fails", "1");
    const r = await setup();
    expect(r.code).not.toBe(0);
    expect(await puts()).toEqual([]);
  });

  it("対話端末でなければ put を飛ばし、手順を案内して失敗で終わる(値は扱わない)", async () => {
    const r = await setup(["--yes"], { env: { BOARD_SETUP_ASSUME_TTY: "" } });
    expect(r.code).not.toBe(0);
    expect(await puts()).toEqual([]);
    expect(r.out).toContain("BOARD_API_KEY");
    expect(r.out).toContain("wrangler secret put");
  });

  it("秘密(環境変数に持たせた値)を出力・呼び出しの引数に載せない", async () => {
    const r = await setup();
    expect(r.code).toBe(0);
    expect(r.out).not.toContain(SENTINEL);
    expect((await calls()).join("\n")).not.toContain(SENTINEL);
  });

  it("失敗時の出力にも秘密を載せない", async () => {
    await put("healthz.code", "503");
    await put(
      "healthz.body",
      JSON.stringify({ status: "misconfigured", missing: ["BOARD_API_KEY"], invalid: [] }),
    );
    const r = await setup();
    expect(r.out).not.toContain(SENTINEL);
  });
});

describe("/healthz の確認", () => {
  it("503 なら欠けている項目名を表示し、失敗で終わる(案内も出す)", async () => {
    await put("healthz.code", "503");
    await put(
      "healthz.body",
      JSON.stringify({
        status: "misconfigured",
        missing: ["BOARD_API_TOKEN"],
        invalid: ["ACCESS_AUD"],
      }),
    );
    const r = await setup();
    expect(r.code).not.toBe(0);
    expect(r.out).toContain("BOARD_API_TOKEN");
    expect(r.out).toContain("ACCESS_AUD");
    expect(r.out).toContain("docs/runbook.md");
  });

  it("到達できないときは再試行して、確認できなかったと伝える", async () => {
    await put("healthz.code", "000");
    const r = await setup();
    expect(r.code).not.toBe(0);
    expect((await calls()).filter((x) => x.startsWith("curl")).length).toBe(2);
  });
});

describe("/healthz が Access の手前で止まるとき", () => {
  const cases: [string, string, string, string][] = [
    [
      "302 で Location が cloudflareaccess.com",
      "302",
      "HTTP/2 302\r\nlocation: https://team.cloudflareaccess.com/cdn-cgi/access/login/x\r\n\r\n",
      "",
    ],
    ["JSON でない 401", "401", "HTTP/2 401\r\n\r\n", "Unauthorized"],
    ["JSON でない 403", "403", "HTTP/2 403\r\n\r\n", "<html>Forbidden</html>"],
  ];
  it.each(cases)("%s: 失敗にせず、Access が有効と案内する", async (_n, code, headers, body) => {
    await put("healthz.code", code);
    await put("healthz.headers", headers);
    await put("healthz.body", body);
    const r = await setup();
    expect(r.code).toBe(0);
    expect((await calls()).filter((x) => x.startsWith("curl")).length).toBe(1);
    expect(r.out).toContain("Access が有効");
    expect(r.out).toContain("Bypass");
    expect(r.out).toContain("docs/runbook.md");
  });

  it("302 でも Location が Access でなければ失敗のまま", async () => {
    await put("healthz.code", "302");
    await put("healthz.headers", "HTTP/2 302\r\nlocation: https://example.org/\r\n\r\n");
    const r = await setup();
    expect(r.code).not.toBe(0);
  });

  it("JSON の 403 は Access ではないので失敗のまま", async () => {
    await put("healthz.code", "403");
    await put("healthz.body", '{"error":"x"}');
    const r = await setup();
    expect(r.code).not.toBe(0);
  });
});

// wrangler deploy の出力(workers.dev のとき): `Deployed <worker> triggers (x sec)` の次の行に `  https://<worker>.<subdomain>.workers.dev`
// (node_modules/wrangler/wrangler-dist/cli.js の triggersDeploy が logger.log する)。
const DEPLOY_OUT_WORKERS_DEV = `Total Upload: 1.00 KiB / gzip: 0.50 KiB
Uploaded e2e-board (1.00 sec)
Deployed e2e-board triggers (0.50 sec)
  https://e2e-board.e2e-sub.workers.dev
Current Version ID: 00000000-0000-0000-0000-000000000000
`;

describe("customDomain が無いとき(workers.dev で公開)", () => {
  const { customDomain: _c, ...NO_DOMAIN } = VALID;

  it.each([
    ["キーなし", NO_DOMAIN],
    ["空文字", { ...VALID, customDomain: "" }],
  ])(
    "%s: deploy の出力の workers.dev の URL で /healthz を確認し、Access の案内を出す",
    async (_n, cfg) => {
      await writeConfig(cfg);
      await put("deploy.out", DEPLOY_OUT_WORKERS_DEV);
      const r = await setup();
      expect(r.code).toBe(0);
      const curl = (await calls()).filter((x) => x.startsWith("curl"));
      expect(curl.length).toBe(1);
      expect(curl[0]).toContain("https://e2e-board.e2e-sub.workers.dev/healthz");
      expect(r.out).toContain("Enable Cloudflare Access");
      expect(r.out).toContain("https://e2e-board.e2e-sub.workers.dev/mcp");
      expect(r.out).toContain("docs/runbook.md");
      const deployed = await readFile(join(root, "deploy/wrangler.jsonc"), "utf8");
      expect(deployed).toContain('"workers_dev": true');
      expect(deployed).not.toContain('"routes"');
    },
  );

  it("Preview URL(<version>-<worker>...)は /healthz の確認に使わない", async () => {
    await writeConfig(NO_DOMAIN);
    await put(
      "deploy.out",
      "Deployed e2e-board triggers (0.50 sec)\n  https://abcd1234-e2e-board.e2e-sub.workers.dev\n",
    );
    const r = await setup();
    expect(r.code).toBe(0);
    expect((await calls()).filter((x) => x.startsWith("curl"))).toEqual([]);
  });

  it("URL が取れなければ die せず、手で確認する案内を出して続行する(curl は呼ばない)", async () => {
    await writeConfig(NO_DOMAIN);
    await put("deploy.out", "Deployed e2e-board triggers (0.50 sec)\n");
    const r = await setup();
    expect(r.code).toBe(0);
    expect((await calls()).filter((x) => x.startsWith("curl"))).toEqual([]);
    expect(r.out).toContain("https://e2e-board.<アカウントのサブドメイン>.workers.dev/healthz");
    expect(r.out).toContain("Enable Cloudflare Access");
    expect(r.out).toContain("docs/runbook.md");
  });

  it("到達できなくても失敗にしない(警告と、手で確認する案内、再実行の案内を出して exit 0)", async () => {
    await writeConfig(NO_DOMAIN);
    await put("deploy.out", DEPLOY_OUT_WORKERS_DEV);
    await put("healthz.code", "000");
    const r = await setup();
    expect(r.code).toBe(0);
    expect((await calls()).filter((x) => x.startsWith("curl")).length).toBe(2);
    expect(r.out).toContain("https://e2e-board.e2e-sub.workers.dev/healthz");
    expect(r.out).toContain("手で確認");
    expect(r.out).toContain("mise run setup");
    expect(r.out).toContain("Enable Cloudflare Access");
  });

  it("再試行の既定は約 2 分(24 回 x 5 秒)。上書きが無ければ curl を 24 回呼ぶ", async () => {
    await writeConfig(NO_DOMAIN);
    await put("deploy.out", DEPLOY_OUT_WORKERS_DEV);
    await put("healthz.code", "000");
    const r = await setup(["--yes"], {
      env: { BOARD_SETUP_HEALTHZ_TRIES: "", BOARD_SETUP_HEALTHZ_SLEEP: "0" },
    });
    expect(r.code).toBe(0);
    expect((await calls()).filter((x) => x.startsWith("curl")).length).toBe(24);
  });

  it("customDomain があるときは、到達できなければこれまでどおり失敗で終わる", async () => {
    await put("healthz.code", "000");
    const r = await setup();
    expect(r.code).not.toBe(0);
  });

  it("URL が取れた上で /healthz が 503 なら、これまでどおり失敗で終わる", async () => {
    await writeConfig(NO_DOMAIN);
    await put("deploy.out", DEPLOY_OUT_WORKERS_DEV);
    await put("healthz.code", "503");
    await put(
      "healthz.body",
      JSON.stringify({ status: "misconfigured", missing: ["BOARD_API_TOKEN"] }),
    );
    const r = await setup();
    expect(r.code).not.toBe(0);
    expect(r.out).toContain("BOARD_API_TOKEN");
  });

  it("customDomain があれば、deploy の出力に workers.dev の URL があってもカスタムドメインを使う", async () => {
    await put("deploy.out", DEPLOY_OUT_WORKERS_DEV);
    const r = await setup();
    expect(r.code).toBe(0);
    const curl = (await calls()).filter((x) => x.startsWith("curl"));
    expect(curl[0]).toContain("https://board.e2e.example.org/healthz");
  });
});

describe("deploy.sh(mise run deploy)", () => {
  it("deploy/ が無ければ wrangler を呼ばずに止まる", async () => {
    const r = await sh("deploy.sh", []);
    expect(r.code).not.toBe(0);
    expect(r.out).toContain("deploy:config");
    expect(await calls()).toEqual([]);
  });

  it("<...> が残っていれば止まる(行番号だけ表示。コメント行は無視)", async () => {
    await mkdir(join(root, "deploy"));
    await writeFile(
      join(root, "deploy/wrangler.jsonc"),
      '// <comment is fine>\n{\n  "name": "<YOUR_SECRETISH_NAME>"\n}\n',
    );
    const r = await sh("deploy.sh", []);
    expect(r.code).not.toBe(0);
    expect(r.out).toContain("3");
    expect(r.out).not.toContain("SECRETISH");
    expect(await calls()).toEqual([]);
  });

  it("生成物があれば wrangler deploy -c deploy/wrangler.jsonc を呼ぶ", async () => {
    await mkdir(join(root, "deploy"));
    await writeFile(join(root, "deploy/wrangler.jsonc"), '// <ok>\n{ "name": "x" }\n');
    const r = await sh("deploy.sh", []);
    expect(r.code).toBe(0);
    expect(await calls()).toEqual(["pnpm exec wrangler deploy -c deploy/wrangler.jsonc"]);
  });
});
