/**
 * find 層の純粋な部分(Go 版 `internal/service/find/` の contract_status / filter / url / fanout_cap / resolver の整形 / types の検証)。
 * Go の `*_test.go` のケースを転記して固定する。
 */
import { describe, expect, it } from "vitest";
import {
  expandContractStatus,
  filterProjectsByContractStatus,
} from "../../src/find/contract-status";
import { errFanoutTooMany, FANOUT_RESOLVE_CAP } from "../../src/find/fanout";
import {
  filterByStatuses,
  filterProjectsByStatus,
  filterProjectsByStatuses,
} from "../../src/find/filter";
import { formatCandidates, RESOLVER_CANDIDATES_MAX } from "../../src/find/resolver";
import {
  validateClientQuery,
  validateDocumentQuery,
  validateInvoiceQuery,
  validateProjectQuery,
} from "../../src/find/types";
import {
  clientUrl,
  documentUrl,
  projectUrl,
  uiBaseUrlFromApiBase,
  vendorUrl,
} from "../../src/find/url";

const P = (id: number, extra: Record<string, unknown> = {}) =>
  ({ id, name: `P${id}`, ...extra }) as never;

describe("expandContractStatus(Go: contract_status_test.go)", () => {
  it("active = 納品の 未着手 / 着手中 / 納品済(受注は見ない)", () => {
    const r = expandContractStatus("active");
    expect(r.order).toEqual([]);
    expect([...r.delivery].sort()).toEqual(["未着手", "着手中", "納品済"].sort());
  });
  it("ended = 検収済", () => {
    expect(expandContractStatus("ended")).toEqual({ delivery: ["検収済"], order: [] });
  });
  it("prospect = 受注の 見積中(高/中/低/除)", () => {
    const r = expandContractStatus("prospect");
    expect(r.delivery).toEqual([]);
    expect([...r.order].sort()).toEqual(
      ["見積中(高)", "見積中(中)", "見積中(低)", "見積中(除)"].sort(),
    );
  });
  it("all = active + ended の納品 4 種 + prospect の受注 4 種", () => {
    const r = expandContractStatus("all");
    expect(r.delivery).toHaveLength(4);
    expect(r.order).toHaveLength(4);
    expect(r.delivery).toContain("検収済");
    expect(r.order).toContain("見積中(除)");
  });
  it("大文字小文字と前後の空白を無視する", () => {
    expect(expandContractStatus("ACTIVE").delivery).toHaveLength(3);
    expect(expandContractStatus(" active ").delivery).toHaveLength(3);
  });
  it("未知の値は 'unknown contract_status' と valid 一覧を含む argument_error", () => {
    try {
      expandContractStatus("unknown");
      expect.unreachable();
    } catch (e) {
      expect((e as { code: string }).code).toBe("argument_error");
      expect((e as Error).message).toContain("unknown contract_status");
      expect((e as Error).message).toContain("valid:");
    }
  });
  it("空は何もしない(エラーなし・空)", () => {
    expect(expandContractStatus("")).toEqual({ delivery: [], order: [] });
  });
});

describe("filterProjectsByContractStatus", () => {
  it("active: 納品ステータス名のみで評価する", () => {
    const projects = [
      P(1, { delivery_status_name: "未着手" }),
      P(2, { delivery_status_name: "検収済" }),
      P(3, { order_status_name: "見積中(中)" }),
    ];
    expect(filterProjectsByContractStatus(projects, "active").map((p) => p.id)).toEqual([1]);
  });
  it("ended", () => {
    const projects = [
      P(1, { delivery_status_name: "未着手" }),
      P(2, { delivery_status_name: "検収済" }),
    ];
    expect(filterProjectsByContractStatus(projects, "ended").map((p) => p.id)).toEqual([2]);
  });
  it("prospect: 受注ステータス名のみで評価する", () => {
    const projects = [
      P(1, { order_status_name: "見積中(中)" }),
      P(2, { order_status_name: "受注確定" }),
    ];
    expect(filterProjectsByContractStatus(projects, "prospect").map((p) => p.id)).toEqual([1]);
  });
  it("all: 5 件中 4 件", () => {
    const projects = [
      P(1, { delivery_status_name: "未着手" }),
      P(2, { delivery_status_name: "着手中" }),
      P(3, { delivery_status_name: "検収済" }),
      P(4, { order_status_name: "見積中(高)" }),
      P(5, { order_status_name: "受注済" }),
    ];
    expect(filterProjectsByContractStatus(projects, "all")).toHaveLength(4);
  });
  it("空の入力は空、空の alias は何も絞らない", () => {
    expect(filterProjectsByContractStatus([], "active")).toEqual([]);
    expect(filterProjectsByContractStatus([P(1)], "")).toHaveLength(1);
  });
  it("見積中(中)×未着手 は active に含まれ、prospect には含まれない(既知の semantics)", () => {
    const p = P(1, { order_status_name: "見積中(中)", delivery_status_name: "未着手" });
    expect(filterProjectsByContractStatus([p], "active")).toHaveLength(1);
    expect(filterProjectsByContractStatus([p], "prospect")).toHaveLength(1);
  });
});

