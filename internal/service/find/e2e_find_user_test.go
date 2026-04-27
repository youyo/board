//go:build e2e

package find_test

import (
	"context"
	"testing"
	"time"

	"github.com/youyo/board/internal/service/find"
)

// T41: NameCont (text-OR) 正常系
func TestE2E_FindUser_ByName_Returns_NonEmpty(t *testing.T) {
	svc := newE2EService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rs, err := svc.FindUser(ctx, find.FindUserQuery{
		FindCommonOpts: find.FindCommonOpts{Limit: 5},
		Name:           "株", // BOARD には users が必ず最低 1 件存在する想定
	})
	if skipIfRateLimit(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("FindUser: %v", err)
	}
	skipIfNoData(t, "users", len(rs), 1)
	t.Logf("users count=%d first.id=%d", len(rs), rs[0].User.ID)
}
