import { describe, expect, it } from "vitest";
import { QueryBuilder } from "../../src/board/query";

// Go の internal/boardapi/query_test.go の期待値を転記したもの。
describe("QueryBuilder (Go query.go)", () => {
  it("page と per_page。0 以下は送らない", () => {
    expect(new QueryBuilder().page(1, 100).encode()).toBe("page=1&per_page=100");
    expect(new QueryBuilder().page(0, 0).encode()).toBe("");
    expect(new QueryBuilder().page(-1, -5).encode()).toBe("");
  });

  it("strCont は <field>_cont。空は送らない。日本語は UTF-8 の %XX", () => {
    expect(new QueryBuilder().strCont("name", "エス").encode()).toBe(
      "name_cont=%E3%82%A8%E3%82%B9",
    );
    expect(new QueryBuilder().strCont("name", "").encode()).toBe("");
  });

  it("strEq / intEq は <field>_eq。空・0 は送らない", () => {
    expect(new QueryBuilder().strEq("custom_no", "AB-100").encode()).toBe("custom_no_eq=AB-100");
    expect(new QueryBuilder().strEq("custom_no", "").encode()).toBe("");
    expect(new QueryBuilder().intEq("client_id", 42).encode()).toBe("client_id_eq=42");
    expect(new QueryBuilder().intEq("client_id", 0).encode()).toBe("");
    expect(new QueryBuilder().intEq("client_id", undefined).encode()).toBe("");
  });

  it("intIn は <field>_in[] の繰り返し(キーは %5B%5D)。空配列は送らない", () => {
    expect(new QueryBuilder().intIn("order_status", [1, 2, 3]).encode()).toBe(
      "order_status_in%5B%5D=1&order_status_in%5B%5D=2&order_status_in%5B%5D=3",
    );
    expect(new QueryBuilder().intIn("order_status", []).encode()).toBe("");
    expect(new QueryBuilder().intIn("order_status", undefined).encode()).toBe("");
  });

  it("strIn は空要素を飛ばす", () => {
    expect(new QueryBuilder().strIn("status", ["a", "", "b"]).encode()).toBe(
      "status_in%5B%5D=a&status_in%5B%5D=b",
    );
  });

  it("dateGteq / dateLteq", () => {
    expect(
      new QueryBuilder()
        .dateGteq("updated_at", "2026-04-01")
        .dateLteq("updated_at", "2026-04-23")
        .encode(),
    ).toBe("updated_at_gteq=2026-04-01&updated_at_lteq=2026-04-23");
    expect(new QueryBuilder().dateGteq("updated_at", "").dateLteq("updated_at", "").encode()).toBe(
      "",
    );
  });

  it("日時の空白は + になる(Go の QueryEscape)", () => {
    expect(new QueryBuilder().dateGteq("updated_at", "2024-01-01 00:00:00").encode()).toBe(
      "updated_at_gteq=2024-01-01+00%3A00%3A00",
    );
  });

  it("flg01: undefined は送らない、true は 1、false は 0", () => {
    expect(new QueryBuilder().flg01("include_archive_flg", undefined).encode()).toBe("");
    expect(new QueryBuilder().flg01("include_archive_flg", true).encode()).toBe(
      "include_archive_flg=1",
    );
    expect(new QueryBuilder().flg01("include_archive_flg", false).encode()).toBe(
      "include_archive_flg=0",
    );
  });

  it("tags は tags[] の繰り返しで、空要素を飛ばす", () => {
    expect(new QueryBuilder().tags(["A", "", "B"]).encode()).toBe("tags%5B%5D=A&tags%5B%5D=B");
  });

  it("responseGroup と set。空は送らない", () => {
    expect(new QueryBuilder().responseGroup("large").encode()).toBe("response_group=large");
    expect(new QueryBuilder().responseGroup("").encode()).toBe("");
    expect(new QueryBuilder().set("x", "1").set("y", "").encode()).toBe("x=1");
  });

  it("キーは辞書順に並べ、同じキーの値は追加順(Go の url.Values.Encode)", () => {
    expect(
      new QueryBuilder()
        .strCont("name", "z")
        .page(2, 50)
        .intIn("s", [3, 1])
        .dateGteq("a", "x")
        .encode(),
    ).toBe("a_gteq=x&name_cont=z&page=2&per_page=50&s_in%5B%5D=3&s_in%5B%5D=1");
  });

  it("Go の QueryEscape と同じ文字を残す(- _ . ~ だけ。* ! ' ( ) は %XX)", () => {
    expect(new QueryBuilder().set("k", "a-b_c.d~e").encode()).toBe("k=a-b_c.d~e");
    expect(new QueryBuilder().set("k", "*!'()").encode()).toBe("k=%2A%21%27%28%29");
    expect(new QueryBuilder().set("k", "a&b=c/d?e#f%").encode()).toBe("k=a%26b%3Dc%2Fd%3Fe%23f%25");
  });

  it("不正なサロゲートを含む文字列は ArgumentError", () => {
    expect(() => new QueryBuilder().set("k", "\ud800").encode()).toThrowError(/invalid/i);
  });
});
