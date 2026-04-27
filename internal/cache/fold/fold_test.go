package fold

import "testing"

func TestNameFold(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"COI", "coi"},
		{"coi", "coi"},
		{"ＣＯＩ", "coi"},        // full-width ASCII → half-width + lower
		{"  COI  ", "coi"},     // trim
		{"株式会社", "株式会社"},     // 漢字はそのまま
		{"カイシャ", "カイシャ"},     // カナはそのまま (NFKC は kata→kata)
		{"ｶﾌﾞｼｷ", "カブシキ"},      // half-width katakana → full-width
		{"", ""},
		{" \t\n ", ""},
	}
	for _, c := range cases {
		got := NameFold(c.in)
		if got != c.want {
			t.Errorf("NameFold(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestContains(t *testing.T) {
	cases := []struct {
		haystack, needle string
		want             bool
	}{
		// ASCII case fold
		{"弘前大学ＣＯＩ研究推進機構", "coi", true},
		{"弘前大学ＣＯＩ研究推進機構", "COI", true},
		{"弘前大学ＣＯＩ研究推進機構", "ＣＯＩ", true},
		// trim
		{"弘前大学ＣＯＩ研究推進機構", " coi ", true},
		// kanji literal
		{"株式会社A", "株", true},
		{"株式会社A", "会社", true},
		// 漢字↔カナ fold は行わない
		{"会社", "カイシャ", false},
		// 全文空 needle → true
		{"any", "", true},
		{"any", "   ", true},
		// 半角→全角カナ統一
		{"カブシキ", "ｶﾌﾞｼｷ", true},
		{"ｶﾌﾞｼｷ", "カブシキ", true},
		// 不一致
		{"abc", "xyz", false},
	}
	for _, c := range cases {
		got := Contains(c.haystack, c.needle)
		if got != c.want {
			t.Errorf("Contains(%q, %q) = %v, want %v", c.haystack, c.needle, got, c.want)
		}
	}
}
