package docs_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// boardBinEnv is the environment variable holding the path to a test-local
// board binary built in TestMain. Tests that need to invoke `board docs ...`
// read this variable rather than assuming a system-installed binary.
const boardBinEnv = "BOARD_M60_TEST_BIN"

// TestMain builds a fresh board binary into a temp directory so that
// TestSkillBody_BashCommandsExecute can invoke `board docs ...` reliably,
// even in CI where no prior build exists. The binary is removed after the
// test run completes.
func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "board_m60_")
	if err != nil {
		log.Fatalf("TestMain: create temp dir: %v", err)
	}
	binPath := filepath.Join(tmpDir, "board")

	// The test working directory is internal/docs. Build the binary from repo root (../..).
	build := exec.Command("go", "build", "-o", binPath, "./cmd/board")
	build.Dir = filepath.Join("..", "..")
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		_ = os.RemoveAll(tmpDir)
		log.Fatalf("TestMain: go build failed: %v", err)
	}

	if err := os.Setenv(boardBinEnv, binPath); err != nil {
		_ = os.RemoveAll(tmpDir)
		log.Fatalf("TestMain: setenv: %v", err)
	}

	code := m.Run()
	_ = os.RemoveAll(tmpDir)
	os.Exit(code)
}

// ------------------------ S-0: plugin manifest ------------------------

// TestPluginManifest_Valid verifies that .claude-plugin/plugin.json exists
// and declares the plugin name as "board" (required for /board:docs namespace
// resolution together with SKILL.md frontmatter name "board:docs").
func TestPluginManifest_Valid(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".claude-plugin", "plugin.json"))
	if err != nil {
		t.Fatalf("read plugin.json: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse plugin.json: %v", err)
	}
	if got, ok := manifest["name"].(string); !ok || got != "board" {
		t.Errorf("plugin.json name: got %q, want %q", manifest["name"], "board")
	}
	if got, ok := manifest["description"].(string); !ok || got == "" {
		t.Errorf("plugin.json description: expected non-empty string")
	}
	if got, ok := manifest["version"].(string); !ok || got == "" {
		t.Errorf("plugin.json version: expected non-empty string")
	}
}

// ------------------------ Skill frontmatter helpers ------------------------

// skillMeta is the minimal frontmatter schema we care about.
type skillMeta struct {
	Name        string
	Description string
}

// parseSkillFile splits SKILL.md into its frontmatter block and body.
// The frontmatter is expected to be the first YAML block delimited by "---"
// at lines 1 and some subsequent line. This parser only understands the
// constructs we actually use: "key: value" and the ">-" block scalar for
// multi-line description. Keeping this local avoids adding a yaml.v3 direct
// dependency just for tests.
func parseSkillFile(t *testing.T) (skillMeta, string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "skills", "docs", "SKILL.md"))
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		t.Fatalf("SKILL.md must start with '---' frontmatter delimiter")
	}
	// Find the closing "---" on its own line.
	rest := text[len("---\n"):]
	closeIdx := strings.Index(rest, "\n---\n")
	if closeIdx == -1 {
		// Allow "---" at the very end with trailing newline.
		if strings.HasSuffix(rest, "\n---") {
			closeIdx = len(rest) - len("\n---")
		} else {
			t.Fatalf("SKILL.md frontmatter has no closing '---'")
		}
	}
	frontmatter := rest[:closeIdx]
	body := ""
	if closeIdx+len("\n---\n") <= len(rest) {
		body = rest[closeIdx+len("\n---\n"):]
	}

	meta := parseFrontmatter(t, frontmatter)
	return meta, body
}

// parseFrontmatter understands:
//   - "key: value"             → single-line scalar
//   - "key: >-" + indented     → folded block scalar (multi-line description)
//   - "key: |"  + indented     → literal block scalar (not currently used, tolerated)
func parseFrontmatter(t *testing.T, fm string) skillMeta {
	t.Helper()
	var meta skillMeta
	lines := strings.Split(fm, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			continue
		}
		// Only care about top-level (unindented) keys.
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		colon := strings.Index(line, ":")
		if colon == -1 {
			continue
		}
		key := strings.TrimSpace(line[:colon])
		valRest := strings.TrimSpace(line[colon+1:])

		if valRest == ">-" || valRest == ">" || valRest == "|" || valRest == "|-" {
			// Collect indented continuation lines.
			var folded []string
			i++
			for ; i < len(lines); i++ {
				ln := lines[i]
				if ln == "" {
					// blank line inside block scalar → paragraph break
					folded = append(folded, "")
					continue
				}
				if !strings.HasPrefix(ln, "  ") && !strings.HasPrefix(ln, "\t") {
					i-- // unread; let outer loop process
					break
				}
				folded = append(folded, strings.TrimLeft(ln, " \t"))
			}
			joined := strings.Join(folded, " ")
			// Collapse multiple spaces introduced by blank-line handling above.
			joined = regexp.MustCompile(`\s+`).ReplaceAllString(joined, " ")
			joined = strings.TrimSpace(joined)
			switch key {
			case "name":
				meta.Name = joined
			case "description":
				meta.Description = joined
			}
			continue
		}

		// Plain scalar; strip optional surrounding quotes.
		val := valRest
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		switch key {
		case "name":
			meta.Name = val
		case "description":
			meta.Description = val
		}
	}
	return meta
}

