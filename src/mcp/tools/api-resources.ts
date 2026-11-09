/**
 * api 層(M04)のリソースの表。22 リソースの list / get ツールはこの表から作る(`api.ts`)。
 *
 * list のフィルタ引数は Go 版 CLI(`internal/cli/api_*.go`)の `cmd.Flags()` を snake_case にしたもの。
 * 引数 → repository のオプションの対応(`opt`)は `src/board/resources/*.ts` の `*ListOptions` に合わせる。
 * 根拠は `plans/board-workers-m04-api-tools.md` の一覧表を参照。
 */

import { LIST_FILTER_SUPPORT } from "../../board/openapi-support";
import { PURCHASE_ORDER_DEFAULT_RESPONSE_GROUP } from "../../board/resources/purchase_orders";

export type FieldKind =
  | { type: "str" }
  /** ID(0 以上の整数。0 は『指定なし』、負の数は argument_error)。 */
  | { type: "int" }
  | { type: "bool" }
  | { type: "strs" }
  | { type: "ints" }
  | { type: "enum"; values: readonly [string, ...string[]] };

export interface FieldDef {
  /** ツール引数名(snake_case。Go のフラグ名のハイフンを `_` にしたもの)。 */
  arg: string;
  /** repository の `*ListOptions` のキー。 */
  opt: string;
  kind: FieldKind;
  description: string;
  /** キャッシュ側で絞れる項目か(M03 の `cacheFirst.keys`)。 */
  cacheable?: boolean;
}

/** cache-first = ゼロフィルタはキャッシュ全件 / nonzero-api = 非ゼロのフィルタは常に API / document = get のみ。 */
export type ResourceMode = "cache-first" | "nonzero-api" | "document";

export interface ResourceDef {
  /** repository のキー(複数形 snake_case)。list ツールは `list_<resource>`。 */
  resource: string;
  /** get ツールは `get_<singular>`。自動の単数化はしない。 */
  singular: string;
  mode: ResourceMode;
  /** 日本語名。title に使う。 */
  label: string;
  /** そのリソースが何か、ほかのリソースとの関係(LLM が選ぶための説明)。 */
  summary: string;
  /** Go 版 CLI の根拠ファイル(`internal/cli/` 以下)。 */
  goFile: string;
  /** list のフィルタ(共通の updated_at_* / include_archive_flg を除く)。document は空。 */
  fields: readonly FieldDef[];
  /** list の response_group に使う列挙値(なければ未対応)。 */
  responseGroups?: readonly [string, ...string[]];
  /**
   * get の response_group に使う列挙値(なければ未対応)。指定すると API を直接呼び、キャッシュには書かない。
   * `cacheGroup` は指定しないときにキャッシュへ書く get が送る値(キャッシュの行はこれで揃える。issue #37)。
   */
  getResponseGroups?: { values: readonly [string, ...string[]]; cacheGroup: string };
}

const str = (arg: string, opt: string, description: string, cacheable = false): FieldDef => ({
  arg,
  opt,
  kind: { type: "str" },
  description,
  ...(cacheable ? { cacheable } : {}),
});
const int = (arg: string, opt: string, description: string, cacheable = false): FieldDef => ({
  arg,
  opt,
  kind: { type: "int" },
  description,
  ...(cacheable ? { cacheable } : {}),
});
const bool = (arg: string, opt: string, description: string): FieldDef => ({
  arg,
  opt,
  kind: { type: "bool" },
  description,
});
const strs = (arg: string, opt: string, description: string): FieldDef => ({
  arg,
  opt,
  kind: { type: "strs" },
  description,
});
const ints = (arg: string, opt: string, description: string, cacheable = false): FieldDef => ({
  arg,
  opt,
  kind: { type: "ints" },
  description,
  ...(cacheable ? { cacheable } : {}),
});

