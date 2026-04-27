package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// runDocsCmd executes a `board docs` subcommand in isolation with captured stdout/stderr.
// It uses NewDocsCmd() directly so root-level PersistentPreRunE (which requires a
// valid profile config) is bypassed. Returns (stdout, stderr, err) from the RunE path.
func runDocsCmd(t *testing.T, args []string) (string, string, error) {
	t.Helper()
	cmd := NewDocsCmd()
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), errb.String(), err
}

// C-1: `board docs`（text）で README を表示
func TestDocs_Readme(t *testing.T) {
	stdout, _, err := runDocsCmd(t, nil)
	if err != nil {
		t.Fatalf("runDocsCmd err: %v", err)
	}
	if !strings.Contains(stdout, "# board") {
		t.Errorf("stdout should contain README heading '# board', got: %s...", stdout[:min(len(stdout), 80)])
	}
}

// C-2: `board docs --format json` で readme モードの JSON
func TestDocs_Readme_JSON(t *testing.T) {
	stdout, _, err := runDocsCmd(t, []string{"--format", "json"})
	if err != nil {
		t.Fatalf("runDocsCmd err: %v", err)
	}
	var got struct {
		Mode    string `json:"mode"`
		Query   string `json:"query"`
		Results []struct {
			File    string `json:"file"`
			Section string `json:"section"`
			Content string `json:"content"`
			Line    int    `json:"line"`
			Size    int    `json:"size"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid JSON: %v\nstdout=%s", err, stdout)
	}
	if got.Mode != "readme" {
		t.Errorf("mode=%q, want readme", got.Mode)
	}
	if len(got.Results) != 1 {
		t.Errorf("results len=%d, want 1", len(got.Results))
	}
	if !strings.Contains(got.Results[0].Content, "# board") {
		t.Errorf("results[0].content should contain '# board'")
	}
}

// C-3: `board docs --list --format json`
func TestDocs_List_JSON(t *testing.T) {
	stdout, _, err := runDocsCmd(t, []string{"--list", "--format", "json"})
	if err != nil {
		t.Fatalf("runDocsCmd err: %v", err)
	}
	var got struct {
		Mode    string `json:"mode"`
		Results []struct {
			File string `json:"file"`
			Size int    `json:"size"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid JSON: %v\nstdout=%s", err, stdout)
	}
	if got.Mode != "list" {
		t.Errorf("mode=%q, want list", got.Mode)
	}
	if len(got.Results) != 8 {
		t.Errorf("results len=%d, want 8", len(got.Results))
	}
	for _, r := range got.Results {
		if r.File == "" {
			t.Errorf("file should not be empty: %+v", r)
		}
		if r.Size <= 0 {
			t.Errorf("size should be > 0: %+v", r)
		}
	}
}

// C-4: `board docs --search Ransack` (text)
func TestDocs_Search(t *testing.T) {
	stdout, _, err := runDocsCmd(t, []string{"--search", "Ransack"})
	if err != nil {
		t.Fatalf("runDocsCmd err: %v", err)
	}
	if !strings.Contains(stdout, "api-reference.md:") {
		t.Errorf("stdout should contain 'api-reference.md:' line, got: %s", stdout)
	}
}

// C-5: `board docs --search Ransack --format json`
func TestDocs_Search_JSON(t *testing.T) {
	stdout, _, err := runDocsCmd(t, []string{"--search", "Ransack", "--format", "json"})
	if err != nil {
		t.Fatalf("runDocsCmd err: %v", err)
	}
	var got struct {
		Mode    string `json:"mode"`
		Query   string `json:"query"`
		Results []struct {
			File string `json:"file"`
			Line int    `json:"line"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.Mode != "search" {
		t.Errorf("mode=%q, want search", got.Mode)
	}
	if got.Query != "Ransack" {
		t.Errorf("query=%q, want Ransack", got.Query)
	}
	if len(got.Results) < 3 {
		t.Errorf("results len=%d, want >= 3", len(got.Results))
	}
}

// C-6: `board docs clients`（text）
func TestDocs_Resource(t *testing.T) {
	stdout, _, err := runDocsCmd(t, []string{"clients"})
	if err != nil {
		t.Fatalf("runDocsCmd err: %v", err)
	}
	if !strings.Contains(stdout, "clients — 顧客マスタ") {
		t.Errorf("stdout should contain clients section heading, got first 200 chars: %s", stdout[:min(len(stdout), 200)])
	}
}

// C-7: `board docs clients --format json`
func TestDocs_Resource_JSON(t *testing.T) {
	stdout, _, err := runDocsCmd(t, []string{"clients", "--format", "json"})
	if err != nil {
		t.Fatalf("runDocsCmd err: %v", err)
	}
	var got struct {
		Mode    string `json:"mode"`
		Query   string `json:"query"`
		Results []struct {
			File    string `json:"file"`
			Section string `json:"section"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.Mode != "resource" {
		t.Errorf("mode=%q, want resource", got.Mode)
	}
	if got.Query != "clients" {
		t.Errorf("query=%q, want clients", got.Query)
	}
	if len(got.Results) != 1 {
		t.Fatalf("results len=%d, want 1", len(got.Results))
	}
	if got.Results[0].Section != "clients" {
		t.Errorf("section=%q, want clients", got.Results[0].Section)
	}
	if !strings.Contains(got.Results[0].Content, "clients — 顧客マスタ") {
		t.Errorf("content should contain clients heading")
	}
}

// C-E1: 存在しない resource → エラー
func TestDocs_Resource_NotFound(t *testing.T) {
	_, _, err := runDocsCmd(t, []string{"no_such_resource"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "section not found") {
		t.Errorf("error should mention 'section not found', got: %v", err)
	}
}

// C-E2: 不正な format → エラー
func TestDocs_Format_Invalid(t *testing.T) {
	_, _, err := runDocsCmd(t, []string{"--format", "xml"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Errorf("error should mention 'unsupported format', got: %v", err)
	}
}

// C-E3: --list と --search 同時指定 → cobra の mutually exclusive エラー
func TestDocs_Conflict_ListSearch(t *testing.T) {
	_, _, err := runDocsCmd(t, []string{"--list", "--search", "foo"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

// C-E4: resource 引数 + --list → エラー
func TestDocs_Conflict_ResourceWithList(t *testing.T) {
	_, _, err := runDocsCmd(t, []string{"clients", "--list"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "resource argument cannot be combined") {
		t.Errorf("error should mention resource/flag conflict, got: %v", err)
	}
}

// C-E5: resource 引数 + --search → エラー
func TestDocs_Conflict_ResourceWithSearch(t *testing.T) {
	_, _, err := runDocsCmd(t, []string{"clients", "--search", "foo"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "resource argument cannot be combined") {
		t.Errorf("error should mention resource/flag conflict, got: %v", err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