// ------------------------ S-1..S-5: frontmatter ------------------------

func TestSkillFile_Exists(t *testing.T) {
	info, err := os.Stat(filepath.Join("..", "..", "skills", "docs", "SKILL.md"))
	if err != nil {
		t.Fatalf("stat SKILL.md: %v", err)
	}
	if info.Size() < 100 {
		t.Errorf("SKILL.md too small: %d bytes (expected >= 100)", info.Size())
	}
}

func TestSkillFrontmatter_Parseable(t *testing.T) {
	meta, body := parseSkillFile(t)
	if meta.Name == "" {
		t.Errorf("frontmatter.name is empty")
	}
	if body == "" {
		t.Errorf("SKILL.md body is empty")
	}
}

func TestSkillFrontmatter_Name(t *testing.T) {
	meta, _ := parseSkillFile(t)
	if meta.Name != "board:docs" {
		t.Errorf("frontmatter.name: got %q, want %q (plugin prefix required for /board:docs namespace)", meta.Name, "board:docs")
	}
}

func TestSkillFrontmatter_Description_NonEmpty(t *testing.T) {
	meta, _ := parseSkillFile(t)
	if n := len(meta.Description); n < 40 {
		t.Errorf("frontmatter.description too short: %d chars (want >= 40)", n)
	}
}

func TestSkillFrontmatter_Description_HasUseCase(t *testing.T) {
	meta, _ := parseSkillFile(t)
	lower := strings.ToLower(meta.Description)
	if !strings.Contains(lower, "board docs") && !strings.Contains(lower, "board") {
		t.Errorf("frontmatter.description must reference 'board' or 'board docs' for LLM triggering; got %q", meta.Description)
	}
}

// ------------------------ S-6..S-10: body content ------------------------

func TestSkillBody_HasList(t *testing.T) {
	_, body := parseSkillFile(t)
	if !strings.Contains(body, "board docs --list") {
		t.Errorf("SKILL.md body must reference `board docs --list`")
	}
}

func TestSkillBody_HasResourceExample(t *testing.T) {
	_, body := parseSkillFile(t)
	// Accept either the literal `<resource>` placeholder or a concrete example like "board docs clients".
	if !strings.Contains(body, "board docs <resource>") && !strings.Contains(body, "board docs clients") {
		t.Errorf("SKILL.md body must include a resource lookup example such as `board docs <resource>` or `board docs clients`")
	}
}

func TestSkillBody_HasFormatJSON(t *testing.T) {
	_, body := parseSkillFile(t)
	if !strings.Contains(body, "--format json") {
		t.Errorf("SKILL.md body must mention `--format json`")
	}
}

func TestSkillBody_HasSearch(t *testing.T) {
	_, body := parseSkillFile(t)
	if !strings.Contains(body, "--search") {
		t.Errorf("SKILL.md body must mention `--search`")
	}
}

func TestSkillBody_HasTroubleshooting(t *testing.T) {
	_, body := parseSkillFile(t)
	if !strings.Contains(body, "Troubleshooting") && !strings.Contains(body, "トラブルシュート") {
		t.Errorf("SKILL.md body must include a Troubleshooting / トラブルシュート section")
	}
}

// ------------------------ S-11: bash commands execute ------------------------

// extractBoardDocsCommands scans the SKILL.md body for fenced shell blocks
// (```sh / ```bash / ```shell) and returns every `board docs ...` command
// line found inside them. Lines are returned with the leading "board "
// stripped so the caller can Exec the binary directly.
func extractBoardDocsCommands(body string) [][]string {
	var cmds [][]string
	lines := strings.Split(body, "\n")
	inShell := false
	for _, raw := range lines {
		trim := strings.TrimSpace(raw)
		if strings.HasPrefix(trim, "```") {
			fence := strings.TrimPrefix(trim, "```")
			switch fence {
			case "sh", "bash", "shell":
				inShell = true
			default:
				inShell = false
			}
			continue
		}
		if !inShell {
			continue
		}
		// Strip inline comments (# ...) but keep quoted # intact.
		// A simple heuristic is sufficient for SKILL.md examples.
		cmdLine := stripInlineComment(trim)
		if !strings.HasPrefix(cmdLine, "board docs") {
			continue
		}
		// Drop shell pipelines; we only exec the `board docs ...` segment.
		if idx := strings.Index(cmdLine, "|"); idx >= 0 {
			cmdLine = strings.TrimSpace(cmdLine[:idx])
		}
		// Strip "2>&1" style redirections.
		cmdLine = regexp.MustCompile(`\s+\d?>&?\S+`).ReplaceAllString(cmdLine, "")
		cmdLine = strings.TrimSpace(cmdLine)
		args := splitArgs(cmdLine)
		if len(args) < 1 || args[0] != "board" {
			continue
		}
		cmds = append(cmds, args[1:]) // drop the leading "board"
	}
	return cmds
}

