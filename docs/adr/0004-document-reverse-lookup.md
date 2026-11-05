# ADR 0004: 書類 ID から案件 ID への逆引き

- 状態: 採用(M05、2026-11-05。M06 で文書化)
- 関連: ADR 0002(D1 キャッシュ)、ADR 0003(締め切り)、`plans/board-workers-m05-find-tools.md`

## 背景

書類(見積・受注・納品・領収)のエンティティは案件 ID を持たない(Go の PoC で確認済み)。`find_estimates(id=書類 ID)` などで案件・顧客を付けるには、書類 ID から案件 ID を逆引きする必要がある。
Go は miss のたびに projects を `response_group=<種別>` で全件走査して表を作った。実測は 2,425 件・25 ページで 16〜32 秒(10 秒を超えたら `ProjectID=0` に縮退)。
Workers は 1 回の呼び出しの締め切りが 30 秒(ADR 0003)で、書類・案件・顧客の取得も同じ枠に入るので、同じ方式では収まらない。

## 決定

**D1 の表(スキーマ v2)+ 再開できるページ単位の走査 + find が案件を取るたびの記録。**

1. 対応は D1 の `document_project_map(document_kind, document_id, project_id, recorded_at)` に残す。**対応は不変**(書類は作られた案件にずっと属する)なので、一度覚えたら引き直さない。全 isolate・全利用者で共有される。
2. find が案件を `response_group=<種別>` 付きで取るたび(`project_id` / `client_name` / `project_name` の経路)、含まれる書類 ID を追加の通信なしで記録する。
3. 表にない書類(id 指定で miss)だけ、ページ単位の走査で補う。1 ページ(100 件)取るごとに、その場で表と進捗(`document_scan_state.next_page`)を**同じ batch で、走査ロックの所有者を条件に**書く。見つけたら止める。次の呼び出しは保存した次のページから続ける。種別ごとに進捗は独立。
4. 走査は締め切りの手前(8 秒を残して)で止め、縮退(`ProjectID=0`)を**応答の `warnings`**(`project_unresolved`、理由 `deadline` / `scan_in_progress` / `api_error` / `not_found`)で伝える。
5. 走査ロックは `document_scan_state` の条件付き UPDATE(stale は 2 分)。取れなければ待たずに `scan_in_progress`。
6. 最後まで走査して見つからなかった結果は 10 分だけ信用する(その間の miss は再走査しない)。

`cache_status` は書類種別ごとの走査状態(`scan_state` / `mapped_rows` / `next_page`)を返す。`cache_clear` は既定で対応表を消さない(不変なので)。`include_document_map: true` で消せる。

## 却下した案

- **projects のキャッシュから D1 上で引く**: refresh が `response_group` を付けずに取るので、キャッシュの案件に書類 ID が入っていない。refresh を変えると全リソースの取得量が増え、M03 の範囲外。
- **Go と同じ全件走査**: 30 秒に収まらない。
- **Durable Objects / Queues で裏の走査**: 決定済み事項(DO は使わない)に反する。
- **`response_group=all` で 1 回の走査にまとめる**: ページごとの重さが未計測。必要なら `DOCUMENT_KIND_INFO.group` を変えて試せる(`mise run test:live` で計測する候補)。

## 帰結

- 表が空のうちは、id 指定の書類が `ProjectID=0` + `deadline` の警告で返りうる。再実行で続きから進み、4 種別とも 1〜2 回で覚え切る見込み(1 ページ 0.7〜1.3 秒の見積もり)。実測は manual-verification。
- 4 種別の全件走査は最大 100 回(BOARD の 1 日 3,000 回の約 3%)で、一度きり。
- ページ番号での再開は、走査の合間に案件が削除・挿入されると取りこぼしうる(案件経路の記録か TTL 後の再走査で拾う)。案件の付け替えは反映されない。
- ロックで守る書き込みは所有者条件つきの batch(ADR 0002)。
