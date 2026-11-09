/**
 * 4 リソースのエンティティ型・フィクスチャが、BOARD の OpenAPI(v1.9.0)の一覧応答のキーの部分集合であること(issue #27)。
 * 型の検査は tsc(`mise run lint`)で効く。`[key: string]: unknown` が付いていると keyof が string になって落ちる。
 */
import { describe, expect, it } from "vitest";
import type {
  InvoiceEntity,
  PaymentEntity,
  PurchaseOrderEntity,
  UserEntity,
} from "../../src/board/resources";
import { invoiceEntity, paymentEntity, purchaseOrderEntity, userEntity } from "../helpers/entities";
import {
  OPENAPI_EXPENDITURE_KEYS,
  OPENAPI_EXPENDITURE_PAYMENT_KEYS,
  OPENAPI_INVOICE_KEYS,
  OPENAPI_PARTNER_REF_KEYS,
  OPENAPI_PROJECT_REF_KEYS,
  OPENAPI_USER_KEYS,
} from "../helpers/openapi-fields";

/** E のキーが K の部分集合でなければ型エラーになる(never にならない)。 */
type KeysWithin<E, K extends string> = [Exclude<keyof E, K>] extends [never] ? true : false;
const typeChecks: {
  user: KeysWithin<UserEntity, (typeof OPENAPI_USER_KEYS)[number]>;
  invoice: KeysWithin<InvoiceEntity, (typeof OPENAPI_INVOICE_KEYS)[number]>;
  purchaseOrder: KeysWithin<PurchaseOrderEntity, (typeof OPENAPI_EXPENDITURE_KEYS)[number]>;
  payment: KeysWithin<PaymentEntity, (typeof OPENAPI_EXPENDITURE_PAYMENT_KEYS)[number]>;
} = { user: true, invoice: true, purchaseOrder: true, payment: true };

const outside = (obj: object, allowed: readonly string[]) =>
  Object.keys(obj).filter((k) => !allowed.includes(k));

describe("エンティティ型は OpenAPI のキーの部分集合", () => {
  it("4 リソースの型の検査が型レベルで通っている(落ちるときは tsc が止める)", () => {
    expect(typeChecks).toEqual({ user: true, invoice: true, purchaseOrder: true, payment: true });
  });
});

describe("フィクスチャのキーは OpenAPI のキーの部分集合(実在しない項目を持ち込まない)", () => {
  it("users", () => {
    expect(outside(userEntity(), OPENAPI_USER_KEYS)).toEqual([]);
  });

  it("invoices(client の入れ子も)", () => {
    const e = invoiceEntity();
    expect(outside(e, OPENAPI_INVOICE_KEYS)).toEqual([]);
    expect(outside(e.client ?? {}, OPENAPI_PARTNER_REF_KEYS)).toEqual([]);
  });

  it("purchase_orders(payee / project の入れ子も)", () => {
    const e = purchaseOrderEntity();
    expect(outside(e, OPENAPI_EXPENDITURE_KEYS)).toEqual([]);
    expect(outside(e.payee ?? {}, OPENAPI_PARTNER_REF_KEYS)).toEqual([]);
    expect(outside(e.project ?? {}, OPENAPI_PROJECT_REF_KEYS)).toEqual([]);
  });

  it("payments(payee の入れ子も)", () => {
    const e = paymentEntity();
    expect(outside(e, OPENAPI_EXPENDITURE_PAYMENT_KEYS)).toEqual([]);
    expect(outside(e.payee ?? {}, OPENAPI_PARTNER_REF_KEYS)).toEqual([]);
  });

  it("検査自体が効く: 旧型の項目(users.name / invoices.client_id / status)は部分集合から外れる", () => {
    expect(outside({ id: 1, name: "x" }, OPENAPI_USER_KEYS)).toEqual(["name"]);
    expect(outside({ client_id: 1, status: "x" }, OPENAPI_INVOICE_KEYS)).toEqual([
      "client_id",
      "status",
    ]);
    expect(outside({ vendor_id: 1, status: "x" }, OPENAPI_EXPENDITURE_KEYS)).toEqual([
      "vendor_id",
      "status",
    ]);
    expect(outside({ purchase_order_id: 1 }, OPENAPI_EXPENDITURE_PAYMENT_KEYS)).toEqual([
      "purchase_order_id",
    ]);
  });
});
