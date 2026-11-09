import { describe, expect, it } from "vitest";
import { applyLimit, isCacheableFilter, isZeroFilter } from "../../src/repository/filter";
import { SPECS } from "../../src/repository/resources";
import { invoiceEntity, paymentEntity, purchaseOrderEntity, userEntity } from "../helpers/entities";

describe("isZeroFilter(Go の *FilterIsZero)", () => {
  it("空・未指定・0・空文字・空配列はゼロ値", () => {
    expect(isZeroFilter({})).toBe(true);
    expect(isZeroFilter({ page: 0, perPage: 0, nameCont: "", tags: [], clientIdEq: 0 })).toBe(true);
    expect(isZeroFilter({ nameCont: undefined, includeArchiveFlg: undefined })).toBe(true);
  });

  it("値があればゼロ値でない。真偽値は false でも『指定した』扱い(Go のポインタ)", () => {
    expect(isZeroFilter({ nameCont: "a" })).toBe(false);
    expect(isZeroFilter({ clientIdEq: 5 })).toBe(false);
    expect(isZeroFilter({ tags: ["x"] })).toBe(false);
    expect(isZeroFilter({ includeArchiveFlg: false })).toBe(false);
    expect(isZeroFilter({ includeArchiveFlg: true })).toBe(false);
  });
});

describe("isCacheableFilter(キャッシュ側で絞れる項目だけか)", () => {
  it("許可した項目以外(サーバー専用の項目)が 1 つでもあれば false", () => {
    const keys = ["nameCont"] as const;
    expect(isCacheableFilter({ nameCont: "a" }, keys)).toBe(true);
    expect(isCacheableFilter({ nameCont: "a", perPage: 10 }, keys)).toBe(false);
    expect(isCacheableFilter({ nameCont: "a", includeArchiveFlg: false }, keys)).toBe(false);
    // ゼロ値のサーバー専用項目は無視する。
    expect(isCacheableFilter({ nameCont: "a", perPage: 0, tags: [] }, keys)).toBe(true);
  });
});

describe("applyLimit", () => {
  it("limit > 0 で切り詰め、0 以下・未指定は全件", () => {
    expect(applyLimit([1, 2, 3], 2)).toEqual([1, 2]);
    expect(applyLimit([1, 2, 3], 0)).toEqual([1, 2, 3]);
    expect(applyLimit([1, 2, 3], undefined)).toEqual([1, 2, 3]);
    expect(applyLimit([1], 5)).toEqual([1]);
  });
});

describe("22 リソースのモード(Go の repository 層の実態)", () => {
  it("cache-first: clients / projects / invoices / purchase_orders / payments / users / vendors", () => {
    const names = Object.values(SPECS)
      .filter((s) => s.kind === "cache-first")
      .map((s) => s.resource)
      .sort();
    expect(names).toEqual([
      "clients",
      "invoices",
      "payments",
      "projects",
      "purchase_orders",
      "users",
      "vendors",
    ]);
  });

  it("nonzero-api: 残りの 11 の一覧リソース", () => {
    const names = Object.values(SPECS)
      .filter((s) => s.kind === "nonzero-api")
      .map((s) => s.resource)
      .sort();
    expect(names).toEqual(
      [
        "accounting_types",
        "client_branches",
        "contacts",
        "document_send_channels",
        "groups",
        "payment_terms",
        "project_costs",
        "project_types",
        "purchase_types",
        "vendor_branches",
        "vendor_contacts",
      ].sort(),
    );
  });

  it("document: estimates / orders / deliveries / receipts は get のみ", () => {
    const names = Object.values(SPECS)
      .filter((s) => s.kind === "document")
      .map((s) => s.resource)
      .sort();
    expect(names).toEqual(["deliveries", "estimates", "orders", "receipts"]);
    expect(Object.keys(SPECS)).toHaveLength(22);
  });
});

