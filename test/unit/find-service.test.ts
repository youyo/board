/**
 * find 層の 11 の検索。Go 版 `internal/service/find/find_*_test.go` のケースを転記して固定する。
 * repository は偽物(test/helpers/find-fakes.ts)。想定外の呼び出しは例外になる。
 */
import { describe, expect, it } from "vitest";
import type { InvoiceEntity, PaymentEntity, PurchaseOrderEntity } from "../../src/board/resources";
import {
  findClients,
  findDeliveries,
  findEstimates,
  findInvoices,
  findOrders,
  findPayments,
  findProjects,
  findPurchaseOrders,
  findReceipts,
  findUsers,
  findVendors,
} from "../../src/find/service";
import { MAX_WARNINGS } from "../../src/find/session";
import { invoiceEntity, paymentEntity, purchaseOrderEntity, userEntity } from "../helpers/entities";
import { apiError, entities, fakeFind, notFound } from "../helpers/find-fakes";

const rejects = (p: Promise<unknown>) =>
  p.then(
    () => undefined,
    (e) => e as Error & { code?: string },
  );

describe("find_clients(Go: find_client_test.go)", () => {
  const base = {
    clients: { get: (id: number) => ({ id, name: `顧客${id}` }) },
    client_branches: { list: (f: { clientIdEq: number }) => [{ id: 100 + f.clientIdEq }] },
    contacts: { list: (f: { clientIdEq: number }) => [{ id: 200 + f.clientIdEq }] },
  };

  it("id: 顧客に支店・担当者・url が付く。支店・担当者は client_id_eq で引く", async () => {
    const f = fakeFind(base);
    const r = await findClients(f.session(), { id: 5 });
    expect(r).toEqual([
      {
        Client: { id: 5, name: "顧客5" },
        Branches: [{ id: 105 }],
        Contacts: [{ id: 205 }],
        url: "https://the-board.jp/clients/5/edit",
      },
    ]);
    expect(f.calls("client_branches.list")[0]).toContain('"clientIdEq":5');
  });

  it("name: nameCont で検索する", async () => {
    const f = fakeFind({ ...base, clients: { list: () => entities(2, "顧客") } });
    const r = await findClients(f.session(), { name: "顧客" });
    expect(r.map((x) => x.Client.id)).toEqual([1, 2]);
    expect(f.calls("clients.list")[0]).toContain('{"nameCont":"顧客"}');
  });

  it("id が name に優先する(search を呼ばない)", async () => {
    const f = fakeFind(base);
    await findClients(f.session(), { id: 5, name: "x" });
    expect(f.calls("clients.list")).toHaveLength(0);
  });

  it("limit=2: 3 件のうち 2 件だけ補助情報を取る。切ったことを total_before_limit に残す", async () => {
    const f = fakeFind({ ...base, clients: { list: () => entities(3, "顧客") } });
    const s = f.session();
    const r = await findClients(s, { name: "顧客", limit: 2 });
    expect(r).toHaveLength(2);
    expect(f.calls("client_branches.list")).toHaveLength(2);
    expect(s.totalBeforeLimit).toBe(3);
  });

  it("limit=0 は無制限で、切っていなければ total_before_limit は付かない", async () => {
    const f = fakeFind({ ...base, clients: { list: () => entities(3, "顧客") } });
    const s = f.session();
    expect(await findClients(s, { name: "顧客", limit: 0 })).toHaveLength(3);
    expect(s.totalBeforeLimit).toBeUndefined();
  });

  it("空のクエリ・負の limit は argument_error", async () => {
    const f = fakeFind(base);
    expect((await rejects(findClients(f.session(), {})))?.message).toContain("at least one field");
    expect((await rejects(findClients(f.session(), { id: 1, limit: -1 })))?.code).toBe(
      "argument_error",
    );
    expect(f.log).toEqual([]);
  });

  it("主体の取得の失敗(get / search)は伝わる", async () => {
    const e1 = await rejects(
      findClients(fakeFind({ clients: { get: () => Promise.reject(notFound()) } }).session(), {
        id: 1,
      }),
    );
    expect(e1?.code).toBe("not_found");
    const e2 = await rejects(
      findClients(fakeFind({ clients: { list: () => Promise.reject(apiError()) } }).session(), {
        name: "x",
      }),
    );
    expect(e2?.code).toBe("api_error");
  });

  it("支店の取得に失敗しても致命的でない: Branches は null、Contacts は返り、警告にエラーコードだけを載せる", async () => {
    const f = fakeFind({
      ...base,
      client_branches: { list: () => Promise.reject(apiError(503)) },
    });
    const s = f.session();
    const r = await findClients(s, { id: 5 });
    expect(r[0]?.Branches).toBeNull();
    expect(r[0]?.Contacts).toEqual([{ id: 205 }]);
    expect(s.warnings).toEqual([
      { code: "enrichment_failed", resource: "client_branches", id: 5, error_code: "api_error" },
    ]);
  });

  it("支店・担当者の両方が失敗しても顧客は返る", async () => {
    const f = fakeFind({
      clients: base.clients,
      client_branches: { list: () => Promise.reject(apiError()) },
      contacts: { list: () => Promise.reject(apiError()) },
    });
    const r = await findClients(f.session(), { id: 5 });
    expect(r[0]).toMatchObject({ Client: { id: 5 }, Branches: null, Contacts: null });
  });
});

describe("find_vendors", () => {
  it("仕入先の支店・担当者は payee_id_eq で引く", async () => {
    const f = fakeFind({
      vendors: { get: (id: number) => ({ id, name: "V" }) },
      vendor_branches: { list: () => [] },
      vendor_contacts: { list: () => [{ id: 1 }] },
    });
    const r = await findVendors(f.session(), { id: 9 });
    expect(r[0]).toEqual({
      Vendor: { id: 9, name: "V" },
      Branches: [],
      Contacts: [{ id: 1 }],
      url: "https://the-board.jp/payees/9/edit",
    });
    expect(f.calls("vendor_branches.list")[0]).toContain('"payeeIdEq":9');
  });
});

