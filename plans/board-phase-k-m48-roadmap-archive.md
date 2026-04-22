# M48: 旧ロードマップ archive 化

## Meta
| 項目 | 値 |
|------|---|
| ステータス | 進行中 🔄 |
| 作成日 | 2026-04-22 |
| 目的 | 完走済みの旧ロードマップ（board-roadmap.md および関連 M*.md）を `plans/archive/` へ移設し、現行の計画ファイルと混在しないように整理する |
| 前提 | M43-M47 完了 |
| 次のマイルストーン | なし（Phase K 完走 → v0.4.0 リリース準備） |

## 背景
- `plans/board-roadmap.md` は board プロジェクト初期の機能実装ロードマップ（M01-M36 相当）で、全マイルストーン完走済み
- `plans/board-m*.md` 系（M01〜M36）も同様に完走済み
- `plans/board-compliance-roadmap.md` および `board-compliance-m*.md` は 42 M 完走済だが、Pending Re-verification（vendor データ投入後）が残っているため **archive 対象外**
- Phase K ロードマップ（`board-phase-k-*`）は進行中のため archive 対象外
- 歴史的記録として価値があるため削除はしない

## 実施内容

### 1. archive ディレクトリ作成
- `plans/archive/` を新設（空ディレクトリの維持目的で `.gitkeep` は不要。ファイルが入るため）

### 2. 移動対象ファイル
完走済みの初期ロードマップ群:
- `plans/board-roadmap.md` → `plans/archive/board-roadmap.md`
- `plans/board-m01-project-init.md` 〜 `plans/board-m36-mcp-tools-schema.md` の全 M*.md → `plans/archive/`

移動しないもの:
- `plans/board-compliance-*.md`（Pending 案件あり）
- `plans/board-phase-k-*.md`（進行中）
- `plans/tender-squishing-dusk.md` や code-name 付き plans（親計画／派生。方針未確定のため一旦残置）

### 3. archive ヘッダ追記
移動後の `plans/archive/board-roadmap.md` 先頭に以下を追記:
```markdown
> **Archived: 2026-04-22** — このロードマップは全マイルストーン完走済みです。
> 現行ロードマップは `plans/board-phase-k-roadmap.md`、準拠検証は `plans/board-compliance-roadmap.md` を参照してください。
```

個別 `board-m*.md` にはヘッダは追記しない（量が多く、root roadmap から辿れれば十分）。

### 4. 参照更新
以下のファイル内リンクを `plans/board-roadmap.md` → `plans/archive/board-roadmap.md` に更新:
- `CLAUDE.md`（プロジェクトルート）
- `plans/board-phase-k-roadmap.md`
- `plans/board-compliance-roadmap.md`
- その他 grep で検出された参照元

### 5. 検証
- `git mv` でファイル履歴を維持
- `grep -r 'plans/board-roadmap.md'` で参照残を確認し 0 件にする
- `grep -r 'board-m[0-9]' CLAUDE.md docs/ plans/board-phase-k-*.md plans/board-compliance-*.md` で壊れたリンクがないか確認

## 完了条件
- [ ] `plans/archive/` 作成済み
- [ ] `board-roadmap.md` と 旧 `board-m*.md` 系が `plans/archive/` 配下に移動済み
- [ ] `board-roadmap.md` 先頭に Archive ヘッダ追記済み
- [ ] CLAUDE.md / phase-k-roadmap / compliance-roadmap の参照更新済み
- [ ] 残存参照なし（`grep` で確認）
- [ ] コミット完了

## リスク
- 低: 計画ファイルの移動のみでコード影響なし
- docs/ や README は移動ファイルを参照していない（事前確認）