// 以下は Go の matchXxxFilter / cacheableXxxFilter を移したもの。
describe("clients のキャッシュ側フィルタ", () => {
  const s = SPECS.clients.cacheFirst;
  const c = (o: Record<string, unknown>) => o as never;

  it("cacheable は name / name_disp / 適格請求書番号だけ(顧客番号 custom_no_eq はサーバー専用。issue #28)", () => {
    for (const f of [{ nameCont: "a" }, { nameDispCont: "a" }, { invoiceSystemNumberEq: "T1" }]) {
      expect(isCacheableFilter(f, s.keys)).toBe(true);
    }
    for (const f of [
      // custom_no は response_group=small(既定)に含まれず、同期済みのキャッシュでは常に 0 件になる
      { customNoEq: "1" },
      { nameCont: "a", customNoEq: "1" },
      { nameCont: "a", page: 2 },
      { nameCont: "a", perPage: 10 },
      { updatedAtGteq: "2026-01-01 00:00:00" },
      { updatedAtLteq: "2026-01-01 00:00:00" },
      { includeArchiveFlg: false },
      { tags: ["x"] },
      { responseGroup: "large" },
    ]) {
      expect(isCacheableFilter(f, s.keys)).toBe(false);
    }
  });

  it("name は NFKC + 小文字 + trim の部分一致(name のみ。name_disp は見ない)", () => {
    const e = c({ id: 1, name: "弘前大学ＣＯＩ研究推進機構", name_disp: "弘大" });
    expect(s.match(e, c({ nameCont: "coi" }))).toBe(true);
    expect(s.match(e, c({ nameCont: " ＣＯＩ " }))).toBe(true);
    expect(s.match(e, c({ nameCont: "弘大" }))).toBe(false);
    expect(s.match(e, c({ nameDispCont: "弘大" }))).toBe(true);
    expect(s.match(e, c({ nameCont: "zzz" }))).toBe(false);
  });

  it("適格請求書番号は完全一致で、null は空文字扱い", () => {
    const e = c({ id: 1, name: "a", invoice_system_number: "T123" });
    expect(s.match(e, c({ invoiceSystemNumberEq: "T123" }))).toBe(true);
    expect(s.match(e, c({ invoiceSystemNumberEq: "T12" }))).toBe(false);
    expect(s.match(c({ id: 2, name: "b" }), c({ invoiceSystemNumberEq: "T123" }))).toBe(false);
  });

  it("複数の条件は AND", () => {
    const e = c({ id: 1, name: "ABC", name_disp: "x", invoice_system_number: "T9" });
    expect(s.match(e, c({ nameCont: "ab", invoiceSystemNumberEq: "T9" }))).toBe(true);
    expect(s.match(e, c({ nameCont: "ab", invoiceSystemNumberEq: "T8" }))).toBe(false);
  });
});

