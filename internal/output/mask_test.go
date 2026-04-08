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
			name:    "empty secrets slice",
			s:       "hello api_key=secret123",
			secrets: []string{},
			want:    "hello api_key=secret123",
		},
		{
			name:    "nil secrets slice",
			s:       "hello api_key=secret123",
			secrets: nil,
			want:    "hello api_key=secret123",
		},
		{
			name:    "mask single secret",
			s:       "x-api-key: mysecretkey",
			secrets: []string{"mysecretkey"},
			want:    "x-api-key: ***",
		},
		{
			name:    "mask multiple secrets",
			s:       "key1=abc123 key2=xyz789",
			secrets: []string{"abc123", "xyz789"},
			want:    "key1=*** key2=***",
		},
		{
			name:    "empty string secret is skipped",
			s:       "hello world",
			secrets: []string{"", "world"},
			want:    "hello ***",
		},
		{
			name:    "secret appears multiple times in string",
			s:       "token=secret token=secret",
			secrets: []string{"secret"},
			want:    "token=*** token=***",
		},
		{
			name:    "no match",
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