describe("find_projects(Go: find_project_test.go)", () => {
  const project = (id: number, extra: object = {}) => ({
    id,
    name: `案件${id}`,
    client: { id: 7 },
    order_status_name: "受注済",
    delivery_status_name: "未着手",
    ...extra,
  });
  const base = {
    clients: { get: (id: number) => ({ id, name: `顧客${id}` }) },
  };

  it("id: 案件に顧客が付き、url は projects/{id}/edit", async () => {
    const f = fakeFind({ ...base, projects: { get: (id: number) => project(id) } });
    const r = await findProjects(f.session(), { id: 42 });
    expect(r).toEqual([
      {
        Project: project(42),
        Client: { id: 7, name: "顧客7" },
        url: "https://the-board.jp/projects/42/edit",
      },
    ]);
  });

  it("案件に顧客がなければ顧客を取りに行かない", async () => {
    const f = fakeFind({ projects: { get: (id: number) => project(id, { client: null }) } });
    const r = await findProjects(f.session(), { id: 1 });
    expect(r[0]?.Client).toBeNull();
    expect(f.calls("clients.")).toHaveLength(0);
  });

  it("client_id → clientIdEq、name → nameCont(AND)", async () => {
    const f = fakeFind({ ...base, projects: { list: () => [project(1)] } });
    await findProjects(f.session(), { clientId: 7, name: "保守" });
    expect(f.calls("projects.list")[0]).toContain('{"clientIdEq":7,"nameCont":"保守"}');
  });

  it("id が他の絞り込みに優先する", async () => {
    const f = fakeFind({ ...base, projects: { get: (id: number) => project(id) } });
    await findProjects(f.session(), { id: 1, name: "x", clientId: 3 });
    expect(f.calls("projects.list")).toHaveLength(0);
  });

  it("limit=2: 3 件のうち 2 件だけ顧客を取る", async () => {
    const f = fakeFind({
      ...base,
      projects: {
        list: () => [
          project(1, { client: { id: 1 } }),
          project(2, { client: { id: 2 } }),
          project(3, { client: { id: 3 } }),
        ],
      },
    });
    const s = f.session();
    expect(await findProjects(s, { name: "案", limit: 2 })).toHaveLength(2);
    expect(f.calls("clients.get")).toHaveLength(2);
    expect(s.totalBeforeLimit).toBe(3);
  });

  it("同じ顧客は 1 回の呼び出しの間 1 度だけ取る", async () => {
    const f = fakeFind({
      ...base,
      projects: { list: () => [project(1), project(2), project(3)] },
    });
    await findProjects(f.session(), { name: "案" });
    expect(f.calls("clients.get")).toHaveLength(1);
  });

  describe("status / statuses(後段の絞り込み)", () => {
    const list = [
      project(1, { order_status_name: "受注済", delivery_status_name: "未着手" }),
      project(2, { order_status_name: "見積中(高)", delivery_status_name: "着手中" }),
      project(3, { order_status_name: "失注", delivery_status_name: "検収済" }),
    ];
    const run = (q: object) =>
      findProjects(fakeFind({ ...base, projects: { list: () => list } }).session(), q).then((r) =>
        r.map((x) => x.Project.id),
      );
    it("受注ステータス名・納品ステータス名のどちらでも(OR)", async () => {
      expect(await run({ name: "x", status: "受注済" })).toEqual([1]);
      expect(await run({ name: "x", status: "着手中" })).toEqual([2]);
      expect(await run({ name: "x", statuses: ["受注済", "検収済"] })).toEqual([1, 3]);
    });
    it("contract_status: active / ended / prospect / all", async () => {
      expect(await run({ name: "x", contractStatus: "active" })).toEqual([1, 2]);
      expect(await run({ name: "x", contractStatus: "ended" })).toEqual([3]);
      expect(await run({ name: "x", contractStatus: "prospect" })).toEqual([2]);
      expect(await run({ name: "x", contractStatus: "all" })).toEqual([1, 2, 3]);
    });
    it("contract_status は client_id と組み合わせられる", async () => {
      expect(await run({ clientId: 7, contractStatus: "ended" })).toEqual([3]);
    });
    it("id 指定のときは status も contract_status も掛けない", async () => {
      const f = fakeFind({ ...base, projects: { get: (id: number) => project(id) } });
      expect(await findProjects(f.session(), { id: 1, status: "存在しない" })).toHaveLength(1);
      expect(await findProjects(f.session(), { id: 1, contractStatus: "ended" })).toHaveLength(1);
    });
    it("未知の contract_status は argument_error", async () => {
      const e = await rejects(run({ name: "x", contractStatus: "bogus" }));
      expect(e?.code).toBe("argument_error");
      expect(e?.message).toContain("unknown contract_status");
    });
    it("status だけ・排他違反・11 件は検索の前に拒否する", async () => {
      const f = fakeFind({ projects: { list: () => list } });
      for (const q of [
        { status: "受注済" },
        { name: "x", status: "a", statuses: ["b"] },
        { name: "x", statuses: Array(11).fill("a") },
        {},
      ]) {
        expect((await rejects(findProjects(f.session(), q)))?.code, JSON.stringify(q)).toBe(
          "argument_error",
        );
      }
      expect(f.log).toEqual([]);
    });
  });

  it("主体の失敗は伝わる / 顧客の取得の失敗は致命的でない(Client は null + 警告)", async () => {
    expect(
      (
        await rejects(
          findProjects(
            fakeFind({ projects: { get: () => Promise.reject(notFound()) } }).session(),
            { id: 1 },
          ),
        )
      )?.code,
    ).toBe("not_found");
    expect(
      (
        await rejects(
          findProjects(
            fakeFind({ projects: { list: () => Promise.reject(apiError()) } }).session(),
            { name: "x" },
          ),
        )
      )?.code,
    ).toBe("api_error");

    const f = fakeFind({
      projects: { get: (id: number) => project(id) },
      clients: { get: () => Promise.reject(notFound()) },
    });
    const s = f.session();
    const r = await findProjects(s, { id: 1 });
    expect(r[0]?.Client).toBeNull();
    expect(s.warnings).toEqual([
      { code: "enrichment_failed", resource: "clients", id: 7, error_code: "not_found" },
    ]);
  });

  describe("client_name の解決(Go: resolver_test.go)", () => {
    it("1 件なら ID に解決して clientIdEq で絞る。解決の読み取りに refresh は付かない", async () => {
      const f = fakeFind(
        {
          clients: { list: () => [{ id: 7, name: "株式会社A" }], get: (id: number) => ({ id }) },
          projects: { list: () => [project(1)] },
        },
        { refresh: "delta" },
      );
      await findProjects(f.session(), { clientName: "A", name: "保守" });
      expect(f.calls("clients.list")[0]).toBe('clients.list {"nameCont":"A"} {}');
      expect(f.calls("projects.list")[0]).toContain('"clientIdEq":7');
      // projects の検索には refresh が付く。
      expect(f.calls("projects.list")[0]).toContain('{"refresh":"delta"}');
    });

    it("複数ヒットは argument_error。候補は最大 5 件で、案件の検索には進まない", async () => {
      const f = fakeFind({
        clients: { list: () => entities(8, "会社") },
        projects: { list: () => [] },
      });
      const e = await rejects(findProjects(f.session(), { clientName: "会社" }));
      expect(e?.code).toBe("argument_error");
      expect(e?.message).toContain('multiple clients match name "会社" (8 hits)');
      expect(e?.message).toContain("id=5");
      expect(e?.message).not.toContain("id=6");
      expect(e?.message).toContain("... (3 more)");
      expect(f.calls("projects.")).toHaveLength(0);
    });

    it("0 件は not_found、検索の失敗は伝わる", async () => {
      const none = await rejects(
        findProjects(fakeFind({ clients: { list: () => [] } }).session(), { clientName: "x" }),
      );
      expect(none?.code).toBe("not_found");
      expect(none?.message).toContain('no client matches name "x"');
      const err = await rejects(
        findProjects(fakeFind({ clients: { list: () => Promise.reject(apiError()) } }).session(), {
          clientName: "x",
        }),
      );
      expect(err?.code).toBe("api_error");
    });
  });
});

