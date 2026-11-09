import type { BoardClient } from "../client";
import type { CallContext } from "../context";
import { getResource, type ItemResult } from "./common";
import type { DocumentDetailEntity } from "./refs";

/** 領収書。GET /v1/documents/receipts/{documentId} の応答(Go の ReceiptEntity)。一覧 API はない。 */
export interface ReceiptEntity {
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
  receipt_date: string;
  disp_receipt_date: string | null;
}

export function getReceipt(
  client: BoardClient,
  documentId: number,
  ctx?: CallContext,
): Promise<ItemResult<ReceiptEntity>> {
  return getResource<ReceiptEntity>(
    client,
    "/v1/documents/receipts",
    documentId,
    undefined,
    "documentId",
    ctx,
  );
}
