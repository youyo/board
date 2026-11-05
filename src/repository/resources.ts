/** 22 リソースの振る舞いの表(根拠は plans/board-workers-m03-d1-cache.md)。 */

import {
  type AccountingTypeEntity,
  type AccountingTypeListOptions,
  buildAccountingTypesQuery,
  buildClientBranchesQuery,
  buildClientsQuery,
  buildContactsQuery,
  buildDocumentSendChannelsQuery,
  buildGroupsQuery,
  buildInvoicesQuery,
  buildPaymentsQuery,
  buildPaymentTermsQuery,
  buildProjectCostsQuery,
  buildProjectsQuery,
  buildProjectTypesQuery,
  buildPurchaseOrdersQuery,
  buildPurchaseTypesQuery,
  buildUsersQuery,
  buildVendorBranchesQuery,
  buildVendorContactsQuery,
  buildVendorsQuery,
  type ClientBranchEntity,
  type ClientBranchListOptions,
  type ClientEntity,
  type ClientListOptions,
  type CommonListOptions,
  type ContactEntity,
  type ContactListOptions,
  type DeliveryEntity,
  type DocumentSendChannelEntity,
  type DocumentSendChannelListOptions,
  type EstimateEntity,
  type GroupEntity,
  type GroupListOptions,
  getAccountingType,
  getClient,
  getClientBranch,
  getContact,
  getDelivery,
  getDocumentSendChannel,
  getEstimate,
  getGroup,
  getInvoice,
  getOrder,
  getPayment,
  getPaymentTerm,
  getProject,
  getProjectCost,
  getProjectType,
  getPurchaseOrder,
  getPurchaseType,
  getReceipt,
  getUser,
  getVendor,
  getVendorBranch,
  getVendorContact,
  type InvoiceEntity,
  type InvoiceListOptions,
  listAccountingTypes,
  listClientBranches,
  listClients,
  listContacts,
  listDocumentSendChannels,
  listGroups,
  listInvoices,
  listPayments,
  listPaymentTerms,
  listProjectCosts,
  listProjects,
  listProjectTypes,
  listPurchaseOrders,
  listPurchaseTypes,
  listUsers,
  listVendorBranches,
  listVendorContacts,
  listVendors,
  type OrderEntity,
  type PaymentEntity,
  type PaymentListOptions,
  type PaymentTermEntity,
  type PaymentTermListOptions,
  type ProjectCostEntity,
  type ProjectCostListOptions,
  type ProjectEntity,
  type ProjectListOptions,
  type ProjectTypeEntity,
  type ProjectTypeListOptions,
  type PurchaseOrderEntity,
  type PurchaseOrderListOptions,
  type PurchaseTypeEntity,
  type PurchaseTypeListOptions,
  type ReceiptEntity,
  type UserEntity,
  type UserListOptions,
  type VendorBranchEntity,
  type VendorBranchListOptions,
  type VendorContactEntity,
  type VendorContactListOptions,
  type VendorEntity,
  type VendorListOptions,
} from "../board/resources";
import { foldContains } from "../cache/fold";
import { type CacheFirstSpec, type DocumentSpec, type NonzeroApiSpec, pathOf } from "./spec";

function cacheFirst<E, F extends CommonListOptions>(
  resource: string,
  s: Omit<CacheFirstSpec<E, F>, "kind" | "resource" | "path">,
): CacheFirstSpec<E, F> {
  return { kind: "cache-first", resource, path: pathOf(resource), ...s };
}

function nonzeroApi<E, F extends CommonListOptions>(
  resource: string,
  s: Omit<NonzeroApiSpec<E, F>, "kind" | "resource" | "path">,
): NonzeroApiSpec<E, F> {
  return { kind: "nonzero-api", resource, path: pathOf(resource), ...s };
}

function document<E>(resource: string, get: DocumentSpec<E>["get"]): DocumentSpec<E> {
  return { kind: "document", resource, path: pathOf(resource), get };
}

// 完全一致の比較。null / 未設定は空文字扱い(Go の `*string` を空文字に直して比べる処理)。
const str = (v: unknown): string => (typeof v === "string" ? v : "");