describe("find_users(issue #27: 姓・名で検索する)", () => {
  it("name は repository に nameCont で渡す。結果の User は姓名の分かれた実データの形", async () => {
    const f = fakeFind({ users: { list: () => [userEntity({ id: 3 })] } });
    const r = await findUsers(f.session(), { name: "山田 太郎" });
    expect(f.calls("users.list")[0]).toContain('{"nameCont":"山田 太郎"}');
    expect(r).toEqual([{ User: userEntity({ id: 3 }) }]);
  });
});

describe("find_invoices(Go: find_invoice_test.go + issue #27)", () => {
  const inv = (id: number, extra: Partial<InvoiceEntity> = {}) => invoiceEntity({ id, ...extra });
  const client = (id: number) => ({ id, name: "顧客", name_disp: "顧客", custom_no: "" });
  const base = {
    clients: { get: (id: number) => ({ id, name: "顧客" }) },
    projects: { get: (id: number) => ({ id, name: "案件" }) },
  };

  it("id: 顧客(client.id で引く)と案件が付き、url は documents/{project_id}/edit", async () => {
    const f = fakeFind({
      ...base,
      invoices: { get: (id: number) => inv(id, { client: client(7), project_id: 42 }) },
    });
    const r = await findInvoices(f.session(), { id: 9 });
    expect(r[0]).toEqual({
      Invoice: inv(9, { client: client(7), project_id: 42 }),
      Project: { id: 42, name: "案件" },
      Client: { id: 7, name: "顧客" },
      url: "https://the-board.jp/documents/42/edit",
    });
    expect(f.calls("clients.get")).toEqual(["clients.get 7 {}"]);
  });

  it("client が null の請求書は顧客を取りに行かない(Client は null、警告なし)", async () => {
    const f = fakeFind({
      projects: base.projects,
      invoices: { get: (id: number) => inv(id, { client: null }) },
    });
    const s = f.session();
    const r = await findInvoices(s, { id: 9 });
    expect(r[0]?.Client).toBeNull();
    expect(f.calls("clients.")).toHaveLength(0);
    expect(s.warnings).toEqual([]);
  });

  it("案件の取得に失敗しても致命的でない", async () => {
    const f = fakeFind({
      clients: base.clients,
      projects: { get: () => Promise.reject(apiError()) },
      invoices: { get: (id: number) => inv(id) },
    });
    const s = f.session();
    const r = await findInvoices(s, { id: 9 });
    expect(r[0]?.Project).toBeNull();
    expect(r[0]?.Client).not.toBeNull();
    expect(s.warnings[0]).toMatchObject({ code: "enrichment_failed", resource: "projects" });
  });

  it("client_id → projectClientIdEq、status 名はコードに変えて invoiceStatusIn に渡す", async () => {
    const f = fakeFind({ ...base, invoices: { list: () => [inv(1)] } });
    await findInvoices(f.session(), { partnerId: 7 });
    await findInvoices(f.session(), { partnerId: 7, status: "請求済" });
    await findInvoices(f.session(), { status: "入金済" });
    expect(f.calls("invoices.list").map((l) => l.split(" ")[1])).toEqual([
      '{"projectClientIdEq":7}',
      '{"projectClientIdEq":7,"invoiceStatusIn":[2]}',
      '{"invoiceStatusIn":[3]}',
    ]);
  });

  it("statuses(複数)は名称の配列をコードの配列にして invoiceStatusIn に渡す(API でもキャッシュでも効く)", async () => {
    const f = fakeFind({ ...base, invoices: { list: () => [inv(1)] } });
    await findInvoices(f.session(), { partnerId: 7, statuses: ["請求済", "入金済"] });
    expect(f.calls("invoices.list")[0]).toContain(
      '{"projectClientIdEq":7,"invoiceStatusIn":[2,3]}',
    );
  });

  it("知らないステータス名は argument_error(受け付ける名称を message に載せ、通信しない)", async () => {
    const f = fakeFind({ ...base, invoices: { list: () => [inv(1)] } });
    for (const q of [{ status: "sent" }, { partnerId: 7, statuses: ["請求済", "paid"] }]) {
      const e = await rejects(findInvoices(f.session(), q));
      expect(e?.code).toBe("argument_error");
      for (const name of ["未請求", "請求OK", "請求済", "一部入金済", "入金済", "回収不能"]) {
        expect(e?.message).toContain(name);
      }
    }
    expect(f.log).toEqual([]);
  });

  it("id 指定ではステータスを掛けない(名称も検査しない)", async () => {
    const f = fakeFind({
      ...base,
      invoices: { get: (id: number) => inv(id, { invoice_status: 9 }) },
    });
    expect(await findInvoices(f.session(), { id: 1, statuses: ["請求済"] })).toHaveLength(1);
    expect(f.calls("invoices.list")).toHaveLength(0);
  });

  it("statuses 単独・空のクエリは拒否 / id が優先 / limit", async () => {
    const f = fakeFind({
      ...base,
      invoices: { list: () => [inv(1), inv(2), inv(3)], get: (id: number) => inv(id) },
    });
    expect((await rejects(findInvoices(f.session(), { statuses: ["請求済"] })))?.code).toBe(
      "argument_error",
    );
    expect((await rejects(findInvoices(f.session(), {})))?.code).toBe("argument_error");
    await findInvoices(f.session(), { id: 1, partnerId: 7 });
    expect(f.calls("invoices.list")).toHaveLength(0);
    const s = f.session();
    expect(await findInvoices(s, { partnerId: 7, limit: 2 })).toHaveLength(2);
    expect(s.totalBeforeLimit).toBe(3);
  });

  it("client_name は解決してから projectClientIdEq にする", async () => {
    const f = fakeFind({
      clients: { list: () => [{ id: 7, name: "A" }], get: (id: number) => ({ id }) },
      projects: base.projects,
      invoices: { list: () => [inv(1)] },
    });
    await findInvoices(f.session(), { partnerName: "A" });
    expect(f.calls("invoices.list")[0]).toContain('{"projectClientIdEq":7}');
  });

  it("get の失敗は伝わる", async () => {
    const f = fakeFind({ invoices: { get: () => Promise.reject(notFound()) } });
    expect((await rejects(findInvoices(f.session(), { id: 1 })))?.code).toBe("not_found");
  });

  describe("案件で絞る(issue #31)", () => {
    const proj = (id: number, project_no: number | null, name = `案件${id}`) => ({
      id,
      name,
      project_no,
    });

    it("project_id: 案件を get して project_no を projectProjectNoEq に渡し、project_id が違う請求書は除く", async () => {
      const f = fakeFind({
        ...base,
        projects: { get: (id: number) => proj(id, 1001) },
        invoices: {
          list: () => [inv(1, { project_id: 42 }), inv(2, { project_id: 43 })],
        },
      });
      const r = await findInvoices(f.session(), { projectId: 42 });
      expect(r.map((x) => x.Invoice.id)).toEqual([1]);
      expect(f.calls("projects.get")[0]).toContain("projects.get 42");
      expect(f.calls("invoices.list")[0]).toContain('{"projectProjectNoEq":1001}');
    });

    it("project_id: project_no が null の案件は projectProjectNoEq を掛けず、後段の project_id だけで絞る", async () => {
      const f = fakeFind({
        ...base,
        projects: { get: (id: number) => proj(id, null) },
        invoices: {
          list: () => [inv(1, { project_id: 42 }), inv(2, { project_id: 43 })],
        },
      });
      const r = await findInvoices(f.session(), { projectId: 42 });
      expect(r.map((x) => x.Invoice.id)).toEqual([1]);
      expect(f.calls("invoices.list")[0]).toContain("invoices.list {} ");
    });

    it("project_name: 1 件に解決して案件番号で絞る(client_name と併用すると clientIdEq も掛け、請求書も顧客で絞る)", async () => {
      const f = fakeFind({
        clients: { list: () => [{ id: 7, name: "A" }], get: (id: number) => ({ id }) },
        projects: {
          list: () => [proj(42, 1001, "Web サイト")],
          get: (id: number) => proj(id, 1001),
        },
        invoices: {
          list: () => [inv(1, { project_id: 42 }), inv(2, { project_id: 99 })],
        },
      });
      const r = await findInvoices(f.session(), { projectName: "Web", partnerName: "A" });
      expect(r.map((x) => x.Invoice.id)).toEqual([1]);
      expect(f.calls("projects.list")[0]).toContain('{"nameCont":"Web","clientIdEq":7}');
      expect(f.calls("invoices.list")[0]).toContain(
        '{"projectClientIdEq":7,"projectProjectNoEq":1001}',
      );
    });

    it("project_name: 0 件は not_found、複数は候補つき argument_error(請求書は引かない)", async () => {
      const none = fakeFind({ ...base, projects: { list: () => [] } });
      expect((await rejects(findInvoices(none.session(), { projectName: "x" })))?.code).toBe(
        "not_found",
      );
      expect(none.calls("invoices.list")).toHaveLength(0);

      const many = fakeFind({
        ...base,
        projects: { list: () => [proj(1, 1, "案件A"), proj(2, 2, "案件B")] },
      });
      const e = await rejects(findInvoices(many.session(), { projectName: "案件" }));
      expect(e?.code).toBe("argument_error");
      expect(e?.message).toContain("multiple projects match");
      expect(e?.message).toContain("id=1");
      expect(e?.message).toContain("id=2");
      expect(many.calls("invoices.list")).toHaveLength(0);
    });

    it("project_name: project_no が null の案件は後段の project_id だけで絞る", async () => {
      const f = fakeFind({
        ...base,
        projects: { list: () => [proj(42, null)], get: (id: number) => proj(id, null) },
        invoices: { list: () => [inv(1, { project_id: 42 }), inv(2, { project_id: 43 })] },
      });
      const r = await findInvoices(f.session(), { projectName: "案件" });
      expect(r.map((x) => x.Invoice.id)).toEqual([1]);
      expect(f.calls("invoices.list")[0]).toContain("invoices.list {} ");
    });

    it("project_id と project_name は排他(argument_error。通信しない)", async () => {
      const f = fakeFind(base);
      const e = await rejects(findInvoices(f.session(), { projectId: 1, projectName: "x" }));
      expect(e?.code).toBe("argument_error");
      expect(e?.message).toContain("mutually exclusive");
      expect(f.log).toEqual([]);
    });

    it("id 指定のときは project_id / project_name を無視する", async () => {
      const f = fakeFind({
        ...base,
        invoices: { get: (id: number) => inv(id, { project_id: 42 }) },
      });
      const r = await findInvoices(f.session(), { id: 9, projectId: 1 });
      expect(r.map((x) => x.Invoice.id)).toEqual([9]);
      expect(f.calls("projects.list")).toHaveLength(0);
      expect(f.calls("invoices.list")).toHaveLength(0);
    });
  });
});

