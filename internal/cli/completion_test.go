package cli_test

import (
	"strings"
	"testing"

	"github.com/youyo/board/internal/cli"
)

func TestNewCompletionCmd_SubCommands(t *testing.T) {
	cmd := cli.NewCompletionCmd()

	names := map[string]bool{}
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}

	for _, want := range []string{"zsh", "bash"} {
		if !names[want] {
			t.Errorf("completion %s subcommand not found", want)
		}
	}
}

func TestNewRootCmd_HasCompletionSubCmd(t *testing.T) {
	root := cli.NewRootCmd("dev")

	var found bool
	for _, sub := range root.Commands() {
		if sub.Name() == "completion" {
			found = true
			break
		}
	}
	if !found {
		t.Error("completion subcommand not found in root")
	}
}

func TestCompletionZsh_Output(t *testing.T) {
	root := cli.NewRootCmd("dev")
	buf := &strings.Builder{}
	root.SetOut(buf)
	root.SetArgs([]string{"completion", "zsh"})

	if err := root.Execute(); err != nil {
		t.Fatalf("completion zsh failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "compdef") && !strings.Contains(out, "#compdef") {
		t.Errorf("zsh completion output does not look like zsh script: %q", out[:min(len(out), 200)])
	}
}

func TestCompletionBash_Output(t *testing.T) {
	root := cli.NewRootCmd("dev")
	buf := &strings.Builder{}
	root.SetOut(buf)
	root.SetArgs([]string{"completion", "bash"})

	if err := root.Execute(); err != nil {
		t.Fatalf("completion bash failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "bash") {
		t.Errorf("bash completion output does not look like bash script: %q", out[:min(len(out), 200)])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
