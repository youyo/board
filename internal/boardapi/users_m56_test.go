package boardapi_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/youyo/board/internal/boardapi"
)

// newUsersMockClientM56 は M56 ユニットテスト用のモッククライアントを返す。
func newUsersMockClientM56(rt roundTripperFunc) *boardapi.Client {
	hc := &http.Client{Transport: rt, Timeout: 5 * time.Second}
	return boardapi.New("https://mock.example.test", "test-key", "test-token", 5*time.Second,
		boardapi.WithHTTPClient(hc),
		boardapi.WithRetryMax(0),
	)
}

// M56_U1: buildUsersQuery でゼロフィルタ時は page/per_page のみ送信される。
func TestBuildUsersQuery_ZeroFilter(t *testing.T) {
	var capturedQuery url.Values
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		capturedQuery = r.URL.Query()
		return jsonResp(`[]`), nil
	})
	client := newUsersMockClientM56(rt)

	_, err := client.ListUsers(context.Background(), boardapi.UserListOptions{})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if capturedQuery.Get("name_cont") != "" {
		t.Errorf("name_cont should not be sent for zero filter, got %q", capturedQuery.Get("name_cont"))
	}
	if capturedQuery.Get("email_cont") != "" {
		t.Errorf("email_cont should not be sent for zero filter, got %q", capturedQuery.Get("email_cont"))
	}
}

// M56_U2: buildUsersQuery で NameCont が name_cont として送信される。
func TestBuildUsersQuery_NameCont(t *testing.T) {
	var observedNameCont string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		observedNameCont = r.URL.Query().Get("name_cont")
		return jsonResp(`[]`), nil
	})
	client := newUsersMockClientM56(rt)

	_, err := client.ListUsers(context.Background(), boardapi.UserListOptions{
		NameCont: "山田",
	})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if observedNameCont != "山田" {
		t.Errorf("name_cont = %q, want %q", observedNameCont, "山田")
	}
}

// M56_U3: buildUsersQuery で EmailCont が email_cont として送信される。
func TestBuildUsersQuery_EmailCont(t *testing.T) {
	var observedEmailCont string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		observedEmailCont = r.URL.Query().Get("email_cont")
		return jsonResp(`[]`), nil
	})
	client := newUsersMockClientM56(rt)

	_, err := client.ListUsers(context.Background(), boardapi.UserListOptions{
		EmailCont: "example.com",
	})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if observedEmailCont != "example.com" {
		t.Errorf("email_cont = %q, want %q", observedEmailCont, "example.com")
	}
}

// M56_U4: buildUsersQuery で UpdatedAtGteq が updated_at_gteq として送信される。
func TestBuildUsersQuery_UpdatedAtGteq(t *testing.T) {
	var observedGteq string
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		observedGteq = r.URL.Query().Get("updated_at_gteq")
		return jsonResp(`[]`), nil
	})
	client := newUsersMockClientM56(rt)

	_, err := client.ListUsers(context.Background(), boardapi.UserListOptions{
		UpdatedAtGteq: "2024-01-01 00:00:00",
	})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if observedGteq != "2024-01-01 00:00:00" {
		t.Errorf("updated_at_gteq = %q, want %q", observedGteq, "2024-01-01 00:00:00")
	}
}

// M56_U5: ListUsers は *ListResult[UserEntity] を返す。
func TestListUsers_ReturnsListResult(t *testing.T) {
	page1 := `[{"id":1,"name":"山田 太郎","last_name":"山田","first_name":"太郎","email":"yamada@example.test","role_id":1,"role_name":"Admin","last_sign_in_at":"","valid_flg":1,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}]`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResp(page1), nil
	})
	client := newUsersMockClientM56(rt)

	result, err := client.ListUsers(context.Background(), boardapi.UserListOptions{})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].ID != 1 {
		t.Errorf("item.ID = %d, want 1", result.Items[0].ID)
	}
}

// M56_U6: GetUser は *ItemResult[UserEntity] を返す。
func TestGetUser_ReturnsItemResult(t *testing.T) {
	body := `{"id":42,"name":"Foo","last_name":"","first_name":"","email":"foo@example.test","role_id":1,"role_name":"Admin","last_sign_in_at":"","valid_flg":1,"updated_at":"2024-01-01T00:00:00+09:00","created_at":"2023-01-01T00:00:00+09:00"}`
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResp(body), nil
	})
	client := newUsersMockClientM56(rt)

	result, err := client.GetUser(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if result == nil || result.Item == nil {
		t.Fatal("result or result.Item is nil")
	}
	if result.Item.ID != 42 {
		t.Errorf("item.ID = %d, want 42", result.Item.ID)
	}
}
