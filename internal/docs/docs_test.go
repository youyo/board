package docs

import (
	"errors"
	"io/fs"
	"sort"
	"strings"
	"testing"
)

// D-1: embed FS のルートが assets ディレクトリで成立しているか
func TestFS_NonEmpty(t *testing.T) {
	fsys := FS()
	info, err := fs.Stat(fsys, "assets")
	if err != nil {
		t.Fatalf("fs.Stat(assets) err: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("assets should be a directory")
	}
}

// D-2: List が 6 件返し、各 Size > 0 であること
func TestList(t *testing.T) {
	entries, err := List()
	if err != nil {
		t.Fatalf("List err: %v", err)
	}
	wantPaths := []string{
		"README.md",
		"adr/ADR-001-find-layer.md",
		"api-reference.md",
		"guides/getting-started.md",
		"guides/mcp-server.md",
		"installation.md",
		"installation_ja.md",
		"migration/v0.7.0.md",
	}
	if len(entries) != len(wantPaths) {
		t.Fatalf("len(entries)=%d, want %d (%v)", len(entries), len(wantPaths), entries)
	}
	got := make([]string, 0, len(entries))
	for _, e := range entries {
		got = append(got, e.File)
		if e.Size <= 0 {
			t.Errorf("entry %q size <= 0", e.File)
		}
	}
	for _, w := range wantPaths {
		found := false
		for _, g := range got {
			if g == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected file %q not in list: %v", w, got)
		}
	}
}

// D-3: List の出力が File 昇順でソート済み
func TestList_SortedByPath(t *testing.T) {
	entries, err := List()
	if err != nil {
		t.Fatalf("List err: %v", err)
	}
	paths := make([]string, len(entries))
	for i, e := range entries {
		paths[i] = e.File
	}
	if !sort.StringsAreSorted(paths) {
		t.Fatalf("entries not sorted: %v", paths)
	}
}

// D-4: README の Read
func TestRead_README(t *testing.T) {
	b, err := Read("README.md")
	if err != nil {
		t.Fatalf("Read README.md err: %v", err)
	}
	if len(b) <= 100 {
		t.Fatalf("README content too short: %d bytes", len(b))
	}
	if !strings.Contains(string(b), "# board") {
		t.Fatalf("README should start with '# board', got: %s...", string(b[:80]))
	}
}

// D-E1: 存在しないファイルは fs.ErrNotExist
func TestRead_NotFound(t *testing.T) {
	_, err := Read("nonexistent.md")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected fs.ErrNotExist, got: %v", err)
	}
}

// D-5: clients セクションを抽出、次節を含まない
func TestExtractSection_Clients(t *testing.T) {
	s, err := ExtractSection("clients")
	if err != nil {
		t.Fatalf("ExtractSection(clients) err: %v", err)
	}
	if !strings.Contains(s, "clients — 顧客マスタ") {
		t.Errorf("result should contain 'clients — 顧客マスタ': %s", s)
	}
	if !strings.Contains(s, "GET /v1/clients") {
		t.Errorf("result should contain BOARD API endpoint 'GET /v1/clients'")
	}
	if strings.Contains(s, "client_branches — 顧客支店") {
		t.Errorf("result should NOT contain next section 'client_branches': %s", s)
	}
}

// D-5b: prefix 誤マッチ防止（"client" は "client_branches" の prefix だが無効）
func TestExtractSection_NoPrefixMatch(t *testing.T) {
	_, err := ExtractSection("client")
	if err == nil {
		t.Fatalf("expected ErrSectionNotFound, got nil")
	}
	if !errors.Is(err, ErrSectionNotFound) {
		t.Fatalf("expected ErrSectionNotFound, got: %v", err)
	}
}

// D-6: project_costs を抽出、次の estimates を含まない
func TestExtractSection_ProjectCosts(t *testing.T) {
	s, err := ExtractSection("project_costs")
	if err != nil {
		t.Fatalf("ExtractSection(project_costs) err: %v", err)
	}
	if !strings.Contains(s, "project_costs — 案件原価") {
		t.Errorf("should contain 'project_costs — 案件原価'")
	}
	if strings.Contains(s, "estimates — 見積書") {
		t.Errorf("should NOT contain 'estimates — 見積書': %s", s)
	}
}

// D-6b: 最終セクションは後続の "## " 大分類見出しで終端される
func TestExtractSection_StopAtUpperHeading(t *testing.T) {
	s, err := ExtractSection("document_send_channels")
	if err != nil {
		t.Fatalf("ExtractSection(document_send_channels) err: %v", err)
	}
	if !strings.Contains(s, "document_send_channels — 書類送付方法") {
		t.Errorf("should contain document_send_channels header")
	}
	// 後続の "## v0.5.0 破壊的変更" は含まれないこと
	if strings.Contains(s, "v0.5.0 破壊的変更") {
		t.Errorf("should NOT include upper-level heading '## v0.5.0 破壊的変更'")
	}
}

