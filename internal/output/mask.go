package output

import "strings"

// MaskSecrets は文字列 s 中の secrets をすべて "***" に置換して返す。
// 空文字列の secret はスキップする。
func MaskSecrets(s string, secrets []string) string {
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		s = strings.ReplaceAll(s, secret, "***")
	}
	return s
}
