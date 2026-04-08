package output

import "strings"

// MaskSecrets replaces all occurrences of each secret in s with "***" and returns the result.
// Empty-string secrets are skipped.
func MaskSecrets(s string, secrets []string) string {
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		s = strings.ReplaceAll(s, secret, "***")
	}
	return s
}
