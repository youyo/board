// Package cli provides command definitions for the board CLI.
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

// NewConfigureCmd returns the configure command.
// When invoked without a subcommand, it runs the interactive configuration wizard.
func NewConfigureCmd() *cobra.Command {
	var profileFlag string
	cmd := &cobra.Command{
		Use:   "configure",
		Short: "Manage configuration",
		Long:  "Manage board CLI configuration. Running without a subcommand starts the interactive setup.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigure(cmd, cmd.InOrStdin(), profileFlag)
		},
	}
	cmd.Flags().StringVarP(&profileFlag, "profile", "p", "", "Target profile name")

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

// runConfigure runs the interactive configuration wizard.
// in is accepted as io.Reader so it can be replaced during testing.
func runConfigure(cmd *cobra.Command, in io.Reader, profileName string) error {
	path := config.ConfigPath()
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}

	if profileName == "" {
		profileName = cfg.CurrentProfile
	}

	// Retrieve the existing profile, or use defaults if it does not exist.
	prof, ok := cfg.Profiles[profileName]
	if !ok {
		prof = config.DefaultProfileConfig()
	} else {
		prof = config.ApplyDefaults(prof)
	}

	scanner := bufio.NewScanner(in)

	// ask helper: returns defaultVal when input is empty.
	ask := func(prompt string, defaultVal string) string {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s [%s]: ", prompt, defaultVal)
		if scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				return line
			}
		}
		return defaultVal
	}

	// Question 1: profile name
	newProfileName := ask("Profile name", profileName)

	// Question 2: base_url
	prof.BaseURL = ask("Base URL", prof.BaseURL)

	// Question 3: api_key (current value is masked)
	maskedKey := maskSecret(prof.APIKey)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "API Key [%s]: ", maskedKey)
	if scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			prof.APIKey = line
		}
	}

	// Question 4: api_token (current value is masked)
	maskedToken := maskSecret(prof.APIToken)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "API Token [%s]: ", maskedToken)
	if scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			prof.APIToken = line
		}
	}

	// Question 5: timezone (global)
	cfg.Timezone = ask("Timezone", cfg.Timezone)

	// Question 6: whether to set as current profile
	setCurrentStr := ask(fmt.Sprintf("Set %q as current profile? (yes/no)", newProfileName), "yes")
	setAsCurrent := setCurrentStr == "yes" || setCurrentStr == "y" || setCurrentStr == "true"

	// Save configuration.
	config.AddOrUpdateProfile(&cfg, newProfileName, prof)
	if setAsCurrent {
		config.SetCurrentProfile(&cfg, newProfileName)
	}

	if err := config.Save(cfg, path); err != nil {
		return err
	}

	// Display the result.
	out := map[string]interface{}{
		"profile": newProfileName,
		"config": map[string]interface{}{
			"base_url":                prof.BaseURL,
			"api_key":                 maskSecret(prof.APIKey),
			"api_token":               maskSecret(prof.APIToken),
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
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
	return nil
}

// MaskSecret is the exported version of maskSecret (for testing).
func MaskSecret(s string) string {
	return maskSecret(s)
}

// maskSecret masks a secret string.
//
// Masking rules:
//   - length 0: "" (empty string)
//   - length 1–3: "****" (fixed 4-character mask)
//   - length 4–7: first 1 char + "****" + last 1 char
//   - length 8+: first 2 chars + "****" + last 2 chars
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
