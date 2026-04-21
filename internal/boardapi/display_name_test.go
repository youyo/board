package boardapi_test

import (
	"testing"

	"github.com/youyo/board/internal/boardapi"
)

func TestUserEntity_DisplayName(t *testing.T) {
	tests := []struct {
		name   string
		entity boardapi.UserEntity
		want   string
	}{
		{
			name:   "Name only",
			entity: boardapi.UserEntity{Name: "Taro Yamada"},
			want:   "Taro Yamada",
		},
		{
			name:   "LastName and FirstName",
			entity: boardapi.UserEntity{LastName: "立花", FirstName: "拓也"},
			want:   "立花 拓也",
		},
		{
			name:   "Name takes priority over LastName/FirstName",
			entity: boardapi.UserEntity{Name: "Display", LastName: "Last", FirstName: "First"},
			want:   "Display",
		},
		{
			name:   "LastName only",
			entity: boardapi.UserEntity{LastName: "立花"},
			want:   "立花",
		},
		{
			name:   "FirstName only",
			entity: boardapi.UserEntity{FirstName: "拓也"},
			want:   "拓也",
		},
		{
			name:   "all empty",
			entity: boardapi.UserEntity{},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entity.DisplayName(); got != tt.want {
				t.Errorf("DisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestContactEntity_DisplayName(t *testing.T) {
	tests := []struct {
		name   string
		entity boardapi.ContactEntity
		want   string
	}{
		{
			name:   "LastName and FirstName",
			entity: boardapi.ContactEntity{LastName: "佐々木", FirstName: "昌代"},
			want:   "佐々木 昌代",
		},
		{
			name:   "LastName only",
			entity: boardapi.ContactEntity{LastName: "佐々木"},
			want:   "佐々木",
		},
		{
			name:   "FirstName only",
			entity: boardapi.ContactEntity{FirstName: "昌代"},
			want:   "昌代",
		},
		{
			name:   "all empty",
			entity: boardapi.ContactEntity{},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entity.DisplayName(); got != tt.want {
				t.Errorf("DisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestVendorContactEntity_DisplayName は M42 再設計後の DisplayName() を検証する。
// Name フィールドは廃止（ContactEntity と同様）。LastName + FirstName のみで構成。
func TestVendorContactEntity_DisplayName(t *testing.T) {
	tests := []struct {
		name   string
		entity boardapi.VendorContactEntity
		want   string
	}{
		{
			name:   "LastName and FirstName",
			entity: boardapi.VendorContactEntity{LastName: "田中", FirstName: "太郎"},
			want:   "田中 太郎",
		},
		{
			name:   "LastName only",
			entity: boardapi.VendorContactEntity{LastName: "田中"},
			want:   "田中",
		},
		{
			name:   "FirstName only",
			entity: boardapi.VendorContactEntity{FirstName: "太郎"},
			want:   "太郎",
		},
		{
			name:   "all empty",
			entity: boardapi.VendorContactEntity{},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entity.DisplayName(); got != tt.want {
				t.Errorf("DisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}
