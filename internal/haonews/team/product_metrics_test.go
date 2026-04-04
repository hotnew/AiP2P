package team

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreCountTasksByStatusCtxUsesIndex(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store, err := OpenStore(root)
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}
	teamRoot := filepath.Join(root, "team", "metrics-index")
	if err := os.MkdirAll(teamRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(teamRoot, "team.json"), []byte(`{"team_id":"metrics-index","title":"Metrics Index"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(team.json) error = %v", err)
	}
	now := time.Now().UTC()
	for i, status := range []string{"open", "doing", "done"} {
		if err := store.AppendTaskCtx(context.Background(), "metrics-index", Task{
			TaskID:    "task-" + status,
			TeamID:    "metrics-index",
			Title:     status,
			Status:    status,
			CreatedBy: "agent://pc75/openclaw01",
			CreatedAt: now.Add(time.Duration(i) * time.Second),
			UpdatedAt: now.Add(time.Duration(i) * time.Second),
		}); err != nil {
			t.Fatalf("AppendTaskCtx(%s) error = %v", status, err)
		}
	}

	counts, err := store.CountTasksByStatusCtx(context.Background(), "metrics-index")
	if err != nil {
		t.Fatalf("CountTasksByStatusCtx error = %v", err)
	}
	if counts["open"] != 1 || counts["doing"] != 1 || counts["done"] != 1 {
		t.Fatalf("unexpected counts = %#v", counts)
	}
}

func TestStoreCountRecentMessagesAndRecentDeliveriesCtx(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store, err := OpenStore(root)
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}
	teamRoot := filepath.Join(root, "team", "metrics-recent")
	if err := os.MkdirAll(teamRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(teamRoot, "team.json"), []byte(`{"team_id":"metrics-recent","title":"Metrics Recent"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(team.json) error = %v", err)
	}

	oldAt := time.Now().UTC().Add(-5 * 24 * time.Hour)
	newAt := time.Now().UTC().Add(-2 * time.Hour)
	for _, item := range []Message{
		{TeamID: "metrics-recent", ChannelID: "main", AuthorAgentID: "agent://pc75/a", MessageType: "chat", Content: "old", CreatedAt: oldAt},
		{TeamID: "metrics-recent", ChannelID: "main", AuthorAgentID: "agent://pc75/a", MessageType: "chat", Content: "new", CreatedAt: newAt},
	} {
		if err := store.AppendMessageCtx(context.Background(), "metrics-recent", item); err != nil {
			t.Fatalf("AppendMessageCtx error = %v", err)
		}
	}
	count, err := store.CountRecentMessagesCtx(context.Background(), "metrics-recent", "main", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("CountRecentMessagesCtx error = %v", err)
	}
	if count != 1 {
		t.Fatalf("CountRecentMessagesCtx = %d, want 1", count)
	}

	records := []WebhookDeliveryRecord{
		{DeliveryID: "old", TeamID: "metrics-recent", URL: "http://127.0.0.1/old", Status: webhookDeliveryStatusFailed, CreatedAt: oldAt, UpdatedAt: oldAt},
		{DeliveryID: "new", TeamID: "metrics-recent", URL: "http://127.0.0.1/new", Status: webhookDeliveryStatusDeadLetter, CreatedAt: newAt, UpdatedAt: newAt},
	}
	if err := store.saveWebhookDeliveriesLocked("metrics-recent", records); err != nil {
		t.Fatalf("saveWebhookDeliveriesLocked error = %v", err)
	}
	recent, err := store.LoadRecentDeliveriesCtx(context.Background(), "metrics-recent", 1)
	if err != nil {
		t.Fatalf("LoadRecentDeliveriesCtx error = %v", err)
	}
	if len(recent) != 1 || recent[0].DeliveryID != "new" {
		t.Fatalf("LoadRecentDeliveriesCtx = %#v, want newest only", recent)
	}
}
