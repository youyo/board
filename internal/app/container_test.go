package app_test

import (
	"reflect"
	"testing"
)

func TestRepositories_AllNonNil(t *testing.T) {
	a := newTestApp(t)
	repos := a.Repos

	v := reflect.ValueOf(*repos)
	typ := v.Type()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.IsNil() {
			t.Errorf("Repositories.%s is nil", typ.Field(i).Name)
		}
	}
}

func TestRepositories_FieldCount(t *testing.T) {
	a := newTestApp(t)
	repos := a.Repos

	v := reflect.ValueOf(*repos)
	const wantFields = 22
	if v.NumField() != wantFields {
		t.Errorf("Repositories has %d fields, want %d", v.NumField(), wantFields)
	}
}
