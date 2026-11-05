/**
 * ツール登録の仕組み。1 ツール = 名前・説明・入力スキーマ(zod)・カテゴリ・ハンドラ。
 * `defineTool` で定義し、`tools/index.ts` の ALL_TOOLS に足すだけで載る。
 */
import type { McpServer, StandardSchemaWithJSON } from "@modelcontextprotocol/server";
import type { z } from "zod";
import { ArgumentError, BoardError, toErrorEnvelope } from "../errors";
import type { Repositories } from "../repository";
import type { DocumentIndex } from "../repository/document-index";
import {
  assertWithinResponseLimit,
  maxResponseTokensShape,
  resolveResponseLimit,
} from "./response-limit";

/** 読み取り / 書き込み(冪等) / 書き込み(非冪等) / 破壊的の 4 区分。 */
export type ToolCategory = "ReadOnly" | "WriteIdempotent" | "WriteNonIdempotent" | "Destructive";

export interface ToolAnnotations {
  title: string;
  readOnlyHint: boolean;
  destructiveHint?: boolean;
  idempotentHint: boolean;
  openWorldHint: boolean;
}

export function annotationsFor(category: ToolCategory, title: string): ToolAnnotations {
  switch (category) {
    case "ReadOnly":
      return { title, readOnlyHint: true, idempotentHint: true, openWorldHint: true };
    case "WriteIdempotent":
      return {
        title,
        readOnlyHint: false,
        destructiveHint: false,
        idempotentHint: true,
        openWorldHint: true,
      };
    case "WriteNonIdempotent":
      return {
        title,
        readOnlyHint: false,
        destructiveHint: false,
        idempotentHint: false,
        openWorldHint: true,
      };
    case "Destructive":
      return {
        title,
        readOnlyHint: false,
        destructiveHint: true,
        idempotentHint: true,
        openWorldHint: true,
      };
  }
}

/** ハンドラに渡す実行時の文脈。 */
export interface ToolContext {
  /** 現在時刻(テストで差し替える)。ツールは `Date.now()` を直接呼ばない。 */
  now: () => Date;
  /** MCP クライアントが呼び出しを取り消したら abort される(SDK の `ctx.mcpReq.signal`)。BOARD への通信まで渡す。 */
  signal?: AbortSignal;
  /** 22 リソースの repository(D1 + BOARD クライアント)。設定されていないサーバーでは呼ぶと例外。 */
  repositories: () => Repositories;
  /** 書類 ID → 案件 ID の逆引き(find 層)。設定されていないサーバーでは呼ぶと例外。 */
  documentIndex: () => DocumentIndex;
  /** BOARD の UI のベース URL(find 層の url の組み立てに使う)。空なら url は付けない。 */
  uiBaseUrl: string;
}

export interface ToolDefinitionInput<S extends z.ZodObject> {
  /** snake_case。接頭辞は付けない(MCP Server Portal が前置する)。 */
  name: string;
  /** annotation の title(日本語)。 */
  title: string;
  description: string;
  category: ToolCategory;
  inputSchema: S;
  /**
   * 応答サイズの上限(引数 max_response_tokens)を付けるか。読み取り(ReadOnly)ツールは既定で付ける。
   * 書き込み系には付けない(成功を失敗に見せないため)。
   */
  responseLimit?: boolean;
  /** 戻り値は JSON にして text content に入れる。構造化出力(structuredContent)は使わない。 */
  handler: (args: z.infer<S>, ctx: ToolContext) => Promise<unknown>;
}

export interface ToolDefinition {
  readonly name: string;
  readonly description: string;
  readonly category: ToolCategory;
  readonly inputSchema: z.ZodObject;
  /** 応答サイズの上限(max_response_tokens)を適用するか。 */
  readonly responseLimit: boolean;
  readonly annotations: ToolAnnotations;
  /** 引数は inputSchema で検証済みのものだけが来る。 */
  readonly handler: (args: unknown, ctx: ToolContext) => Promise<unknown>;
}

const TOOL_NAME = /^[a-z][a-z0-9]*(?:_[a-z0-9]+)*$/;