describe("find_purchase_orders(Go: find_purchase_order_test.go + issue #27)", () => {
  const partner = (id: number) => ({ id, name: "仕入先", name_disp: "仕入先", custom_no: "" });
  const proj = (id: number) => ({ id, name: "案件", project_no: 1, management_no: null });
  const po = (id: number, extra: Partial<PurchaseOrderEntity> = {}) =>
    purchaseOrderEntity({ id, payee: partner(3), project: proj(42), ...extra });
  const base = {
    vendors: { get: (id: number) => ({ id, name: "仕入先" }) },
    projects: { get: (id: number) => ({ id, name: "案件" }) },
  };

  it("id: 仕入先(payee.id)と案件(project.id)が付く。url は project.id から", async () => {
    const f = fakeFind({ ...base, purchase_orders: { get: (id: number) => po(id) } });
    const r = await findPurchaseOrders(f.session(), { id: 1 });
    expect(r[0]).toMatchObject({
      PurchaseOrder: { id: 1 },
      Vendor: { id: 3 },
      Project: { id: 42 },
      url: "https://the-board.jp/documents/42/edit",
    });
    expect(f.calls("vendors.get")).toEqual(["vendors.get 3 {}"]);
    expect(f.calls("projects.get")).toEqual(["projects.get 42 {}"]);
  });

  it("project も payee も無い発注書は何も取りに行かない(url なし)", async () => {
    const f = fakeFind({
      purchase_orders: { get: (id: number) => po(id, { payee: null, project: undefined }) },
    });
    const r = await findPurchaseOrders(f.session(), { id: 1 });
    expect(r[0]).toMatchObject({ Vendor: null, Project: null });
    expect("url" in (r[0] ?? {})).toBe(false);
    expect(f.calls("vendors.")).toHaveLength(0);
    expect(f.calls("projects.")).toHaveLength(0);
  });

  it("vendor_name は仕入先の解決 → payeeIdEq。status 名は発注ステータス / 進捗状況のコードにする", async () => {
    const f = fakeFind({
      ...base,
      vendors: { ...base.vendors, list: () => [{ id: 3, name: "V" }] },
      purchase_orders: { list: () => [po(1)] },
    });
    await findPurchaseOrders(f.session(), { partnerName: "V", status: "発注済" });
    await findPurchaseOrders(f.session(), { partnerId: 3, status: "着手中" });
    await findPurchaseOrders(f.session(), { partnerId: 3, statuses: ["発注済", "発注確定"] });
    expect(f.calls("purchase_orders.list").map((l) => l.split(" ")[1])).toEqual([
      '{"payeeIdEq":3,"expenditureStatusIn":[5]}',
      '{"payeeIdEq":3,"deliveryStatusIn":[2]}',
      '{"payeeIdEq":3,"expenditureStatusIn":[5,4]}',
    ]);
  });

  it("発注ステータスと進捗状況の名称が混ざる statuses は OR(後段で絞る)", async () => {
    const f = fakeFind({
      ...base,
      purchase_orders: {
        list: () => [
          po(1, { expenditure_status: 5, delivery_status: 1 }), // 発注済
          po(2, { expenditure_status: 4, delivery_status: 2 }), // 着手中
          po(3, { expenditure_status: 4, delivery_status: 1 }), // どちらでもない
        ],
      },
    });
    const r = await findPurchaseOrders(f.session(), {
      partnerId: 3,
      statuses: ["発注済", "着手中"],
    });
    expect(r.map((x) => x.PurchaseOrder.id)).toEqual([1, 2]);
    // API には絞り込みなしで取りに行く(AND にならないように)
    expect(f.calls("purchase_orders.list")[0]).toContain('{"payeeIdEq":3}');
  });

  it("知らないステータス名は argument_error(発注ステータスと進捗状況の名称を載せる)", async () => {
    const f = fakeFind({ ...base, purchase_orders: { list: () => [] } });
    const e = await rejects(findPurchaseOrders(f.session(), { status: "open" }));
    expect(e?.code).toBe("argument_error");
    for (const name of [
      "見積中(高)",
      "発注確定",
      "発注済",
      "見送り",
      "未着手",
      "着手中",
      "納品済",
      "検収済",
    ]) {
      expect(e?.message).toContain(name);
    }
    expect(f.log).toEqual([]);
  });

  it("複数の仕入先にマッチしたら曖昧エラー(vendors)", async () => {
    const f = fakeFind({ vendors: { list: () => entities(2, "V") }, purchase_orders: {} });
    const e = await rejects(findPurchaseOrders(f.session(), { partnerName: "V" }));
    expect(e?.message).toContain("multiple vendors match");
  });
});

