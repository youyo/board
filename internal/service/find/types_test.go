package find

import (
	"strings"
	"testing"
)

// T15: Status と Statuses が両方セットされている場合は error
func TestQueryValidate_StatusAndStatusesBothSet_Error(t *testing.T) {
	q := FindProjectQuery{
		Status:   "x",
		Statuses: []string{"y"},
	}
	err := q.validate()
	assertError(t, err)
}

// T15a: Limit=0 は無制限扱いで error なし
func TestFindCommonOpts_LimitZero_AllowedAsUnlimited(t *testing.T) {
	o := FindCommonOpts{Limit: 0}
	assertNoError(t, o.validate())
}

// T15b: Limit=-1 は validation error
func TestFindCommonOpts_LimitNegative_Rejected(t *testing.T) {
	o := FindCommonOpts{Limit: -1}
	assertError(t, o.validate())
}

// T16: 全フィールドがゼロ値の Query は error（at least one field required）
func TestQueryValidate_EmptyQuery_Error(t *testing.T) {
	q := FindProjectQuery{}
	err := q.validate()
	assertError(t, err)
}

// 追加: validateQuery は common.validate() が先に動く
func TestValidateQuery_CommonFails_ReturnsEarly(t *testing.T) {
	common := FindCommonOpts{Limit: -1}
	specific := FindClientQuery{Name: "test"}
	err := validateQuery(common, specific)
	assertError(t, err)
}

// 追加: Statuses が 11 要素で error
func TestQueryValidate_ElevenStatuses_Error(t *testing.T) {
	statuses := make([]string, 11)
	for i := range statuses {
		statuses[i] = "x"
	}
	q := FindProjectQuery{Statuses: statuses}
	err := q.validate()
	assertError(t, err)
}

// 追加: Statuses が 10 要素は OK（N05 advisor R3: Status/Statuses は narrowing 必須のため Name を添える）
func TestQueryValidate_TenStatuses_OK(t *testing.T) {
	statuses := make([]string, 10)
	for i := range statuses {
		statuses[i] = "x"
	}
	q := FindProjectQuery{Name: "proj", Statuses: statuses}
	err := q.validate()
	assertNoError(t, err)
}

// --- V-T01〜V-T10: ContractStatus 3-way 排他 + validate 拡張テスト ---

// V-T01: Status + ContractStatus 両方セットは error (mutually exclusive)
func TestQueryValidate_StatusAndContractStatusBothSet_Error(t *testing.T) {
	q := FindProjectQuery{
		Name:           "x",
		Status:         "受注済",
		ContractStatus: "active",
	}
	err := q.validate()
	assertError(t, err)
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected 'mutually exclusive' in error, got: %v", err)
	}
}

// V-T02: Statuses + ContractStatus 両方セットは error (mutually exclusive)
func TestQueryValidate_StatusesAndContractStatusBothSet_Error(t *testing.T) {
	q := FindProjectQuery{
		Name:           "x",
		Statuses:       []string{"受注済"},
		ContractStatus: "active",
	}
	err := q.validate()
	assertError(t, err)
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected 'mutually exclusive' in error, got: %v", err)
	}
}

// V-T03: Status + Statuses + ContractStatus 全部セットは error (mutually exclusive)
func TestQueryValidate_AllThreeStatusFieldsSet_Error(t *testing.T) {
	q := FindProjectQuery{
		Name:           "x",
		Status:         "受注済",
		Statuses:       []string{"納品済"},
		ContractStatus: "active",
	}
	err := q.validate()
	assertError(t, err)
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected 'mutually exclusive' in error, got: %v", err)
	}
}

// V-T04: ContractStatus のみ（narrow なし）は error (requires at least one of)
func TestQueryValidate_ContractStatusOnly_Error(t *testing.T) {
	q := FindProjectQuery{
		ContractStatus: "active",
	}
	err := q.validate()
	assertError(t, err)
	if !strings.Contains(err.Error(), "requires at least one of") {
		t.Errorf("expected 'requires at least one of' in error, got: %v", err)
	}
}

// V-T05: ContractStatus + Name → nil error
func TestQueryValidate_ContractStatusWithName_OK(t *testing.T) {
	q := FindProjectQuery{
		Name:           "保守",
		ContractStatus: "active",
	}
	assertNoError(t, q.validate())
}

// V-T06: ContractStatus + ClientID → nil error
func TestQueryValidate_ContractStatusWithClientID_OK(t *testing.T) {
	q := FindProjectQuery{
		ClientID:       5,
		ContractStatus: "active",
	}
	assertNoError(t, q.validate())
}

// V-T07: ContractStatus + ID → nil error (ID 指定は narrowing として有効)
func TestQueryValidate_ContractStatusWithID_OK(t *testing.T) {
	q := FindProjectQuery{
		ID:             10,
		ContractStatus: "active",
	}
	assertNoError(t, q.validate())
}

// V-T09: ContractStatus が " ACTIVE " (trim + case-insensitive) でも nil error
func TestQueryValidate_ContractStatusTrimAndCase_OK(t *testing.T) {
	q := FindProjectQuery{
		Name:           "x",
		ContractStatus: " ACTIVE ",
	}
	assertNoError(t, q.validate())
}

// V-T10: regression — Status-only（ContractStatus なし）は依然 "requires at least one of" error
func TestQueryValidate_StatusOnly_Regression_Error(t *testing.T) {
	q := FindProjectQuery{
		Status: "受注済",
	}
	err := q.validate()
	assertError(t, err)
	if !strings.Contains(err.Error(), "requires at least one of") {
		t.Errorf("expected 'requires at least one of' in error, got: %v", err)
	}
}
