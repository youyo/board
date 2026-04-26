package find

import "testing"

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
