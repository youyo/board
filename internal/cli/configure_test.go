package cli_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/youyo/board/internal/cli"
	"github.com/youyo/board/internal/config"
)

func TestConfigureInteractive(t *testing.T) {
	t.Run("answering all questions saves to config.toml", func(t *testing.T) {
		path := newTempConfig(t)

		// stdin mock: responses to each prompt
		input := strings.Join([]string{
			"myprofile",           // profile name
			"https://example.com", // base_url
			"newapikey",           // api_key
			"newapitoken",         // api_token
			"true",                // daily_auto_refresh
			"Asia/Tokyo",          // timezone
			"yes",                 // set as current
			"",
		}, "\n")

		root := cli.NewConfigureCmd()
		root.SetIn(strings.NewReader(input))
		buf := &strings.Builder{}
		root.SetOut(buf)
		root.SetErr(&strings.Builder{})
		root.SetArgs([]string{})
		if _, err := root.ExecuteC(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		saved, err := config.Load(path)
		if err != nil {
			t.Fatalf("load failed: %v", err)
		}
		if saved.CurrentProfile != "myprofile" {
			t.Errorf("expected current_profile=myprofile, got %q", saved.CurrentProfile)
		}
		if saved.Timezone != "Asia/Tokyo" {
			t.Errorf("expected timezone=Asia/Tokyo, got %q", saved.Timezone)
		}
		prof, ok := saved.Profiles["myprofile"]
		if !ok {
			t.Fatal("myprofile not found")
		}
		if prof.BaseURL != "https://example.com" {
			t.Errorf("expected base_url=https://example.com, got %q", prof.BaseURL)
		}
		if prof.APIKey != "newapikey" {
			t.Errorf("expected api_key=newapikey, got %q", prof.APIKey)
		}
		if prof.APIToken != "newapitoken" {
			t.Errorf("expected api_token=newapitoken, got %q", prof.APIToken)
		}
	})

	t.Run("skipping with empty input preserves existing values", func(t *testing.T) {
		path := newTempConfig(t)

		// First, set up initial values.
		cfg := config.DefaultConfig()
		prof := config.DefaultProfileConfig()
		prof.BaseURL = "https://existing.example.com"
		prof.APIKey = "existingkey"
		config.AddOrUpdateProfile(&cfg, "default", prof)
		cfg.Timezone = "America/New_York"
		if err := config.Save(cfg, path); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		// Skip all prompts with empty input.
		input := "\n\n\n\n\n\n\n"

		root := cli.NewConfigureCmd()
		root.SetIn(strings.NewReader(input))
		root.SetOut(&strings.Builder{})
		root.SetErr(&strings.Builder{})
		root.SetArgs([]string{})
		if _, err := root.ExecuteC(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		saved, err := config.Load(path)
		if err != nil {
			t.Fatalf("load failed: %v", err)
		}
		savedProf, ok := saved.Profiles["default"]
		if !ok {
			t.Fatal("default profile not found")
		}
		if savedProf.BaseURL != "https://existing.example.com" {
			t.Errorf("expected existing base_url, got %q", savedProf.BaseURL)
		}
		if saved.Timezone != "America/New_York" {
			t.Errorf("expected timezone=America/New_York, got %q", saved.Timezone)
		}
	})

	t.Run("entering a new profile name creates a new profile", func(t *testing.T) {
		path := newTempConfig(t)

		input := strings.Join([]string{
			"brandnew",        // profile name
			"https://new.com", // base_url
			"",                // api_key (skip)
			"",                // api_token (skip)
			"",                // daily_auto_refresh (skip)
			"",                // timezone (skip)
			"yes",             // set as current
			"",
		}, "\n")

		root := cli.NewConfigureCmd()
		root.SetIn(strings.NewReader(input))
		root.SetOut(&strings.Builder{})
		root.SetErr(&strings.Builder{})
		root.SetArgs([]string{})
		if _, err := root.ExecuteC(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		saved, err := config.Load(path)
		if err != nil {
			t.Fatalf("load failed: %v", err)
		}
		if _, ok := saved.Profiles["brandnew"]; !ok {
			t.Fatal("brandnew profile should have been created")
		}
		if saved.CurrentProfile != "brandnew" {
			t.Errorf("expected current_profile=brandnew, got %q", saved.CurrentProfile)
		}
	})

	t.Run("interactive output contains JSON with masked secrets", func(t *testing.T) {
		newTempConfig(t)

		input := strings.Join([]string{
			"default",        // profile name
			"",               // base_url (skip)
			"mysecretapikey", // api_key
			"mysecrettoken",  // api_token
			"",               // daily_auto_refresh (skip)
			"",               // timezone (skip)
			"yes",            // set as current
			"",
		}, "\n")

		root := cli.NewConfigureCmd()
		root.SetIn(strings.NewReader(input))
		buf := &strings.Builder{}
		root.SetOut(buf)
		root.SetErr(&strings.Builder{})
		root.SetArgs([]string{})
		if _, err := root.ExecuteC(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		out := buf.String()
		// The interactive output includes prompt strings, so parse JSON starting from '{'.
		jsonStart := strings.Index(out, "{")
		if jsonStart == -1 {
			t.Fatalf("no JSON found in output: %s", out)
		}
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(out[jsonStart:]), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v\noutput: %s", err, out)
		}

		cfgMap, _ := result["config"].(map[string]interface{})
		apiKey, _ := cfgMap["api_key"].(string)
		if strings.Contains(apiKey, "mysecretapikey") {
			t.Errorf("api_key should be masked in output, got %q", apiKey)
		}

		fmt.Printf("interactive output api_key: %q\n", apiKey)
	})
}