export const SPECS = {
  // --- cache-first ---------------------------------------------------------------------------
  clients: cacheFirst<ClientEntity, ClientListOptions>("clients", {
    buildQuery: buildClientsQuery,
    list: listClients,
    get: getClient,
    cacheFirst: {
      keys: ["nameCont", "nameDispCont", "invoiceSystemNumberEq", "customNoEq"],
      match: (c, f) =>
        (!f.nameCont || foldContains(c.name, f.nameCont)) &&
        (!f.nameDispCont || foldContains(c.name_disp, f.nameDispCont)) &&
        (!f.invoiceSystemNumberEq || str(c.invoice_system_number) === f.invoiceSystemNumberEq) &&
        (!f.customNoEq || str(c.custom_no) === f.customNoEq),
    },
  }),
  projects: cacheFirst<ProjectEntity, ProjectListOptions>("projects", {
    buildQuery: buildProjectsQuery,
    list: listProjects,
    get: (client, id, ctx) => getProject(client, id, {}, ctx),
    cacheFirst: {
      keys: ["nameCont", "clientIdEq", "clientNameCont", "projectNoEq", "managementNoEq"],
      match: (p, f) =>
        (!f.nameCont || foldContains(p.name, f.nameCont)) &&
        (!f.clientIdEq || p.client?.id === f.clientIdEq) &&
        (!f.clientNameCont ||
          (p.client != null && foldContains(p.client.name, f.clientNameCont))) &&
        (!f.projectNoEq || (p.project_no != null && String(p.project_no) === f.projectNoEq)) &&
        (!f.managementNoEq || str(p.management_no) === f.managementNoEq),
    },
  }),
  invoices: cacheFirst<InvoiceEntity, InvoiceListOptions>("invoices", {
    buildQuery: buildInvoicesQuery,
    list: listInvoices,
    get: getInvoice,
    cacheFirst: {
      keys: ["clientIdEq", "projectIdEq", "statusEq"],
      match: (i, f) =>
        (!f.clientIdEq || i.client_id === f.clientIdEq) &&
        (!f.projectIdEq || i.project_id === f.projectIdEq) &&
        (!f.statusEq || i.status === f.statusEq),
    },
  }),
  purchase_orders: cacheFirst<PurchaseOrderEntity, PurchaseOrderListOptions>("purchase_orders", {
    buildQuery: buildPurchaseOrdersQuery,
    list: listPurchaseOrders,
    get: getPurchaseOrder,
    cacheFirst: {
      keys: ["vendorIdEq", "projectIdEq", "statusEq"],
      match: (p, f) =>
        (!f.vendorIdEq || p.vendor_id === f.vendorIdEq) &&
        (!f.projectIdEq || p.project_id === f.projectIdEq) &&
        (!f.statusEq || p.status === f.statusEq),
    },
  }),
  payments: cacheFirst<PaymentEntity, PaymentListOptions>("payments", {
    buildQuery: buildPaymentsQuery,
    list: listPayments,
    get: getPayment,
    cacheFirst: {
      keys: ["vendorIdEq", "purchaseOrderIdEq", "statusEq"],
      match: (p, f) =>
        (!f.vendorIdEq || p.vendor_id === f.vendorIdEq) &&
        (!f.purchaseOrderIdEq || p.purchase_order_id === f.purchaseOrderIdEq) &&
        (!f.statusEq || p.status === f.statusEq),
    },
  }),
  users: cacheFirst<UserEntity, UserListOptions>("users", {
    buildQuery: buildUsersQuery,
    list: listUsers,
    get: getUser,
    cacheFirst: {
      keys: ["nameCont", "emailCont"],
      match: (u, f) =>
        (!f.nameCont || foldContains(u.name, f.nameCont)) &&
        (!f.emailCont || foldContains(u.email, f.emailCont)),
    },
  }),
  vendors: cacheFirst<VendorEntity, VendorListOptions>("vendors", {
    buildQuery: buildVendorsQuery,
    list: listVendors,
    get: getVendor,
    cacheFirst: {
      keys: ["nameCont"],
      match: (v, f) => !f.nameCont || foldContains(v.name, f.nameCont),
    },
  }),

  // --- nonzero-api ---------------------------------------------------------------------------
  client_branches: nonzeroApi<ClientBranchEntity, ClientBranchListOptions>("client_branches", {
    buildQuery: buildClientBranchesQuery,
    list: listClientBranches,
    get: getClientBranch,
  }),
  contacts: nonzeroApi<ContactEntity, ContactListOptions>("contacts", {
    buildQuery: buildContactsQuery,
    list: listContacts,
    get: getContact,
  }),
  project_costs: nonzeroApi<ProjectCostEntity, ProjectCostListOptions>("project_costs", {
    buildQuery: buildProjectCostsQuery,
    list: listProjectCosts,
    get: getProjectCost,
  }),
  vendor_branches: nonzeroApi<VendorBranchEntity, VendorBranchListOptions>("vendor_branches", {
    buildQuery: buildVendorBranchesQuery,
    list: listVendorBranches,
    get: getVendorBranch,
  }),
  vendor_contacts: nonzeroApi<VendorContactEntity, VendorContactListOptions>("vendor_contacts", {
    buildQuery: buildVendorContactsQuery,
    list: listVendorContacts,
    get: getVendorContact,
  }),
  groups: nonzeroApi<GroupEntity, GroupListOptions>("groups", {
    buildQuery: buildGroupsQuery,
    list: listGroups,
    get: getGroup,
  }),
  payment_terms: nonzeroApi<PaymentTermEntity, PaymentTermListOptions>("payment_terms", {
    buildQuery: buildPaymentTermsQuery,
    list: listPaymentTerms,
    get: getPaymentTerm,
  }),
  project_types: nonzeroApi<ProjectTypeEntity, ProjectTypeListOptions>("project_types", {
    buildQuery: buildProjectTypesQuery,
    list: listProjectTypes,
    get: getProjectType,
  }),
  purchase_types: nonzeroApi<PurchaseTypeEntity, PurchaseTypeListOptions>("purchase_types", {
    buildQuery: buildPurchaseTypesQuery,
    list: listPurchaseTypes,
    get: getPurchaseType,
  }),
  accounting_types: nonzeroApi<AccountingTypeEntity, AccountingTypeListOptions>(
    "accounting_types",
    {
      buildQuery: buildAccountingTypesQuery,
      list: listAccountingTypes,
      get: getAccountingType,
    },
  ),
  document_send_channels: nonzeroApi<DocumentSendChannelEntity, DocumentSendChannelListOptions>(
    "document_send_channels",
    {
      buildQuery: buildDocumentSendChannelsQuery,
      list: listDocumentSendChannels,
      get: getDocumentSendChannel,
    },
  ),

  // --- document(get のみ)----------------------------------------------------------------------
  estimates: document<EstimateEntity>("estimates", getEstimate),
  orders: document<OrderEntity>("orders", getOrder),
  deliveries: document<DeliveryEntity>("deliveries", getDelivery),
  receipts: document<ReceiptEntity>("receipts", getReceipt),
};

export type Specs = typeof SPECS;
