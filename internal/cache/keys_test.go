package cache

import "testing"

// T_KY01: NewEntityKey が正しいフィールドを設定する
func TestNewEntityKey(t *testing.T) {
	key := NewEntityKey("default", "clients", "42")
	if key.Profile != "default" {
		t.Errorf("Profile: got %q, want %q", key.Profile, "default")
	}
	if key.Resource != "clients" {
		t.Errorf("Resource: got %q, want %q", key.Resource, "clients")
	}
	if key.EntityID != "42" {
		t.Errorf("EntityID: got %q, want %q", key.EntityID, "42")
	}
}

// T_KY02: EntityKey はゼロ値で生成できる
func TestEntityKey_Zero(t *testing.T) {
	var key EntityKey
	if key.Profile != "" || key.Resource != "" || key.EntityID != "" {
		t.Error("zero value EntityKey should have empty fields")
	}
}
