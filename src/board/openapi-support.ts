// 自動生成: mise run openapi:fields -- <board_openapi.json> で作る。手で編集しない(scripts/openapi-fields/)。
// 元: BOARD 公式の OpenAPI 仕様 v1.9.0(https://developers.the-board.jp/doc/board_openapi.json)。仕様の JSON 自体はリポジトリに置かない。

export interface ListFilterSupport {
  /** 一覧のクエリに updated_at_gteq があるか */
  updatedAtGteq: boolean;
  /** 一覧のクエリに updated_at_lteq があるか */
  updatedAtLteq: boolean;
  /** 一覧のクエリに include_archive_flg があるか */
  includeArchiveFlg: boolean;
}

/** リソースごとの、一覧が受ける共通フィルタ。仕様に無いものは API に送らず、ツールの引数にも出さない。 */
export const LIST_FILTER_SUPPORT: Readonly<Record<string, ListFilterSupport>> = {
  clients: { updatedAtGteq: true, updatedAtLteq: true, includeArchiveFlg: true },
  client_branches: { updatedAtGteq: true, updatedAtLteq: true, includeArchiveFlg: true },
  contacts: { updatedAtGteq: true, updatedAtLteq: true, includeArchiveFlg: true },
  projects: { updatedAtGteq: true, updatedAtLteq: true, includeArchiveFlg: true },
  project_costs: { updatedAtGteq: false, updatedAtLteq: false, includeArchiveFlg: false },
  invoices: { updatedAtGteq: true, updatedAtLteq: true, includeArchiveFlg: false },
  vendors: { updatedAtGteq: true, updatedAtLteq: true, includeArchiveFlg: true },
  vendor_branches: { updatedAtGteq: true, updatedAtLteq: true, includeArchiveFlg: true },
  vendor_contacts: { updatedAtGteq: true, updatedAtLteq: true, includeArchiveFlg: true },
  purchase_orders: { updatedAtGteq: true, updatedAtLteq: true, includeArchiveFlg: true },
  payments: { updatedAtGteq: true, updatedAtLteq: true, includeArchiveFlg: false },
  users: { updatedAtGteq: false, updatedAtLteq: false, includeArchiveFlg: false },
  groups: { updatedAtGteq: false, updatedAtLteq: false, includeArchiveFlg: true },
  payment_terms: { updatedAtGteq: false, updatedAtLteq: false, includeArchiveFlg: true },
  project_types: { updatedAtGteq: false, updatedAtLteq: false, includeArchiveFlg: true },
  purchase_types: { updatedAtGteq: false, updatedAtLteq: false, includeArchiveFlg: true },
  accounting_types: { updatedAtGteq: false, updatedAtLteq: false, includeArchiveFlg: true },
  document_send_channels: { updatedAtGteq: false, updatedAtLteq: false, includeArchiveFlg: true },
};
