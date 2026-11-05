import type { BoardClient } from "../client";
import type { CallContext } from "../context";
import { getResource, type ItemResult } from "./common";
import type { DocumentDetailEntity } from "./refs";

/** 見積書。GET /v1/documents/estimates/{documentId} の応答(Go の EstimateEntity)。一覧 API はない。 */
export interface EstimateEntity {
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
  valid_period: string;
  [key: string]: unknown;
}

export function getEstimate(
  client: BoardClient,
  documentId: number,
  ctx?: CallContext,
): Promise<ItemResult<EstimateEntity>> {
  return getResource<EstimateEntity>(
    client,
    "/v1/documents/estimates",
    documentId,
    undefined,
    "documentId",
    ctx,
  );
}
