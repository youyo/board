/**
 * find ツールのステータス名(例: `請求済`)→ BOARD の数値コードの変換(issue #27)。
 * コードは `*_status_in[]` に送る値で、キャッシュの絞り込みでも同じ値で一致させる。
 * 知らない名称は argument_error(受け付ける名称の一覧を message に入れる)。
 */
import {
  EXPENDITURE_DELIVERY_STATUSES,
  EXPENDITURE_STATUSES,
  INVOICE_STATUSES,
  PAYMENT_STATUSES,
} from "../board/resources";
import { ArgumentError } from "../errors";

type Table = Readonly<Record<number, string>>;

const namesOf = (t: Table): string[] => Object.values(t);

function codeOf(t: Table, name: string): number | undefined {
  const hit = Object.entries(t).find(([, n]) => n === name);
  return hit ? Number(hit[0]) : undefined;
}

function unknownStatus(name: string, accepted: readonly string[]): ArgumentError {
  return new ArgumentError(
    `unknown status ${JSON.stringify(name)}; accepted names: ${accepted.join(" / ")}`,
  );
}

function unique(codes: number[]): number[] {
  return [...new Set(codes)];
}

function toCodes(names: readonly string[], t: Table): number[] {
  return unique(
    names.map((name) => {
      const code = codeOf(t, name);
      if (code === undefined) throw unknownStatus(name, namesOf(t));
      return code;
    }),
  );
}

/** 請求ステータス(未請求 / 請求OK / 請求済 / 一部入金済 / 入金済 / 回収不能)の名称 → コード。 */
export const invoiceStatusCodes = (names: readonly string[]): number[] =>
  toCodes(names, INVOICE_STATUSES);

/** 支払ステータス(請求書未受領 / 請求書受領済 / 振込予約済 / 支払済)の名称 → コード。 */
export const paymentStatusCodes = (names: readonly string[]): number[] =>
  toCodes(names, PAYMENT_STATUSES);

export interface PurchaseOrderStatusFilter {
  /** 発注ステータスのコード(expenditure_status_in) */
  expenditureStatusIn: number[];
  /** 進捗状況のコード(delivery_status_in) */
  deliveryStatusIn: number[];
}

/**
 * 発注書の名称は、発注ステータスと進捗状況の両方を受ける(名称は重ならない)。
 * 名称ごとに該当するグループへ振り分ける。両方のグループに分かれたときの OR は、呼び出し側が後段で行う。
 */
export function purchaseOrderStatusFilter(names: readonly string[]): PurchaseOrderStatusFilter {
  const expenditureStatusIn: number[] = [];
  const deliveryStatusIn: number[] = [];
  for (const name of names) {
    const e = codeOf(EXPENDITURE_STATUSES, name);
    const d = codeOf(EXPENDITURE_DELIVERY_STATUSES, name);
    if (e !== undefined) expenditureStatusIn.push(e);
    else if (d !== undefined) deliveryStatusIn.push(d);
    else {
      throw unknownStatus(name, [
        ...namesOf(EXPENDITURE_STATUSES),
        ...namesOf(EXPENDITURE_DELIVERY_STATUSES),
      ]);
    }
  }
  return {
    expenditureStatusIn: unique(expenditureStatusIn),
    deliveryStatusIn: unique(deliveryStatusIn),
  };
}
