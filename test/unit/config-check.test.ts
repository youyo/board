import { env } from "cloudflare:workers";
import { describe, expect, it } from "vitest";
import { checkConfig } from "../../src/config-check";

const valid = {
  DB: env.DB,
  ACCESS_TEAM_DOMAIN: "example.cloudflareaccess.com",
  ACCESS_AUD: "aud-tag-value",
  BOARD_BASE_URL: "https://api.the-board.jp",
  BOARD_API_KEY: "api-key-value",
  BOARD_API_TOKEN: "api-token-value",
};

const NAMES = [
  "DB",
  "ACCESS_TEAM_DOMAIN",
  "ACCESS_AUD",
  "BOARD_BASE_URL",
  "BOARD_API_KEY",
  "BOARD_API_TOKEN",
] as const;

describe("checkConfig", () => {
  it("すべてそろっていれば ok", () => {
    expect(checkConfig(valid)).toEqual({ ok: true, missing: [], invalid: [] });
  });

  it.each(NAMES)("%s が無ければ missing に項目名だけが入る", (name) => {
    const { [name]: _removed, ...rest } = valid;
    expect(checkConfig(rest)).toEqual({ ok: false, missing: [name], invalid: [] });
  });

  it("空文字・空白だけの値は未設定として扱う", () => {
    const result = checkConfig({ ...valid, BOARD_API_KEY: "", BOARD_API_TOKEN: "  " });
    expect(result.ok).toBe(false);
    expect(result.missing).toEqual(["BOARD_API_KEY", "BOARD_API_TOKEN"]);
  });

  it("前後に空白・改行がある値も、空白を除いた値で検証して ok(使う側が trim する)", () => {
    const result = checkConfig({
      ...valid,
      BOARD_API_KEY: "api-key-value\n",
      BOARD_API_TOKEN: " api-token-value",
      BOARD_BASE_URL: "https://api.the-board.jp\n",
      ACCESS_AUD: "aud-tag-value ",
    });
    expect(result).toEqual({ ok: true, missing: [], invalid: [] });
  });

  it("D1 バインディングが D1 でない値(文字列など)なら missing", () => {
    expect(checkConfig({ ...valid, DB: "not-a-d1" }).missing).toEqual(["DB"]);
  });

  it("プレースホルダ(<...>)のまま残っている値は invalid", () => {
    const result = checkConfig({
      ...valid,
      ACCESS_TEAM_DOMAIN: "<team>.cloudflareaccess.com",
      ACCESS_AUD: "<access-application-aud>",
      BOARD_BASE_URL: "https://<host>",
      BOARD_API_KEY: "<api-key>",
    });
    expect(result.missing).toEqual([]);
    expect(result.invalid).toEqual([
      "ACCESS_TEAM_DOMAIN",
      "ACCESS_AUD",
      "BOARD_BASE_URL",
      "BOARD_API_KEY",
    ]);
  });

  it.each([
    ["http://api.the-board.jp", false],
    ["api.the-board.jp", false],
    ["https://api.the-board.jp/v1", false],
    ["https://api.the-board.jp?x=1", false],
    ["https://api.the-board.jp", true],
    ["https://api.the-board.jp/", true],
    ["http://localhost:8788", true],
    ["http://127.0.0.1:8788", true],
  ])("BOARD_BASE_URL %s の妥当性は %s", (url, isValid) => {
    const result = checkConfig({ ...valid, BOARD_BASE_URL: url });
    expect(result.invalid.includes("BOARD_BASE_URL")).toBe(!isValid);
  });

  it.each([
    ["https://example.cloudflareaccess.com", false],
    ["example.cloudflareaccess.com/", false],
    ["example cloudflareaccess.com", false],
    ["example.cloudflareaccess.com", true],
  ])("ACCESS_TEAM_DOMAIN %s の妥当性は %s", (host, isValid) => {
    const result = checkConfig({ ...valid, ACCESS_TEAM_DOMAIN: host });
    expect(result.invalid.includes("ACCESS_TEAM_DOMAIN")).toBe(!isValid);
  });

  describe("BOARD_UI_BASE_URL(任意)", () => {
    it("未設定・空・空白だけなら問題にしない", () => {
      expect(checkConfig(valid)).toEqual({ ok: true, missing: [], invalid: [] });
      expect(checkConfig({ ...valid, BOARD_UI_BASE_URL: "" }).ok).toBe(true);
      expect(checkConfig({ ...valid, BOARD_UI_BASE_URL: "  " }).ok).toBe(true);
    });

    it.each([
      ["https://app.board.example.com", true],
      ["https://app.board.example.com/", true],
      ["http://app.board.example.com", false],
      ["http://localhost:8788", false],
      ["app.board.example.com", false],
      ["https://app.board.example.com/ui", false],
      ["https://<ui-host>", false],
    ])("BOARD_UI_BASE_URL %s の妥当性は %s", (url, isValid) => {
      const result = checkConfig({ ...valid, BOARD_UI_BASE_URL: url });
      expect(result.missing).toEqual([]);
      expect(result.invalid.includes("BOARD_UI_BASE_URL")).toBe(!isValid);
      expect(result.ok).toBe(isValid);
    });
  });

  it("結果に値(秘密を含む)を載せない", () => {
    const result = checkConfig({
      ...valid,
      BOARD_BASE_URL: "https://<domain>/secret-path",
      BOARD_API_TOKEN: "<super-secret>",
    });
    const json = JSON.stringify(result);
    expect(json).not.toContain("super-secret");
    expect(json).not.toContain("secret-path");
  });
});
