package haonewslive

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hao.news/internal/apphost"
	core "hao.news/internal/haonews"
	"hao.news/internal/haonews/live"
	haonewstheme "hao.news/internal/themes/haonews"
)

func TestPluginBuildServesLiveIndex(t *testing.T) {
	t.Parallel()

	site, root := buildLiveSite(t)
	store, err := live.OpenLocalStore(filepath.Join(root, "store"))
	if err != nil {
		t.Fatalf("OpenLocalStore error = %v", err)
	}
	if err := store.SaveRoom(live.RoomInfo{
		RoomID:    "room-1",
		Title:     "Live Test",
		Creator:   "agent://pc75/openclaw01",
		CreatedAt: "2026-03-19T00:00:00Z",
		Channel:   "hao.news/live",
	}); err != nil {
		t.Fatalf("SaveRoom error = %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/live", nil)
	rec := httptest.NewRecorder()
	site.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Live Test") {
		t.Fatalf("expected live room in body, got %q", rec.Body.String())
	}
}

func TestPluginBuildServesLiveAPI(t *testing.T) {
	t.Parallel()

	site, root := buildLiveSite(t)
	store, err := live.OpenLocalStore(filepath.Join(root, "store"))
	if err != nil {
		t.Fatalf("OpenLocalStore error = %v", err)
	}
	if err := store.SaveRoom(live.RoomInfo{
		RoomID:    "room-2",
		Title:     "Room API",
		Creator:   "agent://pc75/openclaw01",
		CreatedAt: "2026-03-19T00:00:00Z",
		Channel:   "hao.news/live",
	}); err != nil {
		t.Fatalf("SaveRoom error = %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/live/rooms", nil)
	rec := httptest.NewRecorder()
	site.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "room-2") {
		t.Fatalf("expected room id in API body, got %q", rec.Body.String())
	}
}

