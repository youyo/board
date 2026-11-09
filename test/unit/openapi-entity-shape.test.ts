/**
 * 22 リソースのエンティティ型・フィクスチャが、BOARD の OpenAPI(v1.9.0)の応答キーの部分集合であること(issue #27 / #28)。
 * 型の検査は tsc(`mise run lint`)で効く。`[key: string]: unknown` が付いていると keyof が string になって落ちる。
 * 期待値は scripts/openapi-fields が仕様から生成した test/helpers/openapi-fields.ts。
 */
import { describe, expect, it } from "vitest";
import type {
  AccountingTypeEntity,
  ClientBranchEntity,
  ClientEntity,
  ContactEntity,
  DeliveryEntity,
  DocumentSendChannelEntity,
  EstimateEntity,
  GroupEntity,
  InvoiceEntity,
  OrderEntity,
  PaymentEntity,
  PaymentTermEntity,
  ProjectCostEntity,
  ProjectEntity,
  ProjectTypeEntity,
  PurchaseOrderEntity,
  PurchaseTypeEntity,
  ReceiptEntity,
  UserEntity,
  VendorBranchEntity,
  VendorContactEntity,
  VendorEntity,
} from "../../src/board/resources";
import { invoiceEntity, paymentEntity, purchaseOrderEntity, userEntity } from "../helpers/entities";
import { OPENAPI_RESOURCES, type OpenApiResourceName } from "../helpers/openapi-fields";

type Keys<R extends OpenApiResourceName> = (typeof OPENAPI_RESOURCES)[R]["responseKeys"][number];

/** E のキーが K の部分集合でなければ false(tsc が下のオブジェクトで止める)。 */
type KeysWithin<E, K extends string> = [Exclude<keyof E, K>] extends [never] ? true : false;

type NestedKeys<
  R extends OpenApiResourceName,
  N extends string,
> = (typeof OPENAPI_RESOURCES)[R] extends {
  nestedKeys: { readonly [P in N]: readonly (infer K)[] };
}
  ? K & string
  : never;

const typeChecks: { [R in OpenApiResourceName]: true } & { projectEmbeddedCost: true } = {
  clients: true as KeysWithin<ClientEntity, Keys<"clients">>,
  client_branches: true as KeysWithin<ClientBranchEntity, Keys<"client_branches">>,
  contacts: true as KeysWithin<ContactEntity, Keys<"contacts">>,
  projects: true as KeysWithin<ProjectEntity, Keys<"projects">>,
  project_costs: true as KeysWithin<ProjectCostEntity, Keys<"project_costs">>,
  estimates: true as KeysWithin<EstimateEntity, Keys<"estimates">>,
  invoices: true as KeysWithin<InvoiceEntity, Keys<"invoices">>,
  orders: true as KeysWithin<OrderEntity, Keys<"orders">>,
  deliveries: true as KeysWithin<DeliveryEntity, Keys<"deliveries">>,
  receipts: true as KeysWithin<ReceiptEntity, Keys<"receipts">>,
  vendors: true as KeysWithin<VendorEntity, Keys<"vendors">>,
  vendor_branches: true as KeysWithin<VendorBranchEntity, Keys<"vendor_branches">>,
  vendor_contacts: true as KeysWithin<VendorContactEntity, Keys<"vendor_contacts">>,
  purchase_orders: true as KeysWithin<PurchaseOrderEntity, Keys<"purchase_orders">>,
  payments: true as KeysWithin<PaymentEntity, Keys<"payments">>,
  users: true as KeysWithin<UserEntity, Keys<"users">>,
  groups: true as KeysWithin<GroupEntity, Keys<"groups">>,
  payment_terms: true as KeysWithin<PaymentTermEntity, Keys<"payment_terms">>,
  project_types: true as KeysWithin<ProjectTypeEntity, Keys<"project_types">>,
  purchase_types: true as KeysWithin<PurchaseTypeEntity, Keys<"purchase_types">>,
  accounting_types: true as KeysWithin<AccountingTypeEntity, Keys<"accounting_types">>,
  document_send_channels: true as KeysWithin<
    DocumentSendChannelEntity,
    Keys<"document_send_channels">
  >,
  // 案件に埋め込む project_costs の要素は、案件原価の一覧(ProjectCostEntity)とは別の形(cost / description / invoice_date / payment_date だけ)
  projectEmbeddedCost: true as KeysWithin<
    NonNullable<ProjectEntity["project_costs"]>[number],
    NestedKeys<"projects", "project_costs">
  >,
};

