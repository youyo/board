// `mise run deploy:config` などの入口(Node 24 の型ストリップで直接実行する)。
// board.config.json(秘密でない値)と upstream の wrangler.jsonc から、deploy/wrangler.jsonc を生成・検査する(docs/runbook.md)。
// ロジックは core.ts。ここは引数・ファイル入出力・終了コードだけ。
//
//   node scripts/deploy-config/main.ts generate [--example [--with-domain]] [--config <path>] [--dir <dir>]
//   node scripts/deploy-config/main.ts check    [--config <path>] [--dir <dir>]
//   node scripts/deploy-config/main.ts validate [--pending-d1] [--config <path>]   設定の検証だけ(setup の前提確認)
//   node scripts/deploy-config/main.ts get <workerName|customDomain|d1.databaseId|d1.databaseName> [--config <path>]
//   node scripts/deploy-config/main.ts set-d1 <uuid> [--config <path>]               d1.databaseId を書き戻す
//   node scripts/deploy-config/main.ts find-d1 <name>                                 標準入力の `wrangler d1 list --json` から uuid を出す
//   node scripts/deploy-config/main.ts healthz-report                                 標準入力の /healthz の本文から欠けた項目名を出す
//
// 既定: 設定は <repo>/board.config.json、出力先は <repo>/deploy。--example は board.config.example.json のプレースホルダを
// ダミー値にして <repo>/.wrangler/deploy-example に作る(dry-run 専用。deploy:check)。example の customDomain は空なので workers.dev の形。
// --example --with-domain はダミーのドメインを入れ、カスタムドメインの形を <repo>/.wrangler/deploy-example-domain に作る(deploy:check が両方を検査する)。
// get customDomain は、未設定なら空行を出して終了コード 0(setup.sh が workers.dev の分岐に使う)。
// エラーには項目名だけを出し、設定の値は出さない。
import { existsSync, mkdirSync, readFileSync, renameSync, writeFileSync } from "node:fs";
import { join, relative, resolve, sep } from "node:path";
import {
  exampleConfig,
  findD1ByName,
  formatErrors,
  healthzReport,
  renderWranglerConfig,
  setD1Id,
  validateConfig,
} from "./core.ts";

const ROOT = resolve(import.meta.dirname, "../..");
const USAGE =
  "使い方: deploy-config <generate|check|validate|get|set-d1|find-d1|healthz-report> [--example] [--with-domain] [--pending-d1] [--config <path>] [--dir <dir>]";

interface Args {
  cmd: string;
  positional: string[];
  example: boolean;
  withDomain: boolean;
  pendingD1: boolean;
  config?: string;
  dir?: string;
}

function parseArgs(argv: string[]): Args {
  const [cmd, ...rest] = argv;
  if (!cmd) throw new Error("サブコマンドが要ります");
  const args: Args = { cmd, positional: [], example: false, withDomain: false, pendingD1: false };
  for (let i = 0; i < rest.length; i++) {
    const a = rest[i] as string;
    if (a === "--example") args.example = true;
    else if (a === "--with-domain") args.withDomain = true;
    else if (a === "--pending-d1") args.pendingD1 = true;
    else if (a === "--config" || a === "--dir") {
      const v = rest[++i];
      if (!v) throw new Error(`${a} にはパスが要ります`);
      if (a === "--config") args.config = v;
      else args.dir = v;
    } else if (a.startsWith("--")) throw new Error(`不明な引数: ${a}`);
    else args.positional.push(a);
  }
  return args;
}

function readJson(path: string): unknown {
  if (!existsSync(path)) {
    console.error(`設定ファイルが無い: ${path}(board.config.example.json をコピーして値を入れる)`);
    process.exit(1);
  }
  try {
    return JSON.parse(readFileSync(path, "utf8"));
  } catch {
    // JSON.parse のエラーには入力の一部が含まれうるので、メッセージは出さない
    console.error(`${path} を JSON として読めません`);
    process.exit(1);
  }
}

function fail(errors: Parameters<typeof formatErrors>[0], what: string): never {
  console.error(`${what}(値は表示しません):`);
  for (const line of formatErrors(errors)) console.error(`  - ${line}`);
  process.exit(1);
}

function stdin(): string {
  return readFileSync(0, "utf8");
}

