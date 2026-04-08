// Package cli は board CLI のコマンド定義を提供する。
package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/youyo/board/internal/config"
)

// NewConfigureCmd は configure コマンドを返す。
// サブコマンド未指定の場合は対話式設定を実行する。
func NewConfigureCmd() *cobra.Command {
	var profileFlag string
	cmd := &cobra.Command{
		Use:   "configure",
		Short: "設定を管理する",
		Long:  "board CLI の設定を管理する。サブコマンドなしで実行すると対話式設定を開始する。",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigure(cmd, cmd.InOrStdin(), profileFlag)
		},
	}
	cmd.Flags().StringVarP(&profileFlag, "profile", "p", "", "対象プロファイル名")

	cmd.AddCommand(
		NewConfigureSetCmd(),
		NewConfigureGetCmd(),
		NewConfigureShowCmd(),
		NewConfigureListProfilesCmd(),
		NewConfigureUseCmd(),
		NewConfigureCurrentProfileCmd(),
		NewConfigurePathCmd(),
	)
	return cmd
}

// runConfigure は対話式設定を実行する。
// in を io.Reader で受け取ることでテスト時に差し替え可能にする。
func runConfigure(cmd *cobra.Command, in io.Reader, profileName string) error {
	path := config.ConfigPath()
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}

	if profileName == "" {
		profileName = cfg.CurrentProfile
	}

	// 既存プロファイルを取得（なければデフォルト値）
	prof, ok := cfg.Profiles[profileName]
	if !ok {
		prof = config.DefaultProfileConfig()
	} else {
		prof = config.ApplyDefaults(prof)
	}

	scanner := bufio.NewScanner(in)

	// 質問ヘルパー: 空入力の場合は defaultVal を返す
	ask := func(prompt string, defaultVal string) string {
		fmt.Fprintf(cmd.OutOrStdout(), "%s [%s]: ", prompt, defaultVal)
		if scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				return line
			}
		}
		return defaultVal
	}

	// 質問1: プロファイル名
	newProfileName := ask("Profile name", profileName)

	// 質問2: base_url
	prof.BaseURL = ask("Base URL", prof.BaseURL)

	// 質問3: api_key（現在値はマスク表示）
	maskedKey := maskSecret(prof.APIKey)
	fmt.Fprintf(cmd.OutOrStdout(), "API Key [%s]: ", maskedKey)
	if scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			prof.APIKey = line
		}
	}

	// 質問4: api_token（現在値はマスク表示）
	maskedToken := maskSecret(prof.APIToken)
	fmt.Fprintf(cmd.OutOrStdout(), "API Token [%s]: ", maskedToken)
	if scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			prof.APIToken = line
		}
	}

	// 質問5: daily_auto_refresh
	darDefault := "true"
	if !prof.DailyAutoRefresh {
		darDefault = "false"
	}
	darStr := ask("Daily auto refresh (true/false)", darDefault)
	prof.DailyAutoRefresh = darStr == "true" || darStr == "1" || darStr == "yes"

	// 質問6: timezone（グローバル）
	cfg.Timezone = ask("Timezone", cfg.Timezone)

	// 質問7: current profile にするか
	setCurrentStr := ask(fmt.Sprintf("Set %q as current profile? (yes/no)", newProfileName), "yes")
	setAsCurrent := setCurrentStr == "yes" || setCurrentStr == "y" || setCurrentStr == "true"

	// 保存
	config.AddOrUpdateProfile(&cfg, newProfileName, prof)
	if setAsCurrent {
		config.SetCurrentProfile(&cfg, newProfileName)
	}

	if err := config.Save(cfg, path); err != nil {
		return err
	}

	// 結果表示
	out := map[string]interface{}{
		"profile": newProfileName,
		"config": map[string]interface{}{
			"base_url":                prof.BaseURL,
			"api_key":                 maskSecret(prof.APIKey),
			"api_token":               maskSecret(prof.APIToken),
			"daily_auto_refresh":      prof.DailyAutoRefresh,
			"request_timeout_seconds": prof.RequestTimeoutSeconds,
			"retry_max":               prof.RetryMax,
			"pretty_default":          prof.PrettyDefault,
		},
		"global": map[string]interface{}{
			"current_profile": cfg.CurrentProfile,
			"timezone":        cfg.Timezone,
		},
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(b))
	return nil
}

// MaskSecret は maskSecret のエクスポート版（テスト用）。
func MaskSecret(s string) string {
	return maskSecret(s)
}

// maskSecret はシークレット文字列をマスクする。
//
// マスク仕様:
//   - 長さ 0: "" (空文字)
//   - 長さ 1〜3: "****" (固定4文字マスク)
//   - 長さ 4〜7: 先頭1文字 + "****" + 末尾1文字
//   - 長さ 8〜: 先頭2文字 + "****" + 末尾2文字
func maskSecret(s string) string {
	n := len(s)
	switch {
	case n == 0:
		return ""
	case n <= 3:
		return "****"
	case n <= 7:
		return string(s[0]) + "****" + string(s[n-1])
	default:
		return s[:2] + "****" + s[n-2:]
	}
}
