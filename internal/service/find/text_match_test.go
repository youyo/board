package find

import "testing"

func TestContainsText(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		fields []string
		want   bool
	}{
		{"exact match", "hello", []string{"hello"}, true},
		{"case insensitive", "HELLO", []string{"hello world"}, true},
		{"substring", "ell", []string{"hello"}, true},
		{"no match", "xyz", []string{"hello", "world"}, false},
		{"match in second field", "world", []string{"hello", "world"}, true},
		{"empty text", "", []string{"hello"}, true},
		{"empty fields", "hello", []string{""}, false},
		{"both empty", "", []string{""}, true},
		{"japanese text", "株式", []string{"株式会社ABC"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsText(tt.text, tt.fields...)
			if got != tt.want {
				t.Errorf("containsText(%q, %v) = %v, want %v", tt.text, tt.fields, got, tt.want)
			}
		})
	}
}
