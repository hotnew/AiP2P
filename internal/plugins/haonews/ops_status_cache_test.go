package newsplugin

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestNodeStatusReturnsStaleWhileRefreshing(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	refreshStarted := make(chan struct{}, 1)
	releaseRefresh := make(chan struct{})

	app := &App{
		buildNodeStatusFn: func(Index) NodeStatus {
			call := calls.Add(1)
			if call == 1 {
				return NodeStatus{Summary: "fresh-1"}
			}
			select {
			case refreshStarted <- struct{}{}:
			default:
			}
			<-releaseRefresh
			return NodeStatus{Summary: "fresh-2"}
		},
	}

	first := app.nodeStatus(Index{})
	if first.Summary != "fresh-1" {
		t.Fatalf("first summary = %q, want fresh-1", first.Summary)
	}

	app.nodeStatusMu.Lock()
	app.nodeStatusCache.expiresAt = time.Now().Add(-time.Second)
	app.nodeStatusMu.Unlock()

	start := time.Now()
	stale := app.nodeStatus(Index{})
	if stale.Summary != "fresh-1" {
		t.Fatalf("stale summary = %q, want fresh-1", stale.Summary)
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Fatalf("stale read blocked too long: %s", time.Since(start))
	}

	select {
	case <-refreshStarted:
	case <-time.After(time.Second):
		t.Fatal("expected async refresh to start")
	}

	again := app.nodeStatus(Index{})
	if again.Summary != "fresh-1" {
		t.Fatalf("concurrent stale summary = %q, want fresh-1", again.Summary)
	}

	close(releaseRefresh)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		current := app.nodeStatus(Index{})
		if current.Summary == "fresh-2" {
			if got := calls.Load(); got != 2 {
				t.Fatalf("build calls = %d, want 2", got)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected refreshed node status")
}
