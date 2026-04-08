package cli_test

import (
	"strings"
	"testing"

	"github.com/youyo/board/internal/cli"
	"github.com/youyo/board/internal/config"
)

func TestConfigureGetCmd(t *testing.T) {
	t.Run("timezone を get すると UTC が返る", func(t *testing.T) {
		newTempConfig(t)

		root := cli.NewConfigureCmd()
		out, err := executeCmd(t, root, "get", "timezone")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := strings.TrimSpace(out)
		if got != "UTC" {
			t.Errorf("expected UTC, got %q", got)
		}
	})

	t.Run("profiles.default.base_url を get すると base URL が返る", func(t *testing.T) {
		newTempConfig(t)

		root := cli.NewConfigureCmd()
		out, err := executeCmd(t, root, "get", "profiles.default.base_url")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := strings.TrimSpace(out)
		if got != "https://api.the-board.jp" {
			t.Errorf("expected https://api.the-board.jp, got %q", got)
		}
	})

	t.Run("profiles.default.api_key を get するとマスクされた値が返る", func(t *testing.T) {
		path := newTempConfig(t)

		cfg := config.DefaultConfig()
		prof := config.DefaultProfileConfig()
		prof.APIKey = "abcdefghij"
		config.AddOrUpdateProfile(&cfg, "default", prof)
		if err := config.Save(cfg, path); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		root := cli.NewConfigureCmd()
		out, err := executeCmd(t, root, "get", "profiles.default.api_key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := strings.TrimSpace(out)
		// 長さ10: 先頭2文字 + **** + 末尾2文字 = "ab****ij"
		if got != "ab****ij" {
			t.Errorf("expected ab****ij, got %q", got)
		}
	})

	t.Run("api_key が空の場合マスクは空文字", func(t *testing.T) {
		newTempConfig(t)

		root := cli.NewConfigureCmd()
		out, err := executeCmd(t, root, "get", "profiles.default.api_key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := strings.TrimSpace(out)
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("不正なキーはエラー", func(t *testing.T) {
		newTempConfig(t)

		root := cli.NewConfigureCmd()
		_, err := executeCmd(t, root, "get", "invalid.key.path.too.long")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("引数なしは cobra の args エラー", func(t *testing.T) {
		newTempConfig(t)

		root := cli.NewConfigureCmd()
		_, err := executeCmd(t, root, "get")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestParseKey(t *testing.T) {
	tests := []struct {
		key         string
		wantScope   string
		wantProfile string
		wantField   string
		wantErr     bool
	}{
		{
			key: "timezone", wantScope: "top", wantField: "timezone",
		},
		{
			key: "current_profile", wantScope: "top", wantField: "current_profile",
		},
		{
			key: "profiles.default.api_key", wantScope: "profiles", wantProfile: "default", wantField: "api_key",
		},
		{
			key: "profiles.production.base_url", wantScope: "profiles", wantProfile: "production", wantField: "base_url",
		},
		{key: "invalid", wantErr: true},
		{key: "profiles", wantErr: true},
		{key: "profiles.only_two", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			scope, profile, field, err := cli.ParseKey(tc.key)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for key %q, got nil", tc.key)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if scope != tc.wantScope {
				t.Errorf("scope: expected %q, got %q", tc.wantScope, scope)
			}
			if profile != tc.wantProfile {
				t.Errorf("profile: expected %q, got %q", tc.wantProfile, profile)
			}
			if field != tc.wantField {
				t.Errorf("field: expected %q, got %q", tc.wantField, field)
			}
		})
	}
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"a", "****"},
		{"ab", "****"},
		{"abc", "****"},
		{"abcd", "a****d"},
		{"abcdefg", "a****g"},
		{"abcdefgh", "ab****gh"},
		{"abcdefghij", "ab****ij"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := cli.MaskSecret(tc.input)
			if got != tc.expected {
				t.Errorf("MaskSecret(%q): expected %q, got %q", tc.input, tc.expected, got)
			}
		})
	}
}
