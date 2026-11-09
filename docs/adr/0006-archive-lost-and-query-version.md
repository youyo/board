# ADR 0006: アーカイブ・失注を含めたキャッシュと、クエリの版による旧キャッシュの無効化

- 状態: 採用(v1.8.0 / v1.9.0、2026-11-10)
- 関連: ADR 0002(D1 キャッシュ)、ADR 0004(逆引き)、issue #35・#37・#40

## 背景

BOARD の一覧 API は、`include_archive_flg`(projects は `include_lost_flg` も)を送らないと、アーカイブ済み・失注のレコードを除外する(既定は 0)。
v1.7.0 までは、refresh(full / delta)・API 経路・書類の逆引き走査のどれもこれらを送っていなかった。そのため、キャッシュと名前解決からこれらのレコードが抜けていた。
実機の projects は、指定なしで約 2,900 件、両方を 1 にすると約 5,900 件だった。`refresh_state` は `complete` になるので、利用者は欠けに気づけない。
同名の案件の片方がアーカイブ済みだと、曖昧エラーにならず、残った 1 件に黙って決まる。全部がアーカイブ済みなら `not_found` になる(issue #35)。

もう 1 つの事情として、一覧の既定の `response_group=small` には `archive_flg` が含まれない(projects / clients / payees / expenditures は `large` だけ)。失注は `order_status=9` で分かる。
単体取得(get)で入った行と refresh で入った行で、項目数が違う問題もあった(issue #37)。

## 決定

1. **refresh と逆引き走査は、すべてを取ってキャッシュに入れる。** `include_archive_flg` を受けるリソースすべてで `include_archive_flg=1` を送る(projects は `include_lost_flg=1` も)。delta でも送る。送らないと、前回の同期のあとにアーカイブ・失注になったレコードの更新を取りこぼす。
   prune の件数照合は、同じ条件の `X-Total-Count` と比べることになる。
2. **`list_*` の既定は、アーカイブ・失注込みにする(BOARD の既定とは逆)。** API 経路(`viaApi`)でも、未指定なら include を補い、キャッシュ経路と同じ集合を返す。明示の `false` は `=0` を送る(API 経路になる)。find 層(名前の解決)もこれで揃う。
3. **旧コードのキャッシュは消さず、版で見分ける。** スキーマ v4 で、`sync_state` に `query_version` / `full_query_version` / `delta_query_version`、`document_scan_state` に `query_version` を足した(定数は `src/cache/sync-state.ts` の `CACHE_QUERY_VERSION`。現在 1)。
   - 版を要求するのは、`include_archive_flg` を受ける 14 リソースだけ(clients / client_branches / contacts / projects / vendors / vendor_branches / vendor_contacts / purchase_orders / groups / payment_terms / project_types / purchase_types / accounting_types / document_send_channels)。invoices / payments / users / project_costs はクエリが変わらないので取り直さない。
   - 版が違う(NULL を含む)同期は未同期として扱う(応答の `cache` に `cached_at` を出さない)。進捗は再開しない。逆引きの走査はやり直す。最初の `refresh=delta` は full になる。
   - 版の書き込みは、既存の owner 条件つき batch(ADR 0002)の中で行う。
   - full の完了で版を進めるのは、prune の件数照合が通ったときだけ。取りこぼした full を「揃った」と認めない。
4. **キャッシュに書く単体取得の項目を、refresh の行に揃える(v1.9.0、issue #37)。** キャッシュに書く get は `response_group` を明示して送る。値は一覧の refresh と同じにする(`getProject` は small、`getPurchaseOrder` は一覧の既定と同じ medium。発注書の一覧は medium が既定で、案件 `project` を含むため)。項目が要るときは、`get_project` / `get_purchase_order` の `response_group` を使う(API 直行で、キャッシュに書かない)。
   clients / vendors(`GET /clients/{id}` / `GET /payees/{id}`)は、OpenAPI v1.9.0 の単体取得に `response_group` が無い。仕様に無いものは送らない方針(issue #28)なので、送らずにおく。この 2 つは、get で入った行が refresh の small の行より項目が多いことがある(実機で未確認。manual-verification の V17)。
5. **prune を見送った full は `must_full_resync` を立てる(v1.9.0、issue #40)。** 次の `refresh=delta` は、前回の full の完了(`last_full_synced_at`)から 6 時間(`MUST_FULL_RESYNC_MIN_INTERVAL_MS`)以上たっているとき full として動く(完了時刻が無い・読めないときは切り替える。6 時間未満は delta のまま、`refresh=full` は常に full)。切り替えで始めた full が partial で終われば、次の `refresh=delta` も full の経路になり保存した進捗から再開する。カーソルは full の完了で従来どおり進む(次が full なので取りこぼしには影響しない)。読み取りは続ける(`hasSynced` は変えない)。ADR 0002 の「`full_prune_skipped` を立てて次の full で取り直す」を、自動で full に切り替わる形へ更新した。

## 却下した案

- **`response_group=large` で `archive_flg` をキャッシュに持ち、キャッシュ側で BOARD の既定の除外を再現する**: issue #35 の再現手順が「キャッシュからはアーカイブ・失注込みが返るべき」としているのと逆になる。ペイロードが数倍になり、get の経路の行と形が揃わない(#37)。`archive_flg` が large だけの 4 リソースの特別扱いにもなる。
- **移行で `sync_state` の `last_synced_at` / カーソル / 進捗を消す**: 実行中の旧 refresh(デプロイの切り替わりの間は旧 isolate が動いている)が、あとから書き戻して「同期済み」を復活させうる。版の列なら旧コードは書かないので、新コードからは未同期のままになる。
- **間隔なしで毎回 full に切り替える案**: X-Total-Count が恒常的に多いと、`refresh=delta` のたびに全ページの full(projects は 60 ページ程度)になり、日次 3,000 回の rate limit を使い切って全ツールが rate_limited になる。
- **版のチェックを全リソースに掛ける**: クエリの変わらない invoices などまで、full のやり直しになる。
- **条件なしの `list_*` を、未同期なら API に回す**: 全件のページングで締め切り(ADR 0003)に当たる。旧キャッシュを返すことにして、応答の `cache` に `cached_at` が出ないことで、未取得と判別できるようにした。
- **find 層の各検索に、個別にフラグを足す**: `viaApi` の 1 箇所で足りる。

## 帰結

- 下流は v1.8.0 を取り込んだら、14 リソースで `refresh=full` を `complete` まで回す必要がある(runbook の『v1.8.0 へ上げたとき』)。それまでの名前解決は API 経由になる。
- `list_*` の結果(small)からは、どれがアーカイブ済みか判別できない(`archive_flg` が無い)。失注は `order_status_name` で分かる。除外したいときは API 経路で `include_archive_flg=false` を指定する。
- BOARD 側の事情で `X-Total-Count` が常に取得件数より多いと、full への自動の切り替えが 6 時間に 1 回になる(その間の delta は差分だけを取る。欠けたまま同期済みにするよりよく、rate limit も守る)。full を繰り返しても `full_prune_skipped` が true のままなら、総件数の数え方を疑う。
- 実機で未確認の前提は `docs/manual-verification.md` の V16: フラグの組み合わせの意味、アーカイブ操作で `updated_at` が動くか、`row_count` と `total_count` の一致。
- クエリの内容を変えたら `CACHE_QUERY_VERSION` を上げる。上げると、版を要求する 14 リソースは再び未同期になり、full のやり直しが要る。