describe("filter(Go: filter_test.go)", () => {
  const item = (s: string) => ({ s });
  const getS = (x: { s: string }) => x.s;
  it("複数ステータスに一致するものだけ", () => {
    expect(filterByStatuses([item("a"), item("b"), item("c")], getS, ["a", "c"])).toEqual([
      item("a"),
      item("c"),
    ]);
  });
  it("statuses が空ならそのまま / 一致なしは空", () => {
    expect(filterByStatuses([item("a"), item("b")], getS, [])).toHaveLength(2);
    expect(filterByStatuses([item("a"), item("b")], getS, ["z"])).toEqual([]);
  });
  it("10 件・11 件でもフィルタ自体は拒否しない(上限は validate の仕事)", () => {
    expect(filterByStatuses([item("x")], getS, Array(10).fill("x"))).toHaveLength(1);
    expect(filterByStatuses([item("x")], getS, Array(11).fill("x"))).toHaveLength(1);
  });
  it("projects: 受注または納品のステータス名のどちらかが一致すれば残す(OR)", () => {
    const projects = [
      P(1, { order_status_name: "受注済", delivery_status_name: "未着手" }),
      P(2, { order_status_name: "見積中(高)", delivery_status_name: "着手中" }),
      P(3, { order_status_name: "失注", delivery_status_name: "" }),
    ];
    expect(filterProjectsByStatuses(projects, ["受注済", "着手中"]).map((p) => p.id)).toEqual([
      1, 2,
    ]);
    expect(filterProjectsByStatus(projects, "失注").map((p) => p.id)).toEqual([3]);
    expect(filterProjectsByStatus(projects, "")).toHaveLength(3);
    expect(filterProjectsByStatuses(projects, [])).toHaveLength(3);
  });
});

describe("url", () => {
  const base = "https://the-board.jp";
  it("project / document / client / vendor(payees)の編集 URL", () => {
    expect(projectUrl(base, 42)).toBe("https://the-board.jp/projects/42/edit");
    expect(documentUrl(base, 42)).toBe("https://the-board.jp/documents/42/edit");
    expect(clientUrl(base, 7)).toBe("https://the-board.jp/clients/7/edit");
    expect(vendorUrl(base, 9)).toBe("https://the-board.jp/payees/9/edit");
  });
  it("id=0 またはベース URL が空なら空文字", () => {
    expect(projectUrl(base, 0)).toBe("");
    expect(documentUrl("", 5)).toBe("");
    expect(clientUrl("", 5)).toBe("");
    expect(vendorUrl(base, 0)).toBe("");
  });
  it("UI のベースは API のオリジンの '://api.' を '://' にして導く", () => {
    expect(uiBaseUrlFromApiBase("https://api.the-board.jp")).toBe("https://the-board.jp");
    expect(uiBaseUrlFromApiBase("https://api.board.test/")).toBe("https://board.test");
    expect(uiBaseUrlFromApiBase("https://example.com")).toBe("https://example.com");
  });
});

describe("fanout の上限", () => {
  it("上限は 5。超えたら argument_error で絞り込みを求める", () => {
    expect(FANOUT_RESOLVE_CAP).toBe(5);
    const e = errFanoutTooMany("client_name", "株式会社", 12);
    expect(e.code).toBe("argument_error");
    expect(e.retryable).toBe(false);
    expect(e.message).toContain('client_name "株式会社" matched 12 entities (cap=5)');
    expect(e.message).toContain("narrow the query");
  });
});

