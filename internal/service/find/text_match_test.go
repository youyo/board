package find

import "testing"

// T01: 大文字小文字を区別せずマッチする
func TestContainsText_CaseInsensitiveMatch(t *testing.T) {
	if !containsText("abc", "ABC Corp") {
		t.Fatal("want true, got false")
	}
}

// T02: fields が空の場合は false
func TestContainsText_EmptyFields_ReturnsFalse(t *testing.T) {
	if containsText("x") {
		t.Fatal("want false, got true")
	}
}

// T03: nil 相当（空文字）の *string は "" を返す
func TestDerefString_NilInput_ReturnsEmpty(t *testing.T) {
	if got := derefString(nil); got != "" {
		t.Fatalf("want \"\", got %q", got)
	}
}

// 追加: TrimSpace により空白のみの text は false
func TestContainsText_WhitespaceOnly_ReturnsFalse(t *testing.T) {
	if containsText("   ", "hello") {
		t.Fatal("want false for whitespace-only text")
	}
}

// 追加: derefString は非 nil の *string を正しく返す
func TestDerefString_NonNil_ReturnsValue(t *testing.T) {
	s := "hello"
	if got := derefString(&s); got != "hello" {
		t.Fatalf("want \"hello\", got %q", got)
	}
}