export const RESPONSE_GROUPS_COMMON = ["small", "large"] as const;
/** 発注書(expenditures)の一覧。medium は案件 `project` を含む。未指定なら medium を送る。 */
export const RESPONSE_GROUPS_PURCHASE_ORDERS = ["small", "medium", "large"] as const;
/** 案件の一覧の response_group(OpenAPI の列挙値。medium は支社・原価合計・請求条件など、project_cost は案件原価 `project_costs` を埋め込む)。 */
export const RESPONSE_GROUPS_PROJECTS_LIST = [
  "small",
  "medium",
  "large",
  "estimate",
  "order",
  "delivery",
  "invoice",
  "receipt",
  "project_cost",
  "all",
] as const;
export const RESPONSE_GROUPS_PROJECTS_GET = [
  "estimate",
  "order",
  "delivery",
  "invoice",
  "receipt",
  "all",
] as const;

/** OpenAPI v1.9.0 の GET /expenditures/{id} の response_group。 */
export const RESPONSE_GROUPS_PURCHASE_ORDERS_GET = [
  "small",
  "medium",
  "large",
  "estimate",
  "order",
  "delivery",
  "invoice",
  "all",
] as const;

const responseGroup = (values: readonly [string, ...string[]], note: string): FieldDef => ({
  arg: "response_group",
  opt: "responseGroup",
  kind: { type: "enum", values },
  description: `レスポンスに含める項目のグループ: ${values.join(" / ")}。${note}サーバー側の指定なので、指定すると BOARD の API を直接呼ぶ`,
});

const nameCont = (what: string, cacheable = false) =>
  str("name_cont", "nameCont", `${what}の名前で絞り込む(部分一致)`, cacheable);

/** 区分(1〜3)の選択。未指定は 1 と同じ結果。サーバー側の指定(キャッシュでは絞らない)。 */
const kbn = (arg: string, opt: string, what: string): FieldDef =>
  int(arg, opt, `${what}の種類(1〜3: ${what}1〜${what}3)。未指定は 1 と同じ結果`);

/** list の共通フィルタ(全 list ツールに付く。Go の `--updated-at-gteq` など)。 */
const COMMON_FILTERS: readonly FieldDef[] = [
  str("updated_at_gteq", "updatedAtGteq", "更新日時がこれ以降(`YYYY-MM-DD HH:MM:SS`)"),
  str("updated_at_lteq", "updatedAtLteq", "更新日時がこれ以前(`YYYY-MM-DD HH:MM:SS`)"),
  bool(
    "include_archive_flg",
    "includeArchiveFlg",
    "未指定ならアーカイブ済みも含める(このサーバーの既定。BOARD の既定とは逆)。false で除外する(include_archive_flg=0 を送る)。response_group が small(既定)の一覧応答には archive_flg が無いので、どれがアーカイブ済みかは応答から判別できない",
  ),
];

/**
 * そのリソースの list ツールに付く共通フィルタ。BOARD の OpenAPI に一覧のクエリとして無いものは付けない
 * (無視されて絞り込みが効かないため。issue #28)。どれが有るかは `LIST_FILTER_SUPPORT`(仕様から生成)から導く。
 */
export function commonFiltersFor(resource: string): readonly FieldDef[] {
  const support = LIST_FILTER_SUPPORT[resource];
  if (!support) return [];
  return COMMON_FILTERS.filter((f) => {
    switch (f.arg) {
      case "updated_at_gteq":
        return support.updatedAtGteq;
      case "updated_at_lteq":
        return support.updatedAtLteq;
      case "include_archive_flg":
        return support.includeArchiveFlg;
      default:
        return false;
    }
  });
}