function generate(args: Args, check: boolean): number {
  if (args.withDomain && !args.example) {
    console.error("--with-domain は --example と一緒にだけ使える");
    return 2;
  }
  const configPath = resolve(args.config ?? join(ROOT, "board.config.json"));
  const exampleDir = args.withDomain
    ? ".wrangler/deploy-example-domain"
    : ".wrangler/deploy-example";
  const dir = resolve(args.dir ?? join(ROOT, args.example ? exampleDir : "deploy"));
  const raw = args.example
    ? exampleConfig(readJson(join(ROOT, "board.config.example.json")), {
        withCustomDomain: args.withDomain,
      })
    : readJson(configPath);
  const relBase = relative(dir, ROOT).split(sep).join("/") || ".";
  const base = readFileSync(join(ROOT, "wrangler.jsonc"), "utf8");
  const result = renderWranglerConfig(base, raw, { relBase });
  if (!result.ok) fail(result.errors, "デプロイ用設定を作れません");
  const out = join(dir, "wrangler.jsonc");
  if (check) {
    if (!existsSync(out) || readFileSync(out, "utf8") !== result.text) {
      console.error(`差分あり: ${out}${existsSync(out) ? "" : "(無い)"}`);
      console.error(
        "原因は次のどれか: board.config.json を直したのに生成し直していない / 生成物を手で書き換えた / upstream の wrangler.jsonc の変更を取り込んでいない。",
      );
      console.error("`mise run deploy:config` で作り直してコミットする(手で直さない)");
      return 1;
    }
    console.log(`deploy:config:check OK(${out})`);
    return 0;
  }
  mkdirSync(dir, { recursive: true });
  writeFileSync(out, result.text);
  console.log(`生成しました: ${out}${args.example ? "(ダミー値。dry-run 専用)" : ""}`);
  return 0;
}

function main(): number {
  let args: Args;
  try {
    args = parseArgs(process.argv.slice(2));
  } catch (e) {
    console.error(e instanceof Error ? e.message : e);
    console.error(USAGE);
    return 2;
  }
  const configPath = resolve(args.config ?? join(ROOT, "board.config.json"));
  switch (args.cmd) {
    case "generate":
      return generate(args, false);
    case "check":
      if (args.example) {
        console.error("check に --example は使えません");
        return 2;
      }
      return generate(args, true);
    case "validate": {
      const r = validateConfig(readJson(configPath), { pendingD1: args.pendingD1 });
      if (!r.ok) fail(r.errors, "設定が不正です");
      return 0;
    }
    case "get": {
      const key = args.positional[0];
      const r = validateConfig(readJson(configPath), { pendingD1: true });
      if (!r.ok) fail(r.errors, "設定が不正です");
      const table: Record<string, string> = {
        workerName: r.config.workerName,
        customDomain: r.config.customDomain,
        "d1.databaseId": r.config.d1.databaseId,
        "d1.databaseName": r.config.d1.databaseName,
      };
      if (!key || !(key in table)) {
        console.error(`不明なキー(使えるのは ${Object.keys(table).join(" / ")})`);
        return 2;
      }
      console.log(table[key]);
      return 0;
    }
    case "set-d1": {
      const id = args.positional[0] ?? "";
      const text = readFileSync(configPath, "utf8");
      try {
        // 書き込み中に中断しても board.config.json を壊さないよう、一時ファイルに書いてから rename する(同じディレクトリなので原子的)。
        const next = setD1Id(text, id);
        const tmp = `${configPath}.tmp-${process.pid}`;
        writeFileSync(tmp, next);
        renameSync(tmp, configPath);
      } catch {
        console.error("d1.databaseId を書き戻せません(uuid か、設定が読めるかを確かめる)");
        return 1;
      }
      console.log("board.config.json の d1.databaseId を更新しました");
      return 0;
    }
    case "find-d1": {
      const id = findD1ByName(stdin(), args.positional[0] ?? "");
      if (id) console.log(id);
      return id ? 0 : 1;
    }
    case "healthz-report": {
      const r = healthzReport(stdin());
      if (r.missing.length > 0) console.log(`欠けている項目: ${r.missing.join(", ")}`);
      if (r.invalid.length > 0) console.log(`形式が正しくない項目: ${r.invalid.join(", ")}`);
      return 0;
    }
    default:
      console.error(`不明なサブコマンド: ${args.cmd}`);
      console.error(USAGE);
      return 2;
  }
}

process.exitCode = main();
