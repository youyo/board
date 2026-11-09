/** 他のエンティティの中に入れ子で返ってくる参照型(Go の *_ref.go、document_summary.go、document_detail.go)。 */

export interface ClientRef {
  id: number;
  name: string;
  name_disp: string;
  custom_no: string;
  [key: string]: unknown;
}

export interface ClientBranchRef {
  id: number;
  name: string;
  [key: string]: unknown;
}

export interface CompanyBranchRef {
  id: number;
  name: string;
  [key: string]: unknown;
}

export interface ContactRef {
  id: number;
  last_name: string;
  first_name: string;
  [key: string]: unknown;
}

export interface UserRef {
  id: number;
  last_name: string;
  first_name: string;
  [key: string]: unknown;
}

export interface VendorRef {
  id: number;
  name: string;
  name_disp: string;
  custom_no: string;
  [key: string]: unknown;
}

/** 発注書・支払の `project`(medium 以上の response_group で入る)。 */
export interface ProjectRef {
  id: number;
  name: string;
  project_no: number | null;
  management_no: string | null;
  [key: string]: unknown;
}

export interface HubspotRef {
  hubspot_deal_id: string | null;
  [key: string]: unknown;
}

/** 見積・受注・納品・領収の明細(4 種で構造が同じ)。 */
export interface DocumentDetailEntity {
  no: number;
  detail_date: string | null;
  description: string;
  quantity: string | null;
  unit: string | null;
  unit_price: string | null;
  price: string | null;
  tax_rate: string;
  tax_withholding_flg: number;
  tax_included_flg: number;
  reduced_tax_rate_kbn: number;
  section_description: string | null;
  section_subtotal: string | null;
  document_detail_kbn: number;
  document_detail_kbn_name: string;
  deduction_applicable: boolean;
  [key: string]: unknown;
}

/** projects API が response_group 付きで返す書類の要約。種別ごとのフィールドは該当する response_group のときだけ入る。 */
export interface DocumentSummary {
  id: number;
  message: string | null;
  total: string;
  tax: string;
  tax_withholding: string;
  lock_flg: number;
  seal_approval_status: number;
  document_amount_disp_kbn: number;
  blank_date_flg: number;
  delivery_place: string | null;
  details: DocumentDetailEntity[];
  // estimate のみ
  valid_period?: string | null;
  // order のみ
  disp_order_date?: string | null;
  disp_order_receive_date?: string | null;
  // delivery のみ
  delivery_date?: string | null;
  disp_delivery_date?: string | null;
  disp_delivery_receive_date?: string | null;
  // invoice のみ
  invoice_date?: string | null;
  payment_limit_date?: string | null;
  disp_invoice_date?: string | null;
  multi_bank_info_flg?: number | null;
  // receipt のみ
  receipt_date?: string | null;
  disp_receipt_date?: string | null;
  [key: string]: unknown;
}