describe("find_payments(Go: find_payment_test.go + issue #27)", () => {
  const partner = (id: number) => ({ id, name: "V", name_disp: "V", custom_no: "" });
  const pay = (id: number, extra: Partial<PaymentEntity> = {}) =>
    paymentEntity({ id, payee: partner(3), ...extra });
  it("id: Project は常に null、url は付かない。仕入先(payee.id)は付く", async () => {
    const f = fakeFind({
      payments: { get: (id: number) => pay(id) },
      vendors: { get: (id: number) => ({ id, name: "V" }) },
    });
    const r = await findPayments(f.session(), { id: 1 });
    expect(r[0]).toEqual({ Payment: pay(1), Vendor: { id: 3, name: "V" }, Project: null });
    expect("url" in (r[0] ?? {})).toBe(false);
    expect(f.calls("projects.")).toHaveLength(0);
  });
  it("仕入先の取得に失敗しても致命的でない / payee がなければ取りに行かない", async () => {
    const f1 = fakeFind({
      payments: { get: (id: number) => pay(id) },
      vendors: { get: () => Promise.reject(apiError()) },
    });
    const s = f1.session();
    expect((await findPayments(s, { id: 1 }))[0]?.Vendor).toBeNull();
    expect(s.warnings).toHaveLength(1);
    const f2 = fakeFind({ payments: { get: (id: number) => pay(id, { payee: null }) } });
    await findPayments(f2.session(), { id: 1 });
    expect(f2.calls("vendors.")).toHaveLength(0);
  });
  it("vendor / status 名は expenditurePayeeIdEq / paymentStatusIn に委譲、statuses 単独は拒否", async () => {
    const f = fakeFind({
      payments: { list: () => [pay(1)] },
      vendors: { get: (id: number) => ({ id }) },
    });
    await findPayments(f.session(), { status: "支払済" });
    await findPayments(f.session(), { partnerId: 3, statuses: ["請求書未受領", "振込予約済"] });
    expect(f.calls("payments.list").map((l) => l.split(" ")[1])).toEqual([
      '{"paymentStatusIn":[3]}',
      '{"expenditurePayeeIdEq":3,"paymentStatusIn":[1,4]}',
    ]);
    expect((await rejects(findPayments(f.session(), { statuses: ["支払済"] })))?.code).toBe(
      "argument_error",
    );
  });
  it("知らないステータス名は argument_error(受け付ける名称を載せる)", async () => {
    const f = fakeFind({ payments: { list: () => [] } });
    const e = await rejects(findPayments(f.session(), { status: "paid" }));
    expect(e?.code).toBe("argument_error");
    for (const name of ["請求書未受領", "請求書受領済", "振込予約済", "支払済"]) {
      expect(e?.message).toContain(name);
    }
    expect(f.log).toEqual([]);
  });
});