describe("曖昧な名前の候補(Go: resolver_test.go)", () => {
  const list = (n: number) =>
    Array.from({ length: n }, (_, i) => ({ id: i + 1, name: `会社${i + 1}` }));
  it("複数ヒットは件数・--id の案内・候補を並べる", () => {
    const msg = formatCandidates("client", "会社", list(2));
    expect(msg).toContain('multiple clients match name "会社" (2 hits)');
    expect(msg).toContain("use id to disambiguate");
    expect(msg).toContain('- id=1 name="会社1"');
    expect(msg).toContain('- id=2 name="会社2"');
    expect(msg).not.toContain("more)");
  });
  it("候補は最大 5 件で、超過分は '... (N more)'", () => {
    expect(RESOLVER_CANDIDATES_MAX).toBe(5);
    const msg = formatCandidates("vendor", "x", list(8));
    expect(msg).toContain("multiple vendors match");
    expect(msg).toContain("(8 hits)");
    expect(msg).toContain("id=5");
    expect(msg).not.toContain("id=6");
    expect(msg).toContain("... (3 more)");
  });
  it("ちょうど 5 件なら省略表記なし", () => {
    expect(formatCandidates("client", "x", list(5))).not.toContain("more)");
  });
});

describe("クエリの検証(Go: types_test.go)", () => {
  const code = (fn: () => void) => {
    try {
      fn();
    } catch (e) {
      return (e as { code?: string }).code;
    }
    return undefined;
  };
  it("limit が負なら拒否、0 は無制限として許容", () => {
    expect(code(() => validateClientQuery({ id: 1, limit: -1 }))).toBe("argument_error");
    expect(code(() => validateClientQuery({ id: 1, limit: 0 }))).toBeUndefined();
  });
  it("空のクエリは 'at least one field required'", () => {
    expect(() => validateClientQuery({})).toThrow("at least one field required");
    expect(() => validateInvoiceQuery({})).toThrow("at least one field required");
    expect(() => validateDocumentQuery({})).toThrow("at least one field required");
    expect(() => validateProjectQuery({})).toThrow("at least one field required");
  });
  it("projects: status / statuses / contract_status は互いに排他", () => {
    const base = { name: "x" };
    for (const q of [
      { status: "a", statuses: ["b"] },
      { status: "a", contractStatus: "active" },
      { statuses: ["a"], contractStatus: "active" },
      { status: "a", statuses: ["b"], contractStatus: "active" },
    ]) {
      expect(() => validateProjectQuery({ ...base, ...q })).toThrow("mutually exclusive");
    }
  });
  it("projects: statuses は 10 件まで", () => {
    expect(() => validateProjectQuery({ name: "x", statuses: Array(10).fill("a") })).not.toThrow();
    expect(() => validateProjectQuery({ name: "x", statuses: Array(11).fill("a") })).toThrow(
      "at most 10 statuses",
    );
  });
  it("projects: status 系は id / client(名前でも ID でも) / name のどれかと併用が必要", () => {
    for (const q of [{ status: "受注" }, { statuses: ["a"] }, { contractStatus: "active" }]) {
      expect(() => validateProjectQuery(q)).toThrow("requires at least one of ID, Name, or Client");
    }
    expect(() => validateProjectQuery({ contractStatus: "active", name: "保守" })).not.toThrow();
    expect(() => validateProjectQuery({ contractStatus: " ACTIVE ", clientId: 3 })).not.toThrow();
    expect(() => validateProjectQuery({ contractStatus: "active", clientName: "A" })).not.toThrow();
    expect(() => validateProjectQuery({ status: "受注", id: 1 })).not.toThrow();
  });
  it("invoices: statuses 単独は拒否、status 単独は API に委譲できるので許可", () => {
    expect(() => validateInvoiceQuery({ status: "sent" })).not.toThrow();
    expect(() => validateInvoiceQuery({ statuses: ["a"] })).toThrow("requires one of ID or");
    expect(() => validateInvoiceQuery({ statuses: ["a"], partnerId: 1 })).not.toThrow();
  });
});
