// Package docs provides embedded access to the BOARD CLI documentation set
// (README, api-reference, installation guides, and usage guides) and a few
// lightweight utilities (List / ExtractSection / Search) used by the
// `board docs` subcommand.
//
// The embedded files are a synchronized copy of the repository's top-level
// README.md and docs/ directory (excluding docs/specs/). Use the mise task
// `mise run sync-docs` after editing user-facing documentation to refresh
// internal/docs/assets/, and `mise run check-docs-sync` in CI / locally to
// detect drift.
package docs

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"
)

//go:embed assets
var assetsFS embed.FS

const assetsPrefix = "assets"

// FileEntry represents one embedded documentation file (used for List).
type FileEntry struct {
	File string `json:"file"` // relative path with "assets/" prefix stripped, e.g. "api-reference.md"
	Size int    `json:"size"`
}

// Match represents a single search hit (used for Search).
type Match struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Section string `json:"section"` // nearest enclosing "#### " header, "" if none
	Content string `json:"content"` // matched line + up to ±2 lines of context
}

var (
	// ErrSectionNotFound is returned by ExtractSection when no matching resource header exists.
	ErrSectionNotFound = errors.New("docs: section not found")
	// ErrEmptyKeyword is returned by Search when the keyword is empty.
	ErrEmptyKeyword = errors.New("docs: keyword must not be empty")
)

// FS returns the embedded docs filesystem (rooted at "assets").
func FS() fs.FS { return assetsFS }

// Read reads an embedded file by relative path (without the "assets/" prefix).
func Read(path string) ([]byte, error) {
	return assetsFS.ReadFile(assetsPrefix + "/" + path)
}

// List returns all embedded files sorted by File path (ascending).
func List() ([]FileEntry, error) {
	var entries []FileEntry
	err := fs.WalkDir(assetsFS, assetsPrefix, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(p, assetsPrefix+"/")
		entries = append(entries, FileEntry{File: rel, Size: int(info.Size())})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].File < entries[j].File })
	return entries, nil
}

// sectionHeaderRe matches any "#### <name> — " (em-dash with surrounding ASCII spaces).
// Used to (1) detect the start/end of resource sections and (2) derive Search's Section field.
var sectionHeaderRe = regexp.MustCompile(`^#### ([A-Za-z_][A-Za-z0-9_]*) — `)

// upperHeadingRe matches "## " or "### " upper-level headings.
var upperHeadingRe = regexp.MustCompile(`^(##|###) `)

// ExtractSection extracts a resource section from api-reference.md.
//
// Header contract (strict): the line must match exactly
//
//	(?m)^#### <resource> —
//
// where <resource> is regexp.QuoteMeta(resource) followed by the literal " — "
// (em-dash with surrounding ASCII spaces). This avoids prefix ambiguity
// such as ExtractSection("client") incorrectly matching "#### client_branches —".
//
// The section ends at the first subsequent line matching any of:
//   - another "#### <any> — " (next resource section)
//   - "### ..." (mid-level heading, e.g. "### マスタ")
//   - "## ..."  (top-level heading, e.g. "## v0.5.0 破壊的変更")
//   - EOF
//
// Trailing "---" horizontal rules (and the blank lines adjacent to them) are
// stripped from the returned text.
func ExtractSection(resource string) (string, error) {
	content, err := Read("api-reference.md")
	if err != nil {
		return "", fmt.Errorf("docs: read api-reference.md: %w", err)
	}
	startRe := regexp.MustCompile(`(?m)^#### ` + regexp.QuoteMeta(resource) + ` — `)
	loc := startRe.FindIndex(content)
	if loc == nil {
		return "", ErrSectionNotFound
	}
	start := loc[0]
	// Look for the next boundary line after the start header.
	rest := content[start:]
	lines := bytes.Split(rest, []byte("\n"))
	endLineIdx := len(lines) // default: EOF
	for i := 1; i < len(lines); i++ {
		line := string(lines[i])
		if sectionHeaderRe.MatchString(line) {
			endLineIdx = i
			break
		}
		if upperHeadingRe.MatchString(line) {
			endLineIdx = i
			break
		}
	}
	sectionLines := lines[:endLineIdx]
	// Trim trailing "---" and blank lines that act as section separators.
	for len(sectionLines) > 0 {
		last := strings.TrimSpace(string(sectionLines[len(sectionLines)-1]))
		if last == "" || last == "---" {
			sectionLines = sectionLines[:len(sectionLines)-1]
			continue
		}
		break
	}
	return string(bytes.Join(sectionLines, []byte("\n"))), nil
}

// Search performs case-insensitive substring search over all embedded .md
// files using strings.ToLower (Unicode-aware). It returns matches with up
// to ±2 lines of context and the nearest enclosing "#### " section name.
//
// Consecutive matches within 2 lines of each other in the same file are
// merged into a single Match to avoid duplicated context bloat (this is
// especially important for LLM token cost when used via MCP).
//
// Empty keyword returns ErrEmptyKeyword; no matches returns an empty slice
// with nil error.
func Search(keyword string) ([]Match, error) {
	if keyword == "" {
		return nil, ErrEmptyKeyword
	}
	lowerKeyword := strings.ToLower(keyword)
	var matches []Match
	err := fs.WalkDir(assetsFS, assetsPrefix, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		data, err := assetsFS.ReadFile(p)
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(p, assetsPrefix+"/")
		matches = append(matches, searchFile(rel, data, lowerKeyword)...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if matches == nil {
		matches = []Match{}
	}
	return matches, nil
}

// searchFile scans data for lines containing lowerKeyword (case-insensitive).
// It builds Match entries with ±2 lines of context and the nearest preceding
// "#### " section header, merging consecutive hits within 2 lines.
func searchFile(file string, data []byte, lowerKeyword string) []Match {
	// Handle both LF and CRLF line endings defensively.
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")

	// Pre-compute the nearest "#### " section name per line.
	sections := make([]string, len(lines))
	currentSection := ""
	for i, line := range lines {
		if m := sectionHeaderRe.FindStringSubmatch(line); m != nil {
			currentSection = m[1]
		}
		sections[i] = currentSection
	}

	// Identify hit line indices (0-based).
	var hitIdx []int
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), lowerKeyword) {
			hitIdx = append(hitIdx, i)
		}
	}
	if len(hitIdx) == 0 {
		return nil
	}

	// Merge consecutive hits whose distance <= 2.
	type hitRange struct{ start, hit, end int }
	var ranges []hitRange
	for _, i := range hitIdx {
		if len(ranges) == 0 {
			ranges = append(ranges, hitRange{start: maxInt(0, i-2), hit: i, end: minInt(len(lines)-1, i+2)})
			continue
		}
		last := &ranges[len(ranges)-1]
		if i-last.hit <= 2 {
			// Extend existing range's context window.
			if i+2 > last.end {
				last.end = minInt(len(lines)-1, i+2)
			}
			// Keep the first hit as the canonical line.
			continue
		}
		ranges = append(ranges, hitRange{start: maxInt(0, i-2), hit: i, end: minInt(len(lines)-1, i+2)})
	}

	result := make([]Match, 0, len(ranges))
	for _, r := range ranges {
		contextLines := lines[r.start : r.end+1]
		result = append(result, Match{
			File:    file,
			Line:    r.hit + 1, // 1-based
			Section: sections[r.hit],
			Content: strings.Join(contextLines, "\n"),
		})
	}
	return result
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