describe("書類 4 種(Go: find_estimate_test.go ほか)", () => {
  const project = (id: number, extra: object = {}) => ({
    id,
    name: `案件${id}`,
    client: { id: 7 },
    ...extra,
  });
  const clients = { clients: { get: (id: number) => ({ id, name: `顧客${id}` }) } };

  describe("id", () => {
    it("逆引きで案件が分かれば、案件と顧客が付く。url は documents/{project_id}/edit", async () => {
      const f = fakeFind({
        ...clients,
        estimates: { get: (id: number) => ({ id }) },
        projects: { get: (id: number) => project(id) },
        lookup: () => ({ status: "hit", projectId: 42 }),
      });
      const r = await findEstimates(f.session(), { id: 10 });
      expect(r).toEqual([
        {
          Estimate: { id: 10 },
          ProjectID: 42,
          ClientID: 7,
          Project: project(42),
          Client: { id: 7, name: "顧客7" },
          url: "https://the-board.jp/documents/42/edit",
        },
      ]);
      expect(f.lookup.mock.calls[0]?.slice(0, 2)).toEqual(["estimate", 10]);
    });

    it("逆引きで解決できなければ書類だけ返す(ProjectID=0)。理由つきの警告が出る", async () => {
      for (const [lookup, reason] of [
        [{ status: "miss", reason: "not_found" }, "not_found"],
        [{ status: "unresolved", reason: "deadline" }, "deadline"],
        [{ status: "unresolved", reason: "scan_in_progress" }, "scan_in_progress"],
        [{ status: "unresolved", reason: "api_error", errorCode: "rate_limited" }, "api_error"],
      ] as const) {
        const f = fakeFind({
          orders: { get: (id: number) => ({ id }) },
          lookup: () => lookup,
        });
        const s = f.session();
        const r = await findOrders(s, { id: 3 });
        expect(r).toEqual([
          { Order: { id: 3 }, ProjectID: 0, ClientID: 0, Project: null, Client: null },
        ]);
        expect(s.warnings).toHaveLength(1);
        expect(s.warnings[0]).toMatchObject({
          code: "project_unresolved",
          resource: "orders",
          document_id: 3,
          reason,
        });
        expect(f.calls("projects.")).toHaveLength(0);
      }
    });

    it("逆引き自体が例外でも検索は止まらない(警告つきで縮退)", async () => {
      const f = fakeFind({
        deliveries: { get: (id: number) => ({ id }) },
        lookup: () => Promise.reject(new Error("boom with secret-key")),
      });
      const s = f.session();
      const r = await findDeliveries(s, { id: 3 });
      expect(r[0]?.ProjectID).toBe(0);
      expect(JSON.stringify(s.warnings)).not.toContain("secret-key");
      expect(s.warnings[0]).toMatchObject({ reason: "api_error", error_code: "generic_error" });
    });

    it("書類の取得の失敗は伝わる(逆引きには進まない)", async () => {
      const f = fakeFind({ receipts: { get: () => Promise.reject(notFound()) } });
      expect((await rejects(findReceipts(f.session(), { id: 1 })))?.code).toBe("not_found");
      expect(f.lookup).not.toHaveBeenCalled();
    });

    it("案件の取得に失敗しても致命的でない: ProjectID だけ返す", async () => {
      const f = fakeFind({
        estimates: { get: (id: number) => ({ id }) },
        projects: { get: () => Promise.reject(apiError()) },
        lookup: () => ({ status: "hit", projectId: 42 }),
      });
      const s = f.session();
      const r = await findEstimates(s, { id: 1 });
      expect(r[0]).toMatchObject({ ProjectID: 42, ClientID: 0, Project: null, Client: null });
      expect(s.warnings[0]).toMatchObject({ code: "enrichment_failed", resource: "projects" });
    });

    it("id は project_id に優先する", async () => {
      const f = fakeFind({
        estimates: { get: (id: number) => ({ id }) },
        lookup: () => ({ status: "miss", reason: "not_found" }),
      });
      await findEstimates(f.session(), { id: 1, projectId: 9, clientName: "x" });
      expect(f.calls("projects.")).toHaveLength(0);
    });
  });

  describe("project_id", () => {
    it("response_group=<種別> 付きで案件を取り、書類を取る。逆引きにも記録する", async () => {
      const f = fakeFind({
        ...clients,
        projectsWithGroup: (id) => project(id, { estimate: { id: 500 } }),
        estimates: { get: (id: number) => ({ id }) },
      });
      const r = await findEstimates(f.session(), { projectId: 42 });
      expect(r[0]).toMatchObject({ Estimate: { id: 500 }, ProjectID: 42, ClientID: 7 });
      expect(f.calls("projects.getWithGroup")).toEqual(["projects.getWithGroup 42 estimate"]);
      expect(f.calls("index.record")).toEqual(["index.record estimate 42"]);
    });

    it("書類がない案件は空配列", async () => {
      const f = fakeFind({ projectsWithGroup: (id) => project(id) });
      expect(await findEstimates(f.session(), { projectId: 42 })).toEqual([]);
    });

    it("案件の取得の失敗は伝わる / 書類が 404 なら飛ばす", async () => {
      const f1 = fakeFind({ projectsWithGroup: () => Promise.reject(notFound()) });
      expect((await rejects(findOrders(f1.session(), { projectId: 1 })))?.code).toBe("not_found");
      const f2 = fakeFind({
        ...clients,
        projectsWithGroup: (id) => project(id, { deliveries: [{ id: 1 }, { id: 2 }] }),
        deliveries: {
          get: (id: number) => (id === 1 ? Promise.reject(notFound()) : { id }),
        },
      });
      const r = await findDeliveries(f2.session(), { projectId: 5 });
      expect(r.map((x) => x.Delivery.id)).toEqual([2]);
    });

    it("書類の取得が 404 以外で失敗したら伝わる", async () => {
      const f = fakeFind({
        projectsWithGroup: (id) => project(id, { order: { id: 1 } }),
        orders: { get: () => Promise.reject(apiError()) },
      });
      expect((await rejects(findOrders(f.session(), { projectId: 1 })))?.code).toBe("api_error");
    });

    it("納品・領収は案件の配列を全件ループする", async () => {
      const f = fakeFind({
        ...clients,
        projectsWithGroup: (id) =>
          project(id, { deliveries: [{ id: 1 }, { id: 2 }], receipts: [{ id: 8 }, { id: 9 }] }),
        deliveries: { get: (id: number) => ({ id }) },
        receipts: { get: (id: number) => ({ id }) },
      });
      expect(
        (await findDeliveries(f.session(), { projectId: 5 })).map((x) => x.Delivery.id),
      ).toEqual([1, 2]);
      // 顧客は案件ごとに 1 回だけ(書類が 2 件あっても)。
      expect(f.calls("clients.get")).toHaveLength(1);
      expect((await findReceipts(f.session(), { projectId: 5 })).map((x) => x.Receipt.id)).toEqual([
        8, 9,
      ]);
    });
  });

  describe("client_name / project_name(fanout)", () => {
    const fan = {
      clients: {
        list: () => [
          { id: 1, name: "A1" },
          { id: 2, name: "A2" },
        ],
        get: (id: number) => ({ id, name: `顧客${id}` }),
      },
      projects: {
        list: (f: { clientIdEq?: number; nameCont?: string }) =>
          f.clientIdEq
            ? [
                project(f.clientIdEq * 10, {
                  estimate: { id: f.clientIdEq * 100 },
                  client: { id: f.clientIdEq },
                }),
              ]
            : [
                project(11, { estimate: { id: 111 } }),
                project(12, { estimate: null }),
                project(13, { estimate: { id: 113 } }),
              ],
      },
      estimates: { get: (id: number) => ({ id }) },
    };

    it("client_name: マッチした全顧客の案件に fanout する(曖昧エラーにしない)。顧客は取り直さない", async () => {
      const f = fakeFind(fan);
      const r = await findEstimates(f.session(), { clientName: "A", projectName: "保守" });
      expect(r.map((x) => [x.ProjectID, x.ClientID, x.Estimate.id])).toEqual([
        [10, 1, 100],
        [20, 2, 200],
      ]);
      expect(r[0]?.Client).toEqual({ id: 1, name: "A1" });
      expect(f.calls("clients.get")).toHaveLength(0);
      expect(f.calls("projects.list")[0]).toContain(
        '{"clientIdEq":1,"nameCont":"保守","responseGroup":"estimate"}',
      );
      expect(f.calls("index.record")).toEqual([
        "index.record estimate 10",
        "index.record estimate 20",
      ]);
    });

    it("project_name: 書類のない案件は飛ばし、顧客は案件ごとに補助情報として取る", async () => {
      const f = fakeFind(fan);
      const r = await findEstimates(f.session(), { projectName: "保守" });
      expect(r.map((x) => x.ProjectID)).toEqual([11, 13]);
      expect(f.calls("projects.list")[0]).toContain(
        '{"nameCont":"保守","responseGroup":"estimate"}',
      );
      expect(r[0]?.Client).toEqual({ id: 7, name: "顧客7" });
      expect(f.calls("clients.get")).toHaveLength(1);
    });

    it("limit=1: 1 件で止まり、残りの書類は取らない。切ったので total_before_limit が付く", async () => {
      const f = fakeFind(fan);
      const s = f.session();
      const r = await findEstimates(s, { clientName: "A", limit: 1 });
      expect(r).toHaveLength(1);
      expect(f.calls("estimates.get")).toHaveLength(1);
      // limit に達した時点で残りの顧客(2 件目)の案件の検索を止めるので、候補数は 1(確定ではない。以前は 2)。
      expect(f.calls("projects.list")).toHaveLength(1);
      expect(s.totalBeforeLimit).toBe(1);
    });

    it("limit にちょうど達して候補が尽きたなら、切ったことにならない", async () => {
      const f = fakeFind(fan);
      const s = f.session();
      expect(await findEstimates(s, { clientName: "A", limit: 2 })).toHaveLength(2);
      expect(s.totalBeforeLimit).toBeUndefined();
    });

    it("顧客が 5 件を超えたら argument_error で拒否(案件の検索に進まない)", async () => {
      const f = fakeFind({ ...fan, clients: { list: () => entities(6, "A") } });
      const e = await rejects(findEstimates(f.session(), { clientName: "A" }));
      expect(e?.code).toBe("argument_error");
      expect(e?.message).toContain('client_name "A" matched 6 entities (cap=5)');
      expect(f.calls("projects.")).toHaveLength(0);
    });

    it("顧客がちょうど 5 件なら通る", async () => {
      const f = fakeFind({
        ...fan,
        clients: { list: () => entities(5, "A") },
        projects: { list: () => [] },
      });
      expect(await findEstimates(f.session(), { clientName: "A" })).toEqual([]);
    });

    it("案件が 5 件を超えたら argument_error(project_name)", async () => {
      const f = fakeFind({ ...fan, projects: { list: () => entities(6, "P") } });
      const e = await rejects(findOrders(f.session(), { projectName: "P" }));
      expect(e?.code).toBe("argument_error");
      expect(e?.message).toContain('project_name "P" matched 6 entities (cap=5)');
      expect(f.calls("orders.get")).toHaveLength(0);
    });

    describe("候補の書類の総数の上限(FIND_DOCUMENT_CANDIDATE_CAP = 200)", () => {
      /** 顧客 n 件、各顧客に案件 m 件(案件 1 件につき見積 1 件)。 */
      const many = (n: number, m: number) => ({
        clients: { list: () => entities(n, "A"), get: (id: number) => ({ id }) },
        projects: {
          list: (f: { clientIdEq?: number }) =>
            Array.from({ length: m }, (_, i) =>
              project((f.clientIdEq ?? 0) * 1000 + i + 1, {
                estimate: { id: (f.clientIdEq ?? 0) * 1000 + i + 1 },
                client: { id: f.clientIdEq ?? 0 },
              }),
            ),
        },
        estimates: { get: (id: number) => ({ id }) },
      });

      it("200 件を超えたら、書類を 1 件も取らずに argument_error。project_id か limit での絞り込みを促す", async () => {
        const f = fakeFind(many(3, 70)); // 210 件
        const e = await rejects(findEstimates(f.session(), { clientName: "A" }));
        expect(e?.code).toBe("argument_error");
        expect(e?.message).toContain("200");
        expect(e?.message).toContain("project_id");
        expect(e?.message).toContain("limit");
        expect(f.calls("estimates.get")).toHaveLength(0);
      });

      it("ちょうど 200 件なら通る", async () => {
        const f = fakeFind(many(4, 50));
        expect(await findEstimates(f.session(), { clientName: "A" })).toHaveLength(200);
      });

      it("上限を超えた時点で候補の収集を止める(残りの顧客の案件は検索しない)", async () => {
        const f = fakeFind(many(5, 100));
        await rejects(findEstimates(f.session(), { clientName: "A" }));
        expect(f.calls("projects.list")).toHaveLength(3); // 100 + 100 + 100 > 200
      });

      it("limit に達した時点で、残りの顧客の案件の検索を止める。total_before_limit は候補数(確定ではない)", async () => {
        const f = fakeFind(many(5, 10));
        const s = f.session();
        const r = await findEstimates(s, { clientName: "A", limit: 15 });
        expect(r).toHaveLength(15);
        expect(f.calls("projects.list")).toHaveLength(2);
        expect(f.calls("estimates.get")).toHaveLength(15);
        expect(s.totalBeforeLimit).toBe(20);
      });

      it("limit が大きく、全顧客を見ても足りなければ、全件を返し limited にしない", async () => {
        const f = fakeFind(many(3, 10));
        const s = f.session();
        expect(await findEstimates(s, { clientName: "A", limit: 100 })).toHaveLength(30);
        expect(s.totalBeforeLimit).toBeUndefined();
      });

      it("project_name の経路にも同じ上限が掛かる", async () => {
        const f = fakeFind({
          ...many(1, 1),
          projects: {
            list: () =>
              Array.from({ length: 5 }, (_, i) =>
                project(i + 1, {
                  deliveries: Array.from({ length: 50 }, (_, j) => ({ id: i * 100 + j + 1 })),
                }),
              ),
          },
          deliveries: { get: (id: number) => ({ id }) },
        });
        const e = await rejects(findDeliveries(f.session(), { projectName: "P" }));
        expect(e?.code).toBe("argument_error");
        expect(f.calls("deliveries.get")).toHaveLength(0);
      });
    });

    it("書類の取得が 404 の候補は飛ばす", async () => {
      const f = fakeFind({
        ...fan,
        estimates: { get: (id: number) => (id === 100 ? Promise.reject(notFound()) : { id }) },
      });
      const r = await findEstimates(f.session(), { clientName: "A" });
      expect(r.map((x) => x.Estimate.id)).toEqual([200]);
    });
  });

  it("空のクエリ・負の limit は argument_error(通信しない)", async () => {
    const f = fakeFind({});
    expect((await rejects(findEstimates(f.session(), {})))?.message).toContain(
      "at least one field",
    );
    expect((await rejects(findOrders(f.session(), { id: 1, limit: -1 })))?.code).toBe(
      "argument_error",
    );
    expect(f.log).toEqual([]);
  });

  it("結果のキーは種別ごと(Estimate / Order / Delivery / Receipt)", async () => {
    const f = fakeFind({
      estimates: { get: (id: number) => ({ id }) },
      orders: { get: (id: number) => ({ id }) },
      deliveries: { get: (id: number) => ({ id }) },
      receipts: { get: (id: number) => ({ id }) },
      lookup: () => ({ status: "miss", reason: "not_found" }),
    });
    const keys = async (p: Promise<object[]>) => Object.keys((await p)[0] ?? {});
    const expected = (k: string) => [k, "ProjectID", "ClientID", "Project", "Client"];
    expect(await keys(findEstimates(f.session(), { id: 1 }))).toEqual(expected("Estimate"));
    expect(await keys(findOrders(f.session(), { id: 1 }))).toEqual(expected("Order"));
    expect(await keys(findDeliveries(f.session(), { id: 1 }))).toEqual(expected("Delivery"));
    expect(await keys(findReceipts(f.session(), { id: 1 }))).toEqual(expected("Receipt"));
  });
});