const outside = (obj: object, allowed: readonly string[]) =>
  Object.keys(obj).filter((k) => !allowed.includes(k));

describe("エンティティ型は OpenAPI のキーの部分集合", () => {
  it("22 リソース + 案件に埋め込む project_costs の型の検査が型レベルで通っている(落ちるときは tsc が止める)", () => {
    expect(Object.keys(typeChecks).sort()).toEqual(
      [...Object.keys(OPENAPI_RESOURCES), "projectEmbeddedCost"].sort(),
    );
    expect(Object.values(typeChecks).every((v) => v === true)).toBe(true);
  });

  it("案件に埋め込む project_costs の項目は 4 つだけ", () => {
    expect([...OPENAPI_RESOURCES.projects.nestedKeys.project_costs].sort()).toEqual([
      "cost",
      "description",
      "invoice_date",
      "payment_date",
    ]);
  });
});

describe("フィクスチャのキーは OpenAPI のキーの部分集合(実在しない項目を持ち込まない)", () => {
  it("users", () => {
    expect(outside(userEntity(), OPENAPI_RESOURCES.users.responseKeys)).toEqual([]);
  });

  it("invoices(client の入れ子も)", () => {
    const e = invoiceEntity();
    expect(outside(e, OPENAPI_RESOURCES.invoices.responseKeys)).toEqual([]);
    expect(outside(e.client ?? {}, OPENAPI_RESOURCES.invoices.nestedKeys.client)).toEqual([]);
  });

  it("purchase_orders(payee / project の入れ子も)", () => {
    const e = purchaseOrderEntity();
    const r = OPENAPI_RESOURCES.purchase_orders;
    expect(outside(e, r.responseKeys)).toEqual([]);
    expect(outside(e.payee ?? {}, r.nestedKeys.payee)).toEqual([]);
    expect(outside(e.project ?? {}, r.nestedKeys.project)).toEqual([]);
  });

  it("payments(payee の入れ子も)", () => {
    const e = paymentEntity();
    expect(outside(e, OPENAPI_RESOURCES.payments.responseKeys)).toEqual([]);
    expect(outside(e.payee ?? {}, OPENAPI_RESOURCES.payments.nestedKeys.payee)).toEqual([]);
  });

  it("検査自体が効く: 旧型の項目(users.name / invoices.client_id / status)は部分集合から外れる", () => {
    expect(outside({ id: 1, name: "x" }, OPENAPI_RESOURCES.users.responseKeys)).toEqual(["name"]);
    expect(outside({ client_id: 1, status: "x" }, OPENAPI_RESOURCES.invoices.responseKeys)).toEqual(
      ["client_id", "status"],
    );
    expect(
      outside({ vendor_id: 1, status: "x" }, OPENAPI_RESOURCES.purchase_orders.responseKeys),
    ).toEqual(["vendor_id", "status"]);
    expect(outside({ purchase_order_id: 1 }, OPENAPI_RESOURCES.payments.responseKeys)).toEqual([
      "purchase_order_id",
    ]);
  });
});

describe("生成物", () => {
  it("仕様のバージョンと取得元が書いてある", async () => {
    const m = await import("../helpers/openapi-fields");
    expect(m.OPENAPI_SPEC_VERSION).toBe("1.9.0");
    expect(m.OPENAPI_SPEC_URL).toBe("https://developers.the-board.jp/doc/board_openapi.json");
  });
});
