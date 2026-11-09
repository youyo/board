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

  it("X-Ratelimit-* ヘッダーがあっても rate_limit_* は出さない(BOARD の OpenAPI に記載が無い。issue #33)", () => {
    const h = new Headers({
      "X-Ratelimit-Remaining": "99",
      "X-Ratelimit-Limit": "3000",
      "X-Ratelimit-Reset": "1776643200",
      "X-RateLimit-Remaining": "42",
      "X-Total-Count": "5",
    });
    expect(parseListMeta(h)).toEqual({ total_count: 5 });
  });

  it("空のヘッダーは {}、数値でない値は 0 扱いで出さない", () => {
    expect(parseListMeta(new Headers())).toEqual({});
    expect(parseListMeta(new Headers({ "X-Total-Count": "abc", "X-Page": "0" }))).toEqual({});
  });

  it("undefined のヘッダー(1 ページも取らなかった)は {}", () => {
    expect(parseListMeta(undefined)).toEqual({});
  });
});

describe("parseItemMeta", () => {
  it("ETag・Last-Modified だけ(ページング関係は含めない)", () => {
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
      etag: '"e"',
      last_modified: "Wed, 01 Apr 2026 12:00:00 GMT",
    });
    expect(parseItemMeta(new Headers())).toEqual({});
  });
});
