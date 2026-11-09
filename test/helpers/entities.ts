/**
 * 4 リソース(users / invoices / purchase_orders / payments)の、BOARD の実際の応答の形のフィクスチャ。
 * `satisfies XxxEntity` で型に合わせ、`test/unit/openapi-entity-shape.test.ts` が OpenAPI のキーの部分集合かを検査する。
 * 実在しない項目(`users.name` / `invoices.client_id` / `status` など)をテストに持ち込まない。
 */
import type {
  InvoiceEntity,
  PaymentEntity,
  PurchaseOrderEntity,
  UserEntity,
} from "../../src/board/resources";

const AT = "2026-01-01T00:00:00.000+09:00";

export function userEntity(over: Partial<UserEntity> = {}) {
  return {
    id: 1,
    email: "taro.yamada@example.com",
    last_name: "山田",
    first_name: "太郎",
    role_id: 1,
    role_name: "管理者",
    last_sign_in_at: AT,
    valid_flg: 1,
    created_at: AT,
    updated_at: AT,
    ...over,
  } satisfies UserEntity;
}

export function invoiceEntity(over: Partial<InvoiceEntity> = {}) {
  return {
    id: 1,
    project_id: 100,
    project_no: 1001,
    management_no: "M-001",
    name: "Web サイト制作",
    client: { id: 10, name: "株式会社テスト", name_disp: "テスト", custom_no: "C-10" },
    total: "110000.0",
    tax: "10000.0",
    invoice_date: "2026-01-31",
    payment_limit_date: "2026-02-28",
    order_status: 5,
    order_status_name: "受注済",
    invoice_status: 2,
    invoice_status_name: "請求済",
    payment_method_kbn: 1,
    payment_method_kbn_name: "銀行振込",
    tags: [],
    lock_flg: 0,
    created_at: AT,
    updated_at: AT,
    ...over,
  } satisfies InvoiceEntity;
}

export function purchaseOrderEntity(over: Partial<PurchaseOrderEntity> = {}) {
  return {
    id: 1,
    expenditure_no: 2001,
    name: "デザイン外注",
    payee_id: 20,
    payee: { id: 20, name: "有限会社デザイン", name_disp: "デザイン", custom_no: "V-20" },
    project_id: 100,
    project: { id: 100, name: "Web サイト制作", project_no: 1001, management_no: "M-001" },
    expenditure_status: 5,
    expenditure_status_name: "発注済",
    delivery_status: 2,
    delivery_status_name: "着手中",
    total: "55000.0",
    tax: "5000.0",
    tags: [],
    lock_flg: 0,
    created_at: AT,
    updated_at: AT,
    ...over,
  } satisfies PurchaseOrderEntity;
}

export function paymentEntity(over: Partial<PaymentEntity> = {}) {
  return {
    id: 1,
    expenditure_id: 1,
    expenditure_no: 2001,
    name: "デザイン外注",
    payee: { id: 20, name: "有限会社デザイン", name_disp: "デザイン", custom_no: "V-20" },
    total: "55000.0",
    tax: "5000.0",
    invoice_date: "2026-02-01",
    payment_date: "2026-03-31",
    expenditure_status: 5,
    expenditure_status_name: "発注済",
    payment_status: 3,
    payment_status_name: "支払済",
    tags: [],
    lock_flg: 0,
    created_at: AT,
    updated_at: AT,
    ...over,
  } satisfies PaymentEntity;
}