describe("projects のキャッシュ側フィルタ", () => {
  const s = SPECS.projects.cacheFirst;
  const c = (o: Record<string, unknown>) => o as never;
  const p = c({
    id: 1,
    name: "ProjectA",
    project_no: 42,
    management_no: "M-1",
    client: { id: 10, name: "ＡＢＣ商事" },
  });

  it("cacheable は name / client_id / client_name / project_no / management_no だけ", () => {
    for (const f of [
      { nameCont: "a" },
      { clientIdEq: 1 },
      { clientNameCont: "a" },
      { projectNoEq: "1" },
      { managementNoEq: "a" },
    ]) {
      expect(isCacheableFilter(f, s.keys)).toBe(true);
    }
    for (const f of [
      { createdAtGteq: "2026-01-01 00:00:00" },
      { includeLostFlg: false },
      { orderStatusIn: [1] },
      { deliveryStatusIn: [1] },
      { deliveryDateGteq: "2026-01-01" },
      { invoiceDateLteq: "2026-01-01" },
      { invoiceTimingKbnIn: [1] },
      { tags: ["t"] },
      { responseGroup: "estimate" },
      { updatedAtGteq: "2026-01-01 00:00:00" },
    ]) {
      expect(isCacheableFilter(f, s.keys)).toBe(false);
    }
  });

  it("name_cont / client_id_eq / client_name_cont / project_no_eq / management_no_eq", () => {
    expect(s.match(p, c({ nameCont: "projecta" }))).toBe(true);
    expect(s.match(p, c({ clientIdEq: 10 }))).toBe(true);
    expect(s.match(p, c({ clientIdEq: 11 }))).toBe(false);
    expect(s.match(p, c({ clientNameCont: "abc" }))).toBe(true);
    expect(s.match(p, c({ projectNoEq: "42" }))).toBe(true);
    expect(s.match(p, c({ projectNoEq: "4" }))).toBe(false);
    expect(s.match(p, c({ managementNoEq: "M-1" }))).toBe(true);
    expect(s.match(p, c({ managementNoEq: "m-1" }))).toBe(false); // 完全一致(fold しない)
  });

  it("client が null / project_no が null のときは該当する条件に合致しない", () => {
    const orphan = c({ id: 2, name: "x", project_no: null, management_no: null, client: null });
    expect(s.match(orphan, c({ clientIdEq: 10 }))).toBe(false);
    expect(s.match(orphan, c({ clientNameCont: "a" }))).toBe(false);
    expect(s.match(orphan, c({ projectNoEq: "1" }))).toBe(false);
    expect(s.match(orphan, c({ managementNoEq: "a" }))).toBe(false);
    expect(s.match(orphan, c({ nameCont: "x" }))).toBe(true);
  });
});

const partner = (id: number) => ({ id, name: "取引先", name_disp: "取引先", custom_no: "" });

