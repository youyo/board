package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/docs"
	"github.com/youyo/board/internal/output"
)

// docsResultItem is the union output row used by all docs modes.
type docsResultItem struct {
	File    string `json:"file,omitempty"`
	Section string `json:"section,omitempty"`
	Content string `json:"content,omitempty"`
	Line    int    `json:"line,omitempty"`
	Size    int    `json:"size,omitempty"`
}

type docsPayload struct {
	Mode    string           `json:"mode"`
	Query   string           `json:"query,omitempty"`
	Results []docsResultItem `json:"results"`
}

// NewDocsCmd returns the `board docs` subcommand which surfaces the embedded
// documentation set (README, api-reference, installation, guides) for human
// and LLM consumers alike.
func NewDocsCmd() *cobra.Command {
	var (
		list   bool
		search string
		format string
	)
	cmd := &cobra.Command{
		Use:   "docs [resource]",
		Short: "Show embedded BOARD CLI documentation",
		Long: `board docs shows embedded BOARD CLI documentation.

Modes:
  board docs                    Print README.md to stdout
  board docs --list             List all embedded documentation files
  board docs --search <word>    Case-insensitive search across all docs
  board docs <resource>         Extract a resource section from api-reference.md

--format text (default) prints human-oriented output. --format json emits a
machine-readable payload (mode / query / results[]) suitable for LLM and MCP
consumption. The root --pretty flag applies to JSON output only.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDocs(cmd, args, list, search, format)
		},
	}
	cmd.Flags().BoolVar(&list, "list", false, "List all embedded documentation files")
	cmd.Flags().StringVar(&search, "search", "", "Search embedded docs for the given keyword (case-insensitive)")
	cmd.Flags().StringVar(&format, "format", "text", `Output format: "text" (default) or "json"`)
	cmd.MarkFlagsMutuallyExclusive("list", "search")
	_ = cmd.RegisterFlagCompletionFunc("format", staticCompletion([]string{"text", "json"}))
	return cmd
}

func runDocs(cmd *cobra.Command, args []string, list bool, search, format string) error {
	if format != "text" && format != "json" {
		return fmt.Errorf("unsupported format: %q (expected text or json)", format)
	}
	if (list || search != "") && len(args) > 0 {
		return errors.New("resource argument cannot be combined with --list or --search")
	}

	out := cmd.OutOrStdout()
	pretty := prettyFromCmd(cmd)

	switch {
	case list:
		return runDocsList(out, format, pretty)
	case search != "":
		return runDocsSearch(out, search, format, pretty)
	case len(args) == 1:
		return runDocsResource(out, args[0], format, pretty)
	default:
		return runDocsReadme(out, format, pretty)
	}
}

func runDocsReadme(out io.Writer, format string, pretty bool) error {
	data, err := docs.Read("README.md")
	if err != nil {
		return fmt.Errorf("read README.md: %w", err)
	}
	if format == "json" {
		payload := docsPayload{
			Mode: "readme",
			Results: []docsResultItem{{
				File:    "README.md",
				Content: string(data),
			}},
		}
		return output.Write(out, payload, pretty)
	}
	_, err = out.Write(data)
	return err
}

func runDocsList(out io.Writer, format string, pretty bool) error {
	entries, err := docs.List()
	if err != nil {
		return fmt.Errorf("list docs: %w", err)
	}
	if format == "json" {
		items := make([]docsResultItem, 0, len(entries))
		for _, e := range entries {
			items = append(items, docsResultItem{File: e.File, Size: e.Size})
		}
		return output.Write(out, docsPayload{Mode: "list", Results: items}, pretty)
	}
	for _, e := range entries {
		if _, err := fmt.Fprintf(out, "%s\t%d\n", e.File, e.Size); err != nil {
			return err
		}
	}
	return nil
}

func runDocsSearch(out io.Writer, keyword, format string, pretty bool) error {
	matches, err := docs.Search(keyword)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}
	if format == "json" {
		items := make([]docsResultItem, 0, len(matches))
		for _, m := range matches {
			items = append(items, docsResultItem{
				File:    m.File,
				Section: m.Section,
				Content: m.Content,
				Line:    m.Line,
			})
		}
		return output.Write(out, docsPayload{Mode: "search", Query: keyword, Results: items}, pretty)
	}
	// grep-like text rendering
	for _, m := range matches {
		if m.Section != "" {
			if _, err := fmt.Fprintf(out, "-- section: %s --\n", m.Section); err != nil {
				return err
			}
		}
		for i, line := range strings.Split(m.Content, "\n") {
			if _, err := fmt.Fprintf(out, "%s:%d | %s\n", m.File, m.Line+i, line); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(out); err != nil {
			return err
		}
	}
	return nil
}

func runDocsResource(out io.Writer, resource, format string, pretty bool) error {
	section, err := docs.ExtractSection(resource)
	if err != nil {
		return fmt.Errorf("extract section: %w", err)
	}
	if format == "json" {
		payload := docsPayload{
			Mode:  "resource",
			Query: resource,
			Results: []docsResultItem{{
				File:    "api-reference.md",
				Section: resource,
				Content: section,
			}},
		}
		return output.Write(out, payload, pretty)
	}
	_, err = fmt.Fprintln(out, section)
	return err
}