export function defineTool<S extends z.ZodObject>(input: ToolDefinitionInput<S>): ToolDefinition {
  if (!TOOL_NAME.test(input.name)) {
    throw new Error(`ツール名は snake_case にする: ${input.name}`);
  }
  const responseLimit = input.category === "ReadOnly" && input.responseLimit !== false;
  return {
    name: input.name,
    description: input.description,
    category: input.category,
    inputSchema: responseLimit
      ? input.inputSchema.extend(maxResponseTokensShape)
      : input.inputSchema,
    responseLimit,
    annotations: annotationsFor(input.category, input.title),
    handler: (args, ctx) => input.handler(args as z.infer<S>, ctx),
  };
}

export interface RegisterDeps {
  /** 現在時刻。省略時は `new Date()`。 */
  now?: () => Date;
  /** repository の取得口。省略時、ツールが `ctx.repositories()` を呼ぶと例外(generic_error)。 */
  repositories?: () => Repositories;
  documentIndex?: () => DocumentIndex;
  uiBaseUrl?: string;
}

const noRepositories = (): Repositories => {
  throw new Error("repositories are not configured for this server");
};
const noDocumentIndex = (): DocumentIndex => {
  throw new Error("documentIndex is not configured for this server");
};

function textResult(text: string, isError = false) {
  return { ...(isError ? { isError: true } : {}), content: [{ type: "text" as const, text }] };
}

/** zod の検証エラーを、どの引数が悪いか分かる 1 行にする。 */
function describeIssues(error: z.ZodError): string {
  return error.issues
    .map((i) => `${i.path.length ? i.path.join(".") : "(input)"}: ${i.message}`)
    .join("; ");
}

/**
 * SDK には「公開用のスキーマ」だけを渡し、検証は自前で行う。SDK の検証エラーはプレーンテキストの
 * isError になり、エラーエンベロープにならないため。JSON Schema は zod のものをそのまま使う。
 */
function advertiseOnly(schema: z.ZodObject): StandardSchemaWithJSON {
  const standard = schema["~standard"];
  return {
    "~standard": { ...standard, validate: (value: unknown) => ({ value }) },
  } as unknown as StandardSchemaWithJSON;
}

/**
 * 想定外の例外(generic_error にするもの)を調べられるよう、例外名と message をログに残す。応答には出さない。
 * message に API キー / トークンが混ざりうるので、Bearer と x-api-key の値は伏せる。
 */
function logUnexpected(toolName: string, e: unknown): void {
  const name = e instanceof Error ? e.name : typeof e;
  const raw = e instanceof Error ? e.message : String(e);
  const redacted = raw
    .replace(/Bearer\s+[A-Za-z0-9._~+/=-]+/gi, "Bearer [REDACTED]")
    .replace(/(x-api-key["']?\s*[:=]\s*)\S+/gi, "$1[REDACTED]");
  console.error(`tool ${toolName} failed unexpectedly: ${name}: ${redacted}`);
}

export function registerTools(
  server: McpServer,
  tools: readonly ToolDefinition[],
  deps: RegisterDeps = {},
): void {
  const seen = new Set<string>();
  for (const t of tools) {
    if (seen.has(t.name)) throw new Error(`ツール名が重複している: ${t.name}`);
    seen.add(t.name);
  }

  for (const tool of tools) {
    server.registerTool(
      tool.name,
      {
        title: tool.annotations.title,
        description: tool.description,
        inputSchema: advertiseOnly(tool.inputSchema),
        annotations: tool.annotations,
      },
      async (args: unknown, extra: { mcpReq?: { signal?: AbortSignal } } | undefined) => {
        try {
          const parsed = tool.inputSchema.safeParse(args ?? {});
          if (!parsed.success) throw new ArgumentError(describeIssues(parsed.error));
          // 上限の引数はハンドラを呼ぶ前に検証する(負の値で API を無駄に叩かない)。
          const responseLimit = tool.responseLimit
            ? resolveResponseLimit(
                (parsed.data as { max_response_tokens?: number }).max_response_tokens,
              )
            : undefined;
          const result = await tool.handler(parsed.data, {
            now: deps.now ?? (() => new Date()),
            signal: extra?.mcpReq?.signal,
            repositories: deps.repositories ?? noRepositories,
            documentIndex: deps.documentIndex ?? noDocumentIndex,
            uiBaseUrl: deps.uiBaseUrl ?? "",
          });
          const text = JSON.stringify(result ?? null);
          if (responseLimit !== undefined) assertWithinResponseLimit(text, responseLimit);
          return textResult(text);
        } catch (e) {
          if (!(e instanceof BoardError)) logUnexpected(tool.name, e);
          return textResult(JSON.stringify(toErrorEnvelope(e)), true);
        }
      },
    );
  }
}
