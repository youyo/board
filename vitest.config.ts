import { cloudflareTest } from "@cloudflare/vitest-plugin";
import { defineConfig } from "vitest/config";

// テスト専用の束縛。実在しないダミー値のみ(秘密情報ではない)。
const testBindings = {
  ACCESS_TEAM_DOMAIN: "board-test.cloudflareaccess.com",
  ACCESS_AUD: "test-access-aud",
  BOARD_BASE_URL: "https://api.board.test",
  BOARD_API_KEY: "test-board-api-key",
  BOARD_API_TOKEN: "test-board-api-token",
};

// workerd のテストは、各ファイルの最初のテストで Worker の起動とモジュールの読み込みが走る。
// ファイルを並列に流すとその起動が CPU を取り合い、CPU の少ない CI では 5 秒の既定を超える。
// 起動コストの揺れを吸収するための余裕で、個々のテストの重さの許容ではない(通常は各テスト 1 秒未満)。
const WORKERD_TEST_TIMEOUT_MS = 30_000;

function project(name: string, include: string) {
  return {
    plugins: [
      cloudflareTest({
        wrangler: { configPath: "./wrangler.jsonc" },
        miniflare: { bindings: testBindings },
      }),
    ],
    test: {
      name,
      include: [include],
      testTimeout: WORKERD_TEST_TIMEOUT_MS,
      hookTimeout: WORKERD_TEST_TIMEOUT_MS,
    },
  };
}

export default defineConfig({
  test: {
    projects: [project("unit", "test/unit/**/*.test.ts"), project("e2e", "test/e2e/**/*.test.ts")],
  },
});