describe("invoices / purchase_orders / payments / vendors / users のキャッシュ側フィルタ", () => {
  const c = (o: Record<string, unknown>) => o as never;

  it("invoices: 顧客(client?.id)と請求ステータス(invoice_status を含む)", () => {
    const s = SPECS.invoices.cacheFirst;
    const e = invoiceEntity({
      id: 1,
      client: partner(3),
      invoice_status: 2,
    });
    expect(s.match(e, c({ projectClientIdEq: 3 }))).toBe(true);
    expect(s.match(e, c({ projectClientIdEq: 9 }))).toBe(false);
    expect(s.match(e, c({ invoiceStatusIn: [2, 3] }))).toBe(true);
    expect(s.match(e, c({ invoiceStatusIn: [3, 9] }))).toBe(false);
    expect(s.match(e, c({ projectClientIdEq: 3, invoiceStatusIn: [2] }))).toBe(true);
    expect(s.match(e, c({ projectClientIdEq: 3, invoiceStatusIn: [9] }))).toBe(false);
    // 顧客が無い請求書は顧客 ID の絞り込みに一致しない
    expect(s.match(invoiceEntity({ client: null }), c({ projectClientIdEq: 3 }))).toBe(false);
    expect(isCacheableFilter({ responseGroup: "x" }, s.keys)).toBe(false);
    expect(isCacheableFilter({ projectClientIdEq: 1, invoiceStatusIn: [2] }, s.keys)).toBe(true);
  });

  it("invoices: 案件番号(数値の完全一致)・管理番号(完全一致)・案件名(fold の部分一致)。日付は cacheable でない", () => {
    const s = SPECS.invoices.cacheFirst;
    const e = invoiceEntity({ project_no: 1001, management_no: "M-001", name: "Web サイト制作" });
    expect(s.match(e, c({ projectProjectNoEq: 1001 }))).toBe(true);
    expect(s.match(e, c({ projectProjectNoEq: 1002 }))).toBe(false);
    expect(s.match(invoiceEntity({ project_no: null }), c({ projectProjectNoEq: 1001 }))).toBe(
      false,
    );
    expect(s.match(e, c({ projectManagementNoEq: "M-001" }))).toBe(true);
    expect(s.match(e, c({ projectManagementNoEq: "M-00" }))).toBe(false);
    expect(s.match(invoiceEntity({ management_no: null }), c({ projectManagementNoEq: "M" }))).toBe(
      false,
    );
    expect(s.match(e, c({ projectNameCont: "web" }))).toBe(true);
    expect(s.match(e, c({ projectNameCont: "別案件" }))).toBe(false);
    for (const k of ["projectProjectNoEq", "projectManagementNoEq", "projectNameCont"]) {
      expect(isCacheableFilter({ [k]: k === "projectProjectNoEq" ? 1 : "x" }, s.keys), k).toBe(
        true,
      );
    }
    for (const k of [
      "invoiceDateGteq",
      "invoiceDateLteq",
      "invoicePaymentLimitDateGteq",
      "invoicePaymentLimitDateLteq",
    ]) {
      expect(isCacheableFilter({ [k]: "2026-01-01" }, s.keys), k).toBe(false);
    }
  });

  it("invoices: 旧キー(client_id / status)を持つ行は一致しない(実在しない項目を見ない)", () => {
    const s = SPECS.invoices.cacheFirst;
    const legacy = c({ id: 1, client_id: 3, project_id: 4, status: "paid" });
    expect(s.match(legacy, c({ projectClientIdEq: 3 }))).toBe(false);
    expect(s.match(legacy, c({ invoiceStatusIn: [2] }))).toBe(false);
  });

  it("purchase_orders: 支払先(payee?.id)・発注ステータス・進捗状況。案件はキャッシュで絞らない", () => {
    const s = SPECS.purchase_orders.cacheFirst;
    const e = purchaseOrderEntity({
      payee: partner(3),
      expenditure_status: 5,
      delivery_status: 2,
    });
    expect(s.match(e, c({ payeeIdEq: 3 }))).toBe(true);
    expect(s.match(e, c({ payeeIdEq: 4 }))).toBe(false);
    expect(s.match(e, c({ expenditureStatusIn: [4, 5] }))).toBe(true);
    expect(s.match(e, c({ expenditureStatusIn: [4] }))).toBe(false);
    expect(s.match(e, c({ deliveryStatusIn: [2] }))).toBe(true);
    expect(s.match(e, c({ deliveryStatusIn: [3, 4] }))).toBe(false);
    expect(s.match(purchaseOrderEntity({ payee: null }), c({ payeeIdEq: 3 }))).toBe(false);
    // project_id_eq はサーバー専用: cacheable の対象ではない(指定されたら API に回る)
    expect(s.keys).not.toContain("projectIdEq");
    expect(isCacheableFilter({ projectIdEq: 100 }, s.keys)).toBe(false);
    expect(isCacheableFilter({ payeeIdEq: 1, expenditureStatusIn: [5] }, s.keys)).toBe(true);
  });

  it("purchase_orders: 旧キー(vendor_id / status)を持つ行は一致しない", () => {
    const s = SPECS.purchase_orders.cacheFirst;
    const legacy = c({ id: 1, vendor_id: 3, project_id: 4, status: "sent" });
    expect(s.match(legacy, c({ payeeIdEq: 3 }))).toBe(false);
    expect(s.match(legacy, c({ expenditureStatusIn: [5] }))).toBe(false);
  });

  it("payments: 支払先(payee?.id)・支払ステータス・発注 No(expenditure_no)", () => {
    const s = SPECS.payments.cacheFirst;
    const e = paymentEntity({
      payee: partner(3),
      payment_status: 3,
      expenditure_no: 2001,
    });
    expect(s.match(e, c({ expenditurePayeeIdEq: 3 }))).toBe(true);
    expect(s.match(e, c({ expenditurePayeeIdEq: 4 }))).toBe(false);
    expect(s.match(e, c({ paymentStatusIn: [1, 3] }))).toBe(true);
    expect(s.match(e, c({ paymentStatusIn: [1, 2] }))).toBe(false);
    expect(s.match(e, c({ expenditureExpenditureNoEq: 2001 }))).toBe(true);
    expect(s.match(e, c({ expenditureExpenditureNoEq: 2002 }))).toBe(false);
    expect(s.match(paymentEntity({ payee: null }), c({ expenditurePayeeIdEq: 3 }))).toBe(false);
  });

  it("payments: 旧キー(vendor_id / purchase_order_id / status)を持つ行は一致しない", () => {
    const s = SPECS.payments.cacheFirst;
    const legacy = c({ id: 1, vendor_id: 3, purchase_order_id: 8, status: "paid" });
    expect(s.match(legacy, c({ expenditurePayeeIdEq: 3 }))).toBe(false);
    expect(s.match(legacy, c({ expenditureExpenditureNoEq: 8 }))).toBe(false);
    expect(s.match(legacy, c({ paymentStatusIn: [3] }))).toBe(false);
  });

  it("vendors: name だけ(fold)", () => {
    const s = SPECS.vendors.cacheFirst;
    expect(s.match(c({ id: 1, name: "Ｔｅｓｔ商会" }), c({ nameCont: "test" }))).toBe(true);
    expect(isCacheableFilter({ nameCont: "a" }, s.keys)).toBe(true);
    expect(isCacheableFilter({ nameCont: "a", perPage: 5 }, s.keys)).toBe(false);
  });

  it("users: 名前は姓・名・姓名(連結 / 空白つき)の fold 部分一致、メールは fold 部分一致", () => {
    const s = SPECS.users.cacheFirst;
    const e = userEntity({
      id: 1,
      last_name: "山田",
      first_name: "太郎",
      email: "Taro@Example.com",
    });
    expect(s.match(e, c({ nameCont: "山田" }))).toBe(true); // 姓
    expect(s.match(e, c({ nameCont: "太郎" }))).toBe(true); // 名
    expect(s.match(e, c({ nameCont: "山田太郎" }))).toBe(true); // 連結
    expect(s.match(e, c({ nameCont: "山田 太郎" }))).toBe(true); // 空白つき
    expect(s.match(e, c({ nameCont: "田太" }))).toBe(true); // 連結の部分一致
    expect(s.match(e, c({ nameCont: "次郎" }))).toBe(false);
    expect(s.match(e, c({ emailCont: "taro@example" }))).toBe(true);
    expect(s.match(e, c({ emailCont: "jiro" }))).toBe(false);
    expect(s.match(e, c({ nameCont: "太郎", emailCont: "jiro" }))).toBe(false);
    const wide = userEntity({ last_name: "Ｓｍｉｔｈ", first_name: "ジョン" });
    expect(s.match(wide, c({ nameCont: "smith ジョン" }))).toBe(true);
  });

  it("users: 旧キー(name)を持つ行は名前の絞り込みに一致しない(実在しない項目を見ない)", () => {
    const s = SPECS.users.cacheFirst;
    expect(
      s.match(c({ id: 1, name: "山田 太郎", email: "a@example.com" }), c({ nameCont: "太郎" })),
    ).toBe(false);
  });

  it("users: name_cont / email_cont は『キャッシュ専用』のキー(API には送れない)", () => {
    const s = SPECS.users.cacheFirst;
    expect([...(s.cacheOnlyKeys ?? [])].sort()).toEqual(["emailCont", "nameCont"]);
    expect([...s.keys].sort()).toEqual(["emailCont", "nameCont"]);
    // 他のリソースにはキャッシュ専用のキーが無い
    for (const spec of Object.values(SPECS)) {
      if (spec.kind === "cache-first" && spec.resource !== "users") {
        expect((spec.cacheFirst as { cacheOnlyKeys?: unknown }).cacheOnlyKeys).toBeUndefined();
      }
    }
  });
});