// D-E2: 存在しない resource
func TestExtractSection_Unknown(t *testing.T) {
	_, err := ExtractSection("no_such_resource")
	if err == nil {
		t.Fatalf("expected ErrSectionNotFound, got nil")
	}
	if !errors.Is(err, ErrSectionNotFound) {
		t.Fatalf("expected ErrSectionNotFound, got: %v", err)
	}
}

// D-7: Ransack を検索して複数件のマッチ（連続ヒットマージ後）、Section と Line が正しく入る
func TestSearch_Ransack(t *testing.T) {
	matches, err := Search("Ransack")
	if err != nil {
		t.Fatalf("Search(Ransack) err: %v", err)
	}
	// 実際の assets 内の Ransack 出現数は README=1, api-reference=3。
	// api-reference の 3 件は近接行ではないため通常 3 件、README 含めて計 4 件程度になる。
	if len(matches) < 3 {
		t.Fatalf("expected >= 3 matches, got %d", len(matches))
	}
	for _, m := range matches {
		if !strings.Contains(strings.ToLower(m.Content), "ransack") {
			t.Errorf("Content should contain ransack (case-insensitive): %s", m.Content)
		}
		if m.Line <= 0 {
			t.Errorf("Match.Line should be > 0, got %d", m.Line)
		}
		if m.File == "" {
			t.Errorf("Match.File should not be empty")
		}
	}
}

// D-8: 大文字小文字無視
func TestSearch_CaseInsensitive(t *testing.T) {
	lower, err := Search("board")
	if err != nil {
		t.Fatalf("Search(board) err: %v", err)
	}
	upper, err := Search("BOARD")
	if err != nil {
		t.Fatalf("Search(BOARD) err: %v", err)
	}
	if len(lower) != len(upper) {
		t.Errorf("case-insensitive mismatch: lower=%d upper=%d", len(lower), len(upper))
	}
}

// D-9: コンテキスト行が ±2 で取れる（最低限 1 行以上の content があること）
func TestSearch_ContextLines(t *testing.T) {
	matches, err := Search("Ransack")
	if err != nil {
		t.Fatalf("Search err: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("expected at least 1 match")
	}
	m := matches[0]
	lines := strings.Split(m.Content, "\n")
	if len(lines) < 1 {
		t.Errorf("Content should have at least 1 line, got: %q", m.Content)
	}
	if len(lines) > 5 {
		t.Errorf("Content should have at most 5 lines (±2 context), got %d lines", len(lines))
	}
}

// D-10: 同一セクション内の連続ヒットはマージされる
func TestSearch_ConsecutiveMerged(t *testing.T) {
	// "Ransack" は api-reference.md の冒頭付近で複数の連続行にマッチする。
	// マージ実装がなければ件数が増えるため、間隔 <=2 行の連続ヒットは
	// 1 Match に統合されることを期待する。
	matches, err := Search("Ransack")
	if err != nil {
		t.Fatalf("Search err: %v", err)
	}
	// 各マッチの (File, Line) が個別であることを検証（間隔 2 以下の重複が無い）
	type key struct {
		file string
		line int
	}
	seen := map[string]int{}
	for _, m := range matches {
		// 同一ファイル内で前の Match と Line 差が 2 以下にならないことを検証
		prevLine, ok := seen[m.File]
		if ok && m.Line-prevLine <= 2 {
			t.Errorf("consecutive matches not merged in %s: prev=%d, curr=%d", m.File, prevLine, m.Line)
		}
		seen[m.File] = m.Line
	}
}

// D-11: Match.Section フィールドに直近の "#### " 見出し名が入る
func TestSearch_SectionField(t *testing.T) {
	matches, err := Search("顧客名")
	if err != nil {
		t.Fatalf("Search err: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("expected at least 1 match for 顧客名")
	}
	// 少なくとも 1 件は clients セクションに属しているはず
	foundClients := false
	for _, m := range matches {
		if m.Section == "clients" {
			foundClients = true
			break
		}
	}
	if !foundClients {
		t.Errorf("expected at least one match with Section=clients, got matches: %+v", matches)
	}
}

// D-E3: 空キーワードは ErrEmptyKeyword
func TestSearch_Empty(t *testing.T) {
	_, err := Search("")
	if err == nil {
		t.Fatalf("expected ErrEmptyKeyword, got nil")
	}
	if !errors.Is(err, ErrEmptyKeyword) {
		t.Fatalf("expected ErrEmptyKeyword, got: %v", err)
	}
}

// D-E4: マッチなしは空スライス (nil でなく) + err=nil
func TestSearch_NoMatch(t *testing.T) {
	matches, err := Search("きわめて珍しい文字列XYZ123ABC")
	if err != nil {
		t.Fatalf("expected nil err, got: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected empty matches, got %d", len(matches))
	}
}