export const API_RESOURCES: readonly ResourceDef[] = [
  {
    resource: "clients",
    singular: "client",
    mode: "cache-first",
    label: "顧客",
    summary:
      "顧客(取引先)マスタ。案件・請求書・顧客支店・顧客担当者はこの顧客 ID に紐づく。名前から顧客 ID を探すのに使う",
    goFile: "api_clients.go",
    responseGroups: RESPONSE_GROUPS_COMMON,
    fields: [
      nameCont("顧客", true),
      str("name_disp_cont", "nameDispCont", "顧客の表示名(略称)で絞り込む(部分一致)", true),
      str(
        "invoice_system_number_eq",
        "invoiceSystemNumberEq",
        "適格請求書発行事業者番号(インボイス登録番号)が一致する顧客",
        true,
      ),
      str(
        "custom_no_eq",
        "customNoEq",
        "顧客管理番号(custom_no)が一致する顧客。サーバー側の指定なので、指定すると BOARD の API を直接呼ぶ(custom_no は既定の response_group=small に含まれず、キャッシュでは絞らない)",
      ),
      strs("tags", "tags", "タグで絞り込む(複数指定可)"),
      responseGroup(RESPONSE_GROUPS_COMMON, "large は get 専用の項目も含める。"),
    ],
  },
  {
    resource: "client_branches",
    singular: "client_branch",
    mode: "nonzero-api",
    label: "顧客支店",
    summary: "顧客の支店・拠点。顧客 ID(client_id)で顧客に紐づく",
    goFile: "api_client_branches.go",
    fields: [int("client_id", "clientIdEq", "顧客 ID で絞り込む(完全一致)")],
  },
  {
    resource: "contacts",
    singular: "contact",
    mode: "nonzero-api",
    label: "顧客担当者",
    summary: "顧客の担当者。顧客 ID(client_id)で顧客に紐づく",
    goFile: "api_contacts.go",
    fields: [int("client_id", "clientIdEq", "顧客 ID で絞り込む(完全一致)")],
  },
  {
    resource: "projects",
    singular: "project",
    mode: "cache-first",
    label: "案件",
    summary:
      "案件。見積書・受注書・納品書・請求書・領収書(書類)と案件原価はこの案件に紐づく。顧客 ID(client_id)で顧客に紐づく",
    goFile: "api_projects.go",
    responseGroups: RESPONSE_GROUPS_PROJECTS_LIST,
    getResponseGroups: { values: RESPONSE_GROUPS_PROJECTS_GET, cacheGroup: "small" },
    fields: [
      nameCont("案件", true),
      int("client_id", "clientIdEq", "顧客 ID で絞り込む(完全一致)", true),
      str("client_name_cont", "clientNameCont", "顧客名で絞り込む(部分一致)", true),
      ints(
        "order_status_in",
        "orderStatusIn",
        "受注ステータスで絞り込む(いずれかに一致): 1=見積中(高) / 2=見積中(中) / 3=見積中(低) / 4=受注確定 / 5=受注済 / 8=見積中(除)",
      ),
      ints(
        "delivery_status_in",
        "deliveryStatusIn",
        "納品ステータスで絞り込む(いずれかに一致): 1=未着手 / 2=着手中 / 3=納品済 / 4=検収済",
      ),
      str("project_no_eq", "projectNoEq", "案件番号が一致する案件", true),
      str("management_no_eq", "managementNoEq", "管理番号が一致する案件", true),
      str("delivery_date_gteq", "deliveryDateGteq", "納品日がこれ以降(`YYYY-MM-DD`)"),
      str("delivery_date_lteq", "deliveryDateLteq", "納品日がこれ以前(`YYYY-MM-DD`)"),
      str("invoice_date_gteq", "invoiceDateGteq", "請求日がこれ以降(`YYYY-MM-DD`)"),
      str("invoice_date_lteq", "invoiceDateLteq", "請求日がこれ以前(`YYYY-MM-DD`)"),
      ints(
        "invoice_timing_kbn_in",
        "invoiceTimingKbnIn",
        "請求タイミングで絞り込む(いずれかに一致): 1=一括請求 / 2=定期請求",
      ),
      strs("tags", "tags", "タグで絞り込む(複数指定可)"),
      responseGroup(
        RESPONSE_GROUPS_PROJECTS_LIST,
        "estimate / order / delivery / invoice / receipt は該当書類を埋め込み、all は全部を含める。",
      ),
      str("created_at_gteq", "createdAtGteq", "作成日時がこれ以降(`YYYY-MM-DD HH:MM:SS`)"),
      str("created_at_lteq", "createdAtLteq", "作成日時がこれ以前(`YYYY-MM-DD HH:MM:SS`)"),
      bool(
        "include_lost_flg",
        "includeLostFlg",
        "未指定なら失注した案件も含める(このサーバーの既定。BOARD の既定とは逆)。false で除外する(include_lost_flg=0 を送る)。失注かどうかは応答の order_status_name で分かる",
      ),
    ],
  },
  {
    resource: "project_costs",
    singular: "project_cost",
    mode: "nonzero-api",
    label: "案件原価",
    summary: "案件の原価(外注費・仕入など)。案件 ID(project_id)で案件に紐づく",
    goFile: "api_project_costs.go",
    fields: [int("project_id", "projectIdEq", "案件 ID で絞り込む(完全一致)")],
  },
  {
    resource: "estimates",
    singular: "estimate",
    mode: "document",
    label: "見積書",
    summary:
      "見積書(書類)。一覧 API はないので、案件(get_project の response_group)から書類 ID を辿って取る",
    goFile: "api_estimates.go",
    fields: [],
  },
  {
    resource: "invoices",
    singular: "invoice",
    mode: "cache-first",
    label: "請求書",
    summary:
      "請求書。案件(project_id)と顧客(client の入れ子。顧客 ID は client.id)に紐づく。請求ステータスは数値コード(invoice_status)と名称(invoice_status_name)で返る",
    goFile: "api_invoices.go",
    responseGroups: RESPONSE_GROUPS_COMMON,
    fields: [
      int(
        "project_client_id_eq",
        "projectClientIdEq",
        "顧客 ID で絞り込む(完全一致。請求書の client.id)",
        true,
      ),
      ints(
        "invoice_status_in",
        "invoiceStatusIn",
        "請求ステータスで絞り込む(いずれかに一致): 1=未請求 / 4=請求OK / 2=請求済 / 5=一部入金済 / 3=入金済 / 9=回収不能",
        true,
      ),
      int(
        "project_project_no_eq",
        "projectProjectNoEq",
        "案件番号で絞り込む(完全一致。整数。請求書の project_no)",
        true,
      ),
      str(
        "project_management_no_eq",
        "projectManagementNoEq",
        "案件の管理番号で絞り込む(完全一致。請求書の management_no)",
        true,
      ),
      str(
        "project_name_cont",
        "projectNameCont",
        "案件名で絞り込む(部分一致。請求書の name は案件名)",
        true,
      ),
      str("invoice_date_gteq", "invoiceDateGteq", "請求日がこれ以降(`YYYY-MM-DD`)"),
      str("invoice_date_lteq", "invoiceDateLteq", "請求日がこれ以前(`YYYY-MM-DD`)"),
      str(
        "invoice_payment_limit_date_gteq",
        "invoicePaymentLimitDateGteq",
        "支払期限がこれ以降(`YYYY-MM-DD`)",
      ),
      str(
        "invoice_payment_limit_date_lteq",
        "invoicePaymentLimitDateLteq",
        "支払期限がこれ以前(`YYYY-MM-DD`)",
      ),
      responseGroup(RESPONSE_GROUPS_COMMON, ""),
    ],
  },
  {
    resource: "orders",
    singular: "order",
    mode: "document",
    label: "受注書",
    summary:
      "受注書(書類)。一覧 API はないので、案件(get_project の response_group)から書類 ID を辿って取る",
    goFile: "api_orders.go",
    fields: [],
  },
  {
    resource: "deliveries",
    singular: "delivery",
    mode: "document",
    label: "納品書",
    summary:
      "納品書(書類)。一覧 API はないので、案件(get_project の response_group)から書類 ID を辿って取る",
    goFile: "api_deliveries.go",
    fields: [],
  },
  {
    resource: "receipts",
    singular: "receipt",
    mode: "document",
    label: "領収書",
    summary:
      "領収書(書類)。一覧 API はないので、案件(get_project の response_group)から書類 ID を辿って取る",
    goFile: "api_receipts.go",
    fields: [],
  },
  {
    resource: "vendors",
    singular: "vendor",
    mode: "cache-first",
    label: "仕入先",
    summary:
      "仕入先(支払先)マスタ。仕入先支店・仕入先担当者・発注書・支払いはこの仕入先 ID に紐づく",
    goFile: "api_vendors.go",
    fields: [nameCont("仕入先", true)],
  },
  {
    resource: "vendor_branches",
    singular: "vendor_branch",
    mode: "nonzero-api",
    label: "仕入先支店",
    summary: "仕入先の支店・拠点。仕入先 ID(payee_id_eq)で仕入先に紐づく",
    goFile: "api_vendor_branches.go",
    fields: [int("payee_id_eq", "payeeIdEq", "仕入先 ID で絞り込む(完全一致)")],
  },
  {
    resource: "vendor_contacts",
    singular: "vendor_contact",
    mode: "nonzero-api",
    label: "仕入先担当者",
    summary: "仕入先の担当者。仕入先 ID(payee_id_eq)で仕入先に紐づく",
    goFile: "api_vendor_contacts.go",
    fields: [int("payee_id_eq", "payeeIdEq", "仕入先 ID で絞り込む(完全一致)")],
  },
  {
    resource: "purchase_orders",
    singular: "purchase_order",
    mode: "cache-first",
    label: "発注書",
    summary:
      "発注書(仕入側。BOARD の API では expenditures)。支払先(payee の入れ子。仕入先 ID は payee.id)と案件(project の入れ子)に紐づく。発注ステータスは expenditure_status、進捗状況は delivery_status(数値コードと _name の名称)で返る",
    goFile: "api_purchase_orders.go",
    responseGroups: RESPONSE_GROUPS_PURCHASE_ORDERS,
    getResponseGroups: {
      values: RESPONSE_GROUPS_PURCHASE_ORDERS_GET,
      cacheGroup: PURCHASE_ORDER_DEFAULT_RESPONSE_GROUP,
    },
    fields: [
      int(
        "payee_id_eq",
        "payeeIdEq",
        "支払先(仕入先)ID で絞り込む(完全一致。発注書の payee.id)",
        true,
      ),
      int(
        "project_id_eq",
        "projectIdEq",
        "案件 ID で絞り込む(完全一致)。サーバー側の指定なので、指定すると BOARD の API を直接呼ぶ(キャッシュでは絞らない)",
      ),
      ints(
        "expenditure_status_in",
        "expenditureStatusIn",
        "発注ステータスで絞り込む(いずれかに一致): 1=見積中(高) / 2=見積中(中) / 3=見積中(低) / 8=見積中(除) / 4=発注確定 / 5=発注済 / 9=見送り",
        true,
      ),
      ints(
        "delivery_status_in",
        "deliveryStatusIn",
        "進捗状況で絞り込む(いずれかに一致): 1=未着手 / 2=着手中 / 3=納品済 / 4=検収済",
        true,
      ),
      responseGroup(
        RESPONSE_GROUPS_PURCHASE_ORDERS,
        "未指定なら medium を送る(案件 project を含む)。small は project を含まない。",
      ),
    ],
  },
  {
    resource: "payments",
    singular: "payment",
    mode: "cache-first",
    label: "支払い",
    summary:
      "仕入先への支払い(BOARD の API では expenditure_payments)。支払先(payee の入れ子。仕入先 ID は payee.id)と発注(expenditure_id / expenditure_no)に紐づく。支払ステータスは payment_status(数値コードと payment_status_name の名称)で返る",
    goFile: "api_payments.go",
    responseGroups: RESPONSE_GROUPS_COMMON,
    fields: [
      int(
        "expenditure_payee_id_eq",
        "expenditurePayeeIdEq",
        "支払先(仕入先)ID で絞り込む(完全一致。支払の payee.id)",
        true,
      ),
      ints(
        "payment_status_in",
        "paymentStatusIn",
        "支払ステータスで絞り込む(いずれかに一致): 1=請求書未受領 / 2=請求書受領済 / 4=振込予約済 / 3=支払済",
        true,
      ),
      int(
        "expenditure_expenditure_no_eq",
        "expenditureExpenditureNoEq",
        "発注 No で絞り込む(完全一致)。発注の ID ではなく、発注書の expenditure_no(支払の expenditure_no と同じ値)",
        true,
      ),
      responseGroup(RESPONSE_GROUPS_COMMON, ""),
    ],
  },
  {
    resource: "users",
    singular: "user",
    mode: "cache-first",
    label: "ユーザー",
    summary:
      "BOARD のユーザー(社内の担当者)マスタ。姓名は last_name / first_name に分かれていて、name という項目は無い",
    goFile: "api_users.go",
    fields: [
      str(
        "name_cont",
        "nameCont",
        "ユーザーの姓・名・姓名(連結、または空白区切り)で絞り込む(部分一致。キャッシュ専用: BOARD の API に名前の絞り込みは無いので、未同期なら絞り込みなしで全件を取ってからこちらで絞る)",
        true,
      ),
      str(
        "email_cont",
        "emailCont",
        "メールアドレスで絞り込む(部分一致。キャッシュ専用: name_cont と同じ扱い)",
        true,
      ),
    ],
  },
  {
    resource: "groups",
    singular: "group",
    mode: "nonzero-api",
    label: "グループ",
    summary: "ユーザーのグループ(部署など)マスタ",
    goFile: "api_groups.go",
    fields: [nameCont("グループ")],
  },
  {
    resource: "payment_terms",
    singular: "payment_term",
    mode: "nonzero-api",
    label: "支払条件",
    summary: "支払条件(締め日・支払日の定義)マスタ",
    goFile: "api_payment_terms.go",
    fields: [nameCont("支払条件")],
  },
  {
    resource: "project_types",
    singular: "project_type",
    mode: "nonzero-api",
    label: "案件種別",
    summary: "案件種別マスタ",
    goFile: "api_project_types.go",
    fields: [nameCont("案件種別"), kbn("project_type_kbn_eq", "projectTypeKbnEq", "案件区分")],
  },
  {
    resource: "purchase_types",
    singular: "purchase_type",
    mode: "nonzero-api",
    label: "購買種別",
    summary: "購買(仕入)種別マスタ",
    goFile: "api_purchase_types.go",
    fields: [
      nameCont("購買種別"),
      kbn("expenditure_type_kbn_eq", "expenditureTypeKbnEq", "発注区分"),
    ],
  },
  {
    resource: "accounting_types",
    singular: "accounting_type",
    mode: "nonzero-api",
    label: "勘定科目種別",
    summary: "勘定科目の種別マスタ",
    goFile: "api_accounting_types.go",
    fields: [
      nameCont("勘定科目種別"),
      kbn("accounting_type_kbn_eq", "accountingTypeKbnEq", "会計区分"),
    ],
  },
  {
    resource: "document_send_channels",
    singular: "document_send_channel",
    mode: "nonzero-api",
    label: "書類送付方法",
    summary: "書類の送付方法(メール・郵送など)マスタ",
    goFile: "api_document_send_channels.go",
    fields: [nameCont("書類送付方法")],
  },
];
