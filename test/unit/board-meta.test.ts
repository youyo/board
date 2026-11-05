import { describe, expect, it } from "vitest";
import { parseItemMeta, parseListMeta } from "../../src/board/meta";

// Go の internal/boardapi/result_test.go の転記。
describe("parseListMeta", () => {
  it("件数・ページ・Retry-After・ETag・Last-Modified を写す", () => {
    const h = new Headers({
      "X-Total-Count": "299",
      "X-Page": "3",
      "X-Per-Page": "50",
      "Retry-After": "10",
      ETag: 'W/"abc123"',
      "Last-Modified": "Wed, 01 Apr 2026 12:00:00 GMT",
    });
    expect(parseListMeta(h)).toEqual({
      total_count: 299,
      page: 3,
      per_page: 50,
      retry_after: 10,
      etag: 'W/"abc123"',
      last_modified: "Wed, 01 Apr 2026 12:00:00 GMT",
    });
  });

  it("レート制限ヘッダーは X-Ratelimit-* でも X-RateLimit-* でも読める", () => {
    expect(
      parseListMeta(new Headers({ "X-Ratelimit-Remaining": "99", "X-Ratelimit-Limit": "3000" })),
    ).toEqual({ rate_limit_remaining: 99, rate_limit_limit: 3000 });
    expect(
      parseListMeta(new Headers({ "X-RateLimit-Remaining": "42", "X-RateLimit-Limit": "1000" })),
    ).toEqual({ rate_limit_remaining: 42, rate_limit_limit: 1000 });
  });

  it("rate_limit_reset は epoch 秒を ISO 8601 (UTC・秒まで) にする", () => {
    expect(parseListMeta(new Headers({ "X-Ratelimit-Reset": "1776643200" }))).toEqual({
      rate_limit_reset: "2026-04-20T00:00:00Z",
    });
  });

  it("空のヘッダーは {}、数値でない値は 0 扱いで出さない", () => {
    expect(parseListMeta(new Headers())).toEqual({});
    expect(
      parseListMeta(
        new Headers({ "X-Total-Count": "abc", "X-Ratelimit-Reset": "soon", "X-Page": "0" }),
      ),
    ).toEqual({});
  });

  it("undefined のヘッダー(1 ページも取らなかった)は {}", () => {
    expect(parseListMeta(undefined)).toEqual({});
  });
});

describe("parseItemMeta", () => {
  it("レート制限・ETag・Last-Modified だけ(ページング関係は含めない)", () => {
    const h = new Headers({
      "X-Ratelimit-Remaining": "7",
      "X-Ratelimit-Limit": "3000",
      "X-Ratelimit-Reset": "1776643200",
      ETag: '"e"',
      "Last-Modified": "Wed, 01 Apr 2026 12:00:00 GMT",
      "X-Total-Count": "5",
      "Retry-After": "3",
    });
    expect(parseItemMeta(h)).toEqual({
      rate_limit_remaining: 7,
      rate_limit_limit: 3000,
      rate_limit_reset: "2026-04-20T00:00:00Z",
      etag: '"e"',
      last_modified: "Wed, 01 Apr 2026 12:00:00 GMT",
    });
    expect(parseItemMeta(new Headers())).toEqual({});
  });
});
