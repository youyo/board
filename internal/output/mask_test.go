package output_test

import (
	"testing"

	"github.com/youyo/board/internal/output"
)

func TestMaskSecrets(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		secrets []string
		want    string
	}{
		{
			name:    "空のsecrets",
			s:       "hello api_key=secret123",
			secrets: []string{},
			want:    "hello api_key=secret123",
		},
		{
			name:    "nilのsecrets",
			s:       "hello api_key=secret123",
			secrets: nil,
			want:    "hello api_key=secret123",
		},
		{
			name:    "単一のsecretをマスク",
			s:       "x-api-key: mysecretkey",
			secrets: []string{"mysecretkey"},
			want:    "x-api-key: ***",
		},
		{
			name:    "複数のsecretをマスク",
			s:       "key1=abc123 key2=xyz789",
			secrets: []string{"abc123", "xyz789"},
			want:    "key1=*** key2=***",
		},
		{
			name:    "空文字列のsecretはスキップ",
			s:       "hello world",
			secrets: []string{"", "world"},
			want:    "hello ***",
		},
		{
			name:    "secretが文中に複数回出現",
			s:       "token=secret token=secret",
			secrets: []string{"secret"},
			want:    "token=*** token=***",
		},
		{
			name:    "マッチなし",
			s:       "no secrets here",
			secrets: []string{"notpresent"},
			want:    "no secrets here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := output.MaskSecrets(tt.s, tt.secrets)
			if got != tt.want {
				t.Errorf("MaskSecrets() = %q, want %q", got, tt.want)
			}
		})
	}
}