describe("共通の振る舞い(session)", () => {
  it("refresh はリソースごとに最初の 1 回の読み取りにだけ付く。書類は 1 件ごとに効く", async () => {
    const f = fakeFind(
      {
        projects: {
          list: () => [
            { id: 1, client: { id: 7 } },
            { id: 2, client: { id: 8 } },
          ],
        },
        clients: { get: (id: number) => ({ id }) },
      },
      { refresh: "delta" },
    );
    await findProjects(f.session(), { name: "x" });
    expect(f.calls("projects.list")[0]).toContain('{"refresh":"delta"}');
    const gets = f.calls("clients.get");
    expect(gets[0]).toContain('{"refresh":"delta"}');
    expect(gets[1]).toMatch(/ \{\}$/);

    const g = fakeFind(
      {
        projectsWithGroup: (id) => ({ id, deliveries: [{ id: 1 }, { id: 2 }] }),
        deliveries: { get: (id: number) => ({ id }) },
      },
      { refresh: "full" },
    );
    await findDeliveries(g.session(), { projectId: 3 });
    expect(g.calls("deliveries.get").every((l) => l.endsWith('{"refresh":"full"}'))).toBe(true);
  });

  it("cache は読んだリソースを初めて出た順にまとめる(refresh_state は引き継ぐ)", async () => {
    const f = fakeFind({
      projects: { list: () => [{ id: 1, client: { id: 7 } }] },
      clients: { get: (id: number) => ({ id }) },
    });
    const s = f.session();
    await findProjects(s, { name: "x" });
    expect(s.cache.map((c) => c.resource)).toEqual(["projects", "clients"]);
  });

  it("警告は MAX_WARNINGS 件まで。超えた分は warnings_truncated にまとめる", async () => {
    const f = fakeFind({
      projects: {
        list: () => Array.from({ length: 25 }, (_, i) => ({ id: i + 1, client: { id: 100 + i } })),
      },
      clients: { get: () => Promise.reject(notFound()) },
    });
    const s = f.session();
    await findProjects(s, { name: "x" });
    expect(s.warnings).toHaveLength(MAX_WARNINGS + 1);
    expect(s.warnings.at(-1)).toEqual({ code: "warnings_truncated", omitted: 5 });
  });

  it("repository が truncated を返したら list_truncated の警告", async () => {
    const f = fakeFind({ projects: { list: () => [] } });
    const repos = f.session().repos as unknown as { projects: { list: unknown } };
    const orig = repos.projects.list as (...a: unknown[]) => Promise<object>;
    repos.projects.list = async (...a: unknown[]) => ({ ...(await orig(...a)), truncated: true });
    const s = f.session();
    await findProjects(s, { name: "x" });
    expect(s.warnings).toEqual([
      expect.objectContaining({ code: "list_truncated", resource: "projects" }),
    ]);
  });
});
