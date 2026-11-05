import type { BoardClient } from "../client";
import {
  baseQuery,
  type CommonListOptions,
  getResource,
  type ItemResult,
  type ListPaging,
  type ListResult,
  listResource,
} from "./common";

/** 書類送付方法。GET /v1/document_send_channels の応答 1 要素。 */
export interface DocumentSendChannelEntity {
  id: number;
  name: string;
  memo: string;
  /** ISO 8601 */
  updated_at: string;
  /** ISO 8601 */
  created_at: string;
  [key: string]: unknown;
}

/** GET /v1/document_send_channels のクエリ(Ransack)。未指定の項目は送らない。 */
export interface DocumentSendChannelListOptions extends CommonListOptions {
  /** 書類送付方法名の部分一致(name_cont) */
  nameCont?: string;
}

export function buildDocumentSendChannelsQuery(
  opts: DocumentSendChannelListOptions,
  page: number,
  perPage: number,
): string {
  return baseQuery(opts, page, perPage).strCont("name", opts.nameCont).encode();
}

export function listDocumentSendChannels(
  client: BoardClient,
  opts: DocumentSendChannelListOptions = {},
  paging?: ListPaging,
): Promise<ListResult<DocumentSendChannelEntity>> {
  return listResource<DocumentSendChannelEntity, DocumentSendChannelListOptions>(
    client,
    "/v1/document_send_channels",
    buildDocumentSendChannelsQuery,
    opts,
    paging,
  );
}

export function getDocumentSendChannel(
  client: BoardClient,
  id: number,
): Promise<ItemResult<DocumentSendChannelEntity>> {
  return getResource<DocumentSendChannelEntity>(client, "/v1/document_send_channels", id);
}
