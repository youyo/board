package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/youyo/board/internal/cli"
	"github.com/youyo/board/internal/config"
)

func TestConfigureShowCmd(t *testing.T) {
	t.Run("デフォルト config では secrets がマスクされた JSON が返る", func(t *testing.T) {
		path := newTempConfig(t)

		// api_key と api_token を設定
		cfg := config.DefaultConfig()
		prof := config.DefaultProfileConfig()
		prof.APIKey = "myapikey12345"
		prof.APIToken = "myapitoken123"
		config.AddOrUpdateProfile(&cfg, "default", prof)
		if err := config.Save(cfg, path); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		root := cli.NewConfigureCmd()
		out, err := executeCmd(t, root, "show")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v\noutput: %s", err, out)
		}

		cfgMap, ok := result["config"].(map[string]interface{})
		if !ok {
			t.Fatal("config field not found or not a map")
		}

		apiKey, _ := cfgMap["api_key"].(string)
		if strings.Contains(apiKey, "myapikey12345") {
			t.Errorf("api_key should be masked, got %q", apiKey)
		}
		if apiKey == "" && len("myapikey12345") > 0 {
			// 空でないはず
			t.Errorf("api_key should not be empty when value is set")
		}
		// マスクパターン確認: 先頭2文字 + **** + 末尾2文字
		if apiKey != "my****45" {
			t.Errorf("expected my****45, got %q", apiKey)
		}

		apiToken, _ := cfgMap["api_token"].(string)
		if apiToken != "my****23" {
			t.Errorf("expected my****23, got %q", apiToken)
		}
	})

	t.Run("--profile 指定でそのプロファイルの情報が表示される", func(t *testing.T) {
		path := newTempConfig(t)

		cfg := config.DefaultConfig()
		staging := config.DefaultProfileConfig()
		staging.BaseURL = "https://staging.example.com"
		config.AddOrUpdateProfile(&cfg, "staging", staging)
		if err := config.Save(cfg, path); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		root := cli.NewConfigureCmd()
		out, err := executeCmd(t, root, "show", "--profile", "staging")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if result["profile"] != "staging" {
			t.Errorf("expected profile=staging, got %v", result["profile"])
		}

		cfgMap, _ := result["config"].(map[string]interface{})
		if cfgMap["base_url"] != "https://staging.example.com" {
			t.Errorf("expected staging base_url, got %v", cfgMap["base_url"])
		}
	})

	t.Run("存在しないプロファイルを指定するとエラー", func(t *testing.T) {
		newTempConfig(t)

		root := cli.NewConfigureCmd()
		_, err := executeCmd(t, root, "show", "--profile", "nonexistent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "profile not found") {
			t.Errorf("expected profile not found error, got: %v", err)
		}
	})
}
