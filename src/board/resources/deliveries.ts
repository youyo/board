import type { BoardClient } from "../client";
import type { CallContext } from "../context";
import { getResource, type ItemResult } from "./common";
import type { DocumentDetailEntity } from "./refs";

/** 納品書。GET /v1/documents/deliveries/{documentId} の応答(Go の DeliveryEntity)。一覧 API はない。 */
export interface DeliveryEntity {
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
  delivery_date: string;
  disp_delivery_date: string | null;
  disp_delivery_receive_date: string | null;
  [key: string]: unknown;
}

export function getDelivery(
  client: BoardClient,
  documentId: number,
  ctx?: CallContext,
): Promise<ItemResult<DeliveryEntity>> {
  return getResource<DeliveryEntity>(
    client,
    "/v1/documents/deliveries",
    documentId,
    undefined,
    "documentId",
    ctx,
  );
}
