import type { BoardClient } from "../client";
import type { CallContext } from "../context";
import { getResource, type ItemResult } from "./common";
import type { DocumentDetailEntity } from "./refs";

/** 受注書(注文書)。GET /v1/documents/orders/{documentId} の応答(Go の OrderEntity)。一覧 API はない。 */
export interface OrderEntity {
  id: number;
  message: string | null;
  total: string;
  tax: string;
  tax_withholding: string;
  seal_approval_status: number;
  document_amount_disp_kbn: number;
  blank_date_flg: number;
  lock_flg: number;
  delivery_place: string | null;
  details: DocumentDetailEntity[];
  disp_order_date: string | null;
  disp_order_receive_date: string | null;
  [key: string]: unknown;
}

export function getOrder(
  client: BoardClient,
  documentId: number,
  ctx?: CallContext,
): Promise<ItemResult<OrderEntity>> {
  return getResource<OrderEntity>(
    client,
    "/v1/documents/orders",
    documentId,
    undefined,
    "documentId",
    ctx,
  );
}
