package aip2p

import "testing"

func TestReserveDailyQuota(t *testing.T) {
	t.Parallel()

	counts := map[string]int64{}
	createdAt := "2026-03-13T12:00:00Z"
	if !reserveDailyQuota(counts, createdAt, 1) {
		t.Fatal("expected first item to be allowed")
	}
	if reserveDailyQuota(counts, createdAt, 1) {
		t.Fatal("expected second item on same utc day to be rejected")
	}
}