func TestPluginBuildServesLiveRoomDetails(t *testing.T) {
	t.Parallel()

	site, root := buildLiveSite(t)
	store, err := live.OpenLocalStore(filepath.Join(root, "store"))
	if err != nil {
		t.Fatalf("OpenLocalStore error = %v", err)
	}
	room := live.RoomInfo{
		RoomID:    "room-3",
		Title:     "Room Detail",
		Creator:   "agent://pc75/openclaw01",
		CreatedAt: "2026-03-19T00:00:00Z",
		Channel:   "hao.news/live",
	}
	if err := store.SaveRoom(room); err != nil {
		t.Fatalf("SaveRoom error = %v", err)
	}
	if err := store.AppendEvent(room.RoomID, live.LiveMessage{
		Protocol:     live.ProtocolVersion,
		Type:         live.TypeHeartbeat,
		RoomID:       room.RoomID,
		Sender:       room.Creator,
		SenderPubKey: strings.Repeat("a", 64),
		Seq:          0,
		Timestamp:    "2026-03-19T00:00:05Z",
	}); err != nil {
		t.Fatalf("AppendEvent heartbeat error = %v", err)
	}
	if err := store.AppendEvent(room.RoomID, live.LiveMessage{
		Protocol:     live.ProtocolVersion,
		Type:         live.TypeTaskUpdate,
		RoomID:       room.RoomID,
		Sender:       room.Creator,
		SenderPubKey: strings.Repeat("a", 64),
		Seq:          1,
		Timestamp:    "2026-03-19T00:00:10Z",
		Payload: live.LivePayload{
			Metadata: map[string]any{
				"task_id":     "task-1",
				"status":      "doing",
				"description": "同步直播任务状态",
				"assigned_to": "agent://pc76/openclaw01",
				"progress":    60,
			},
		},
	}); err != nil {
		t.Fatalf("AppendEvent error = %v", err)
	}
	if err := store.AppendEvent(room.RoomID, live.LiveMessage{
		Protocol:     live.ProtocolVersion,
		Type:         live.TypeArchiveNotice,
		RoomID:       room.RoomID,
		Sender:       room.Creator,
		SenderPubKey: strings.Repeat("a", 64),
		Seq:          2,
		Timestamp:    "2026-03-19T00:00:20Z",
		Payload: live.LivePayload{
			Content: "/posts/abc123",
			Metadata: map[string]any{
				"archive.infohash":    "abc123",
				"archive.viewer_url":  "/posts/abc123",
				"archive.archived_at": "2026-03-19T00:05:00Z",
			},
		},
	}); err != nil {
		t.Fatalf("AppendEvent archive notice error = %v", err)
	}
	if err := store.SaveArchiveResult(room.RoomID, live.ArchiveResult{
		RoomID:     room.RoomID,
		Channel:    "hao.news/live",
		Events:     1,
		ArchivedAt: "2026-03-19T00:05:00Z",
		ViewerURL:  "/posts/abc123",
		Published:  core.PublishResult{InfoHash: "abc123"},
	}); err != nil {
		t.Fatalf("SaveArchiveResult error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/live/"+room.RoomID, nil)
	rec := httptest.NewRecorder()
	site.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "打开归档文章") {
		t.Fatalf("expected archive link in body, got %q", body)
	}
	if !strings.Contains(body, "任务 ID") || !strings.Contains(body, "task-1") {
		t.Fatalf("expected task summary in body, got %q", body)
	}
	if !strings.Contains(body, "任务概览") || !strings.Contains(body, "更新次数") {
		t.Fatalf("expected task aggregate in body, got %q", body)
	}
	if !strings.Contains(body, "任务分组") || !strings.Contains(body, "按状态") || !strings.Contains(body, "按负责人") {
		t.Fatalf("expected task group panels in body, got %q", body)
	}
	if strings.Contains(body, "<span>heartbeat</span>") {
		t.Fatalf("expected heartbeats hidden by default, got %q", body)
	}
	if strings.Contains(body, "<span>archive_notice</span>") || strings.Contains(body, "archive.archived_at") || strings.Contains(body, "附带结构化元数据") {
		t.Fatalf("expected archive notices hidden by default, got %q", body)
	}
	if !strings.Contains(body, "显示心跳") || !strings.Contains(body, "关闭自动更新") {
		t.Fatalf("expected spectator controls in body, got %q", body)
	}
}