// stripInlineComment removes trailing "# ..." comments that are not inside
// quotes. This is a minimal implementation that handles SKILL.md examples.
func stripInlineComment(s string) string {
	inSingle := false
	inDouble := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\'' && !inDouble {
			inSingle = !inSingle
		}
		if c == '"' && !inSingle {
			inDouble = !inDouble
		}
		if c == '#' && !inSingle && !inDouble {
			// Require the '#' to be at start of line or preceded by whitespace.
			if i == 0 || s[i-1] == ' ' || s[i-1] == '\t' {
				return strings.TrimRight(s[:i], " \t")
			}
		}
	}
	return s
}

// splitArgs is a tiny shell-word splitter good enough for our documented
// examples (double-quoted strings, no escapes). It intentionally does not
// handle every POSIX edge case.
func splitArgs(s string) []string {
	var args []string
	var cur strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			inQuote = !inQuote
		case ' ', '\t':
			if inQuote {
				cur.WriteByte(c)
			} else if cur.Len() > 0 {
				args = append(args, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		args = append(args, cur.String())
	}
	return args
}

func TestSkillBody_BashCommandsExecute(t *testing.T) {
	bin := os.Getenv(boardBinEnv)
	if bin == "" {
		t.Fatalf("%s is empty; TestMain should have set it", boardBinEnv)
	}
	_, body := parseSkillFile(t)
	cmds := extractBoardDocsCommands(body)
	if len(cmds) == 0 {
		t.Fatalf("no `board docs` commands found in SKILL.md body; SKILL.md probably lacks executable examples")
	}
	for i, args := range cmds {
		// The commands might include placeholders like <resource>; skip those
		// – we cannot execute an instructional placeholder.
		if containsPlaceholder(args) {
			continue
		}
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(bin, args...)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil {
			t.Errorf("command #%d %v failed: %v\nstderr: %s", i, args, err, stderr.String())
			continue
		}
		if stdout.Len() == 0 && stderr.Len() == 0 {
			t.Errorf("command #%d %v produced no output", i, args)
		}
	}
}

func containsPlaceholder(args []string) bool {
	for _, a := range args {
		if strings.Contains(a, "<") && strings.Contains(a, ">") {
			return true
		}
	}
	return false
}

// ------------------------ S-12: no resource duplication ------------------------

func TestSkill_NoResourceListDuplication(t *testing.T) {
	_, body := parseSkillFile(t)
	// Full BOARD resource set (22). If SKILL.md lists more than 5 of these
	// verbatim it likely duplicates information that should live in
	// api-reference.md instead. We allow up to 5 so that 2-3 example calls
	// plus incidental mentions remain possible.
	resources := []string{
		"clients", "client_branches", "contacts", "projects", "project_costs",
		"estimates", "invoices", "orders", "deliveries", "receipts",
		"vendors", "vendor_branches", "vendor_contacts", "purchase_orders", "payments",
		"users", "groups", "payment_terms", "project_types", "purchase_types",
		"accounting_types", "document_send_channels",
	}
	hits := 0
	var hit []string
	for _, r := range resources {
		// Count distinct resource names only (presence, not occurrences).
		if strings.Contains(body, r) {
			hits++
			hit = append(hit, r)
		}
	}
	if hits > 5 {
		t.Errorf("SKILL.md mentions %d BOARD resources (limit 5, to avoid duplicating api-reference.md): %v", hits, hit)
	}
}

// ------------------------ S-13/S-14: README sections ------------------------

func TestReadme_HasSkillSection(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	if !bytes.Contains(data, []byte("/board:docs")) {
		t.Errorf("README.md must reference `/board:docs` skill")
	}
}

func TestReadmeJa_HasSkillSection(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "README_ja.md"))
	if err != nil {
		t.Fatalf("read README_ja.md: %v", err)
	}
	if !bytes.Contains(data, []byte("/board:docs")) {
		t.Errorf("README_ja.md must reference `/board:docs` skill")
	}
}

// Avoid unused import warnings if extractBoardDocsCommands stops using fmt.
var _ = fmt.Sprintf