func TestPluginBuildServesLiveRoomAPIIncludesTaskSummaries(t *testing.T) {
	t.Parallel()

	site, root := buildLiveSite(t)
	store, err := live.OpenLocalStore(filepath.Join(root, "store"))
	if err != nil {
		t.Fatalf("OpenLocalStore error = %v", err)
	}
	room := live.RoomInfo{
		RoomID:    "room-4",
		Title:     "Room API Detail",
		Creator:   "agent://pc75/openclaw01",
		CreatedAt: "2026-03-19T00:00:00Z",
		Channel:   "hao.news/live",
	}
	if err := store.SaveRoom(room); err != nil {
		t.Fatalf("SaveRoom error = %v", err)
	}
	for idx, status := range []string{"todo", "doing"} {
		if err := store.AppendEvent(room.RoomID, live.LiveMessage{
			Protocol:     live.ProtocolVersion,
			Type:         live.TypeTaskUpdate,
			RoomID:       room.RoomID,
			Sender:       room.Creator,
			SenderPubKey: strings.Repeat("a", 64),
			Seq:          uint64(idx + 1),
			Timestamp:    fmt.Sprintf("2026-03-19T00:00:1%dZ", idx),
			Payload: live.LivePayload{
				Metadata: map[string]any{
					"task_id":  "task-api",
					"status":   status,
					"progress": 25 + idx*25,
				},
			},
		}); err != nil {
			t.Fatalf("AppendEvent error = %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/live/rooms/"+room.RoomID, nil)
	rec := httptest.NewRecorder()
	site.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "\"task_summaries\"") || !strings.Contains(body, "\"task-api\"") {
		t.Fatalf("expected task summaries in API body, got %q", body)
	}
	if !strings.Contains(body, "\"update_count\": 2") {
		t.Fatalf("expected update_count in API body, got %q", body)
	}
	if !strings.Contains(body, "\"task_by_status\"") || !strings.Contains(body, "\"task_by_assignee\"") {
		t.Fatalf("expected grouped task fields in API body, got %q", body)
	}
}

func TestPluginBuildServesLiveRoomAPIIncludesHeartbeatsWhenRequested(t *testing.T) {
	t.Parallel()

	site, root := buildLiveSite(t)
	store, err := live.OpenLocalStore(filepath.Join(root, "store"))
	if err != nil {
		t.Fatalf("OpenLocalStore error = %v", err)
	}
	room := live.RoomInfo{
		RoomID:    "room-5",
		Title:     "Heartbeat API",
		Creator:   "agent://pc75/openclaw01",
		CreatedAt: "2026-03-19T00:00:00Z",
		Channel:   "hao.news/live",
	}
	if err := store.SaveRoom(room); err != nil {
		t.Fatalf("SaveRoom error = %v", err)
	}
	if err := store.AppendEvent(room.RoomID, live.LiveMessage{
		Protocol:     live.ProtocolVersion,
		Type:         live.TypeHeartbeat,
		RoomID:       room.RoomID,
		Sender:       room.Creator,
		SenderPubKey: strings.Repeat("a", 64),
		Seq:          1,
		Timestamp:    "2026-03-19T00:00:10Z",
	}); err != nil {
		t.Fatalf("AppendEvent error = %v", err)
	}
	if err := store.AppendEvent(room.RoomID, live.LiveMessage{
		Protocol:     live.ProtocolVersion,
		Type:         live.TypeArchiveNotice,
		RoomID:       room.RoomID,
		Sender:       room.Creator,
		SenderPubKey: strings.Repeat("a", 64),
		Seq:          2,
		Timestamp:    "2026-03-19T00:00:20Z",
		Payload: live.LivePayload{
			Content: "/posts/archive-1",
			Metadata: map[string]any{
				"archive.infohash": "archive-1",
			},
		},
	}); err != nil {
		t.Fatalf("AppendEvent archive notice error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/live/rooms/"+room.RoomID+"?show_heartbeats=1", nil)
	rec := httptest.NewRecorder()
	site.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "\"show_heartbeats\": true") {
		t.Fatalf("expected show_heartbeats flag in API body, got %q", body)
	}
	if !strings.Contains(body, "\"type\": \"heartbeat\"") {
		t.Fatalf("expected heartbeat event in API body, got %q", body)
	}
	if !strings.Contains(body, "\"type\": \"archive_notice\"") {
		t.Fatalf("expected archive_notice event in API body when controls shown, got %q", body)
	}
}

func TestPluginBuildFiltersBlockedLiveRoomByOriginPublicKey(t *testing.T) {
	t.Parallel()

	site, root := buildLiveSite(t)
	store, err := live.OpenLocalStore(filepath.Join(root, "store"))
	if err != nil {
		t.Fatalf("OpenLocalStore error = %v", err)
	}
	blockedKey := strings.Repeat("b", 64)
	rulesBody := []byte("{\n  \"live_blocked_origin_public_keys\": [\"" + blockedKey + "\"]\n}\n")
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatalf("MkdirAll config error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "subscriptions.json"), rulesBody, 0o644); err != nil {
		t.Fatalf("WriteFile subscriptions error = %v", err)
	}
	if err := store.SaveRoom(live.RoomInfo{
		RoomID:          "room-blocked",
		Title:           "Blocked Room",
		Creator:         "agent://pc75/blocked",
		CreatorPubKey:   blockedKey,
		ParentPublicKey: strings.Repeat("c", 64),
		CreatedAt:       "2026-03-19T00:00:00Z",
		Channel:         "hao.news/live",
	}); err != nil {
		t.Fatalf("SaveRoom error = %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/live", nil)
	rec := httptest.NewRecorder()
	site.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "Blocked Room") {
		t.Fatalf("expected blocked room hidden from live index, got %q", rec.Body.String())
	}
}

func TestPluginBuildFiltersBlockedLiveRoomEventsByOriginPublicKey(t *testing.T) {
	t.Parallel()

	site, root := buildLiveSite(t)
	store, err := live.OpenLocalStore(filepath.Join(root, "store"))
	if err != nil {
		t.Fatalf("OpenLocalStore error = %v", err)
	}
	blockedKey := strings.Repeat("b", 64)
	rulesBody := []byte("{\n  \"live_blocked_origin_public_keys\": [\"" + blockedKey + "\"]\n}\n")
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatalf("MkdirAll config error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "subscriptions.json"), rulesBody, 0o644); err != nil {
		t.Fatalf("WriteFile subscriptions error = %v", err)
	}
	room := live.RoomInfo{
		RoomID:          "room-events",
		Title:           "Room Events",
		Creator:         "agent://pc75/openclaw01",
		CreatorPubKey:   strings.Repeat("a", 64),
		ParentPublicKey: strings.Repeat("d", 64),
		CreatedAt:       "2026-03-19T00:00:00Z",
		Channel:         "hao.news/live",
	}
	if err := store.SaveRoom(room); err != nil {
		t.Fatalf("SaveRoom error = %v", err)
	}
	if err := store.AppendEvent(room.RoomID, live.LiveMessage{
		Protocol:     live.ProtocolVersion,
		Type:         live.TypeMessage,
		RoomID:       room.RoomID,
		Sender:       "agent://pc75/allowed",
		SenderPubKey: strings.Repeat("a", 64),
		Seq:          1,
		Timestamp:    "2026-03-19T00:00:10Z",
		Payload: live.LivePayload{
			Content: "allowed event",
			Metadata: map[string]any{
				"origin_public_key": strings.Repeat("a", 64),
				"parent_public_key": strings.Repeat("d", 64),
			},
		},
	}); err != nil {
		t.Fatalf("AppendEvent allowed error = %v", err)
	}
	if err := store.AppendEvent(room.RoomID, live.LiveMessage{
		Protocol:     live.ProtocolVersion,
		Type:         live.TypeMessage,
		RoomID:       room.RoomID,
		Sender:       "agent://pc75/blocked",
		SenderPubKey: blockedKey,
		Seq:          2,
		Timestamp:    "2026-03-19T00:00:20Z",
		Payload: live.LivePayload{
			Content: "blocked event",
			Metadata: map[string]any{
				"origin_public_key": blockedKey,
				"parent_public_key": strings.Repeat("c", 64),
			},
		},
	}); err != nil {
		t.Fatalf("AppendEvent blocked error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/live/rooms/"+room.RoomID+"?show_heartbeats=1", nil)
	rec := httptest.NewRecorder()
	site.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "allowed event") {
		t.Fatalf("expected allowed live event in API body, got %q", body)
	}
	if strings.Contains(body, "blocked event") {
		t.Fatalf("expected blocked live event hidden from API body, got %q", body)
	}
}

func TestPluginBuildServesLiveRoomAPIVisibility(t *testing.T) {
	t.Parallel()

	site, root := buildLiveSite(t)
	store, err := live.OpenLocalStore(filepath.Join(root, "store"))
	if err != nil {
		t.Fatalf("OpenLocalStore error = %v", err)
	}
	allowedParent := strings.Repeat("d", 64)
	rulesBody := []byte("{\n  \"live_allowed_parent_public_keys\": [\"" + allowedParent + "\"]\n}\n")
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatalf("MkdirAll config error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "subscriptions.json"), rulesBody, 0o644); err != nil {
		t.Fatalf("WriteFile subscriptions error = %v", err)
	}
	room := live.RoomInfo{
		RoomID:          "room-visibility",
		Title:           "Room Visibility",
		Creator:         "agent://pc75/openclaw01",
		CreatorPubKey:   strings.Repeat("a", 64),
		ParentPublicKey: allowedParent,
		CreatedAt:       "2026-03-19T00:00:00Z",
		Channel:         "hao.news/live",
	}
	if err := store.SaveRoom(room); err != nil {
		t.Fatalf("SaveRoom error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/live/rooms/"+room.RoomID, nil)
	rec := httptest.NewRecorder()
	site.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "\"room_visibility\": \"allowed_parent\"") {
		t.Fatalf("expected room_visibility in API body, got %q", body)
	}
}

func TestPluginBuildServesLivePendingIndexForBlockedRoom(t *testing.T) {
	t.Parallel()

	site, root := buildLiveSite(t)
	store, err := live.OpenLocalStore(filepath.Join(root, "store"))
	if err != nil {
		t.Fatalf("OpenLocalStore error = %v", err)
	}
	blockedKey := strings.Repeat("e", 64)
	rulesBody := []byte("{\n  \"live_blocked_origin_public_keys\": [\"" + blockedKey + "\"]\n}\n")
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatalf("MkdirAll config error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "subscriptions.json"), rulesBody, 0o644); err != nil {
		t.Fatalf("WriteFile subscriptions error = %v", err)
	}
	if err := store.SaveRoom(live.RoomInfo{
		RoomID:          "room-pending",
		Title:           "Pending Room",
		Creator:         "agent://pc75/pending",
		CreatorPubKey:   blockedKey,
		ParentPublicKey: strings.Repeat("f", 64),
		CreatedAt:       "2026-03-19T00:00:00Z",
		Channel:         "hao.news/live",
	}); err != nil {
		t.Fatalf("SaveRoom error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/live/pending", nil)
	rec := httptest.NewRecorder()
	site.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Pending Room") || !strings.Contains(body, "blocked_origin") {
		t.Fatalf("expected blocked room in pending live index, got %q", body)
	}
}

func TestPluginBuildServesLivePendingRoomAPIForBlockedEvents(t *testing.T) {
	t.Parallel()

	site, root := buildLiveSite(t)
	store, err := live.OpenLocalStore(filepath.Join(root, "store"))
	if err != nil {
		t.Fatalf("OpenLocalStore error = %v", err)
	}
	blockedKey := strings.Repeat("b", 64)
	rulesBody := []byte("{\n  \"live_blocked_origin_public_keys\": [\"" + blockedKey + "\"]\n}\n")
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatalf("MkdirAll config error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "subscriptions.json"), rulesBody, 0o644); err != nil {
		t.Fatalf("WriteFile subscriptions error = %v", err)
	}
	room := live.RoomInfo{
		RoomID:          "room-pending-events",
		Title:           "Pending Events",
		Creator:         "agent://pc75/openclaw01",
		CreatorPubKey:   strings.Repeat("a", 64),
		ParentPublicKey: strings.Repeat("d", 64),
		CreatedAt:       "2026-03-19T00:00:00Z",
		Channel:         "hao.news/live",
	}
	if err := store.SaveRoom(room); err != nil {
		t.Fatalf("SaveRoom error = %v", err)
	}
	if err := store.AppendEvent(room.RoomID, live.LiveMessage{
		Protocol:     live.ProtocolVersion,
		Type:         live.TypeMessage,
		RoomID:       room.RoomID,
		Sender:       "agent://pc75/allowed",
		SenderPubKey: strings.Repeat("a", 64),
		Seq:          1,
		Timestamp:    "2026-03-19T00:00:10Z",
		Payload: live.LivePayload{
			Content: "allowed event",
			Metadata: map[string]any{
				"origin_public_key": strings.Repeat("a", 64),
				"parent_public_key": strings.Repeat("d", 64),
			},
		},
	}); err != nil {
		t.Fatalf("AppendEvent allowed error = %v", err)
	}
	if err := store.AppendEvent(room.RoomID, live.LiveMessage{
		Protocol:     live.ProtocolVersion,
		Type:         live.TypeMessage,
		RoomID:       room.RoomID,
		Sender:       "agent://pc75/blocked",
		SenderPubKey: blockedKey,
		Seq:          2,
		Timestamp:    "2026-03-19T00:00:20Z",
		Payload: live.LivePayload{
			Content: "blocked event",
			Metadata: map[string]any{
				"origin_public_key": blockedKey,
				"parent_public_key": strings.Repeat("c", 64),
			},
		},
	}); err != nil {
		t.Fatalf("AppendEvent blocked error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/live/pending/"+room.RoomID, nil)
	rec := httptest.NewRecorder()
	site.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "\"scope\": \"live-pending-room\"") {
		t.Fatalf("expected pending room scope in API body, got %q", body)
	}
	if !strings.Contains(body, "blocked event") || strings.Contains(body, "allowed event") {
		t.Fatalf("expected only blocked live event in pending room API, got %q", body)
	}
}

func TestPluginBuildServesLiveRoomAPIIncludesPendingBlockedEvents(t *testing.T) {
	t.Parallel()

	site, root := buildLiveSite(t)
	store, err := live.OpenLocalStore(filepath.Join(root, "store"))
	if err != nil {
		t.Fatalf("OpenLocalStore error = %v", err)
	}
	blockedKey := strings.Repeat("b", 64)
	rulesBody := []byte("{\n  \"live_blocked_origin_public_keys\": [\"" + blockedKey + "\"]\n}\n")
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatalf("MkdirAll config error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "subscriptions.json"), rulesBody, 0o644); err != nil {
		t.Fatalf("WriteFile subscriptions error = %v", err)
	}
	room := live.RoomInfo{
		RoomID:          "room-api-pending-count",
		Title:           "API Pending Count",
		Creator:         "agent://pc75/openclaw01",
		CreatorPubKey:   strings.Repeat("a", 64),
		ParentPublicKey: strings.Repeat("d", 64),
		CreatedAt:       "2026-03-19T00:00:00Z",
		Channel:         "hao.news/live",
	}
	if err := store.SaveRoom(room); err != nil {
		t.Fatalf("SaveRoom error = %v", err)
	}
	if err := store.AppendEvent(room.RoomID, live.LiveMessage{
		Protocol:     live.ProtocolVersion,
		Type:         live.TypeMessage,
		RoomID:       room.RoomID,
		Sender:       "agent://pc75/blocked",
		SenderPubKey: blockedKey,
		Seq:          1,
		Timestamp:    "2026-03-19T00:00:20Z",
		Payload: live.LivePayload{
			Content: "blocked event",
			Metadata: map[string]any{
				"origin_public_key": blockedKey,
				"parent_public_key": strings.Repeat("c", 64),
			},
		},
	}); err != nil {
		t.Fatalf("AppendEvent blocked error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/live/rooms/"+room.RoomID, nil)
	rec := httptest.NewRecorder()
	site.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "\"pending_blocked_events\": 1") {
		t.Fatalf("expected pending_blocked_events in live room API body, got %q", body)
	}
}

func buildLiveSite(t *testing.T) (*apphost.Site, string) {
	t.Helper()
	liveAnnouncementWatcherDisabledForTests = true

	root := t.TempDir()
	cfg := apphost.Config{
		RuntimeRoot:      filepath.Join(root, "runtime"),
		StoreRoot:        filepath.Join(root, "store"),
		ArchiveRoot:      filepath.Join(root, "archive"),
		RulesPath:        filepath.Join(root, "config", "subscriptions.json"),
		WriterPolicyPath: filepath.Join(root, "config", "writer_policy.json"),
		NetPath:          filepath.Join(root, "config", "haonews_net.inf"),
		Project:          "hao.news",
		Version:          "test",
		Plugin:           "hao-news-live",
		Plugins:          []string{"hao-news-content", "hao-news-live"},
	}
	site, err := Plugin{}.Build(context.Background(), cfg, haonewstheme.Theme{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	t.Cleanup(func() {
		_ = site.Close(context.Background())
	})
	return site, root
}
