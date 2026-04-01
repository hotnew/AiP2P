package haonewsteam

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hao.news/internal/apphost"
	teamcore "hao.news/internal/haonews/team"
	themehaonews "hao.news/internal/themes/haonews"
)

func TestPluginBuildServesTeamIndex(t *testing.T) {
	t.Parallel()

	site, root := buildTeamSite(t)
	teamRoot := filepath.Join(root, "store", "team", "project-alpha")
	if err := os.MkdirAll(teamRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(teamRoot, "team.json"), []byte(`{
  "team_id": "project-alpha",
  "title": "Project Alpha",
  "description": "Coordination team",
  "visibility": "team",
  "owner_agent_id": "agent://pc75/openclaw01",
  "channels": ["main", "research"],
  "created_at": "2026-04-01T00:00:00Z",
  "updated_at": "2026-04-01T01:00:00Z"
}`), 0o644); err != nil {
		t.Fatalf("WriteFile(team.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(teamRoot, "members.json"), []byte(`[
  {"agent_id":"agent://pc75/openclaw01","role":"owner","status":"active"},
  {"agent_id":"agent://pc75/live-alpha","role":"member","status":"active"}
]`), 0o644); err != nil {
		t.Fatalf("WriteFile(members.json) error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/teams", nil)
	rec := httptest.NewRecorder()
	site.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Project Alpha") || !strings.Contains(body, "Coordination team") {
		t.Fatalf("expected team in body, got %q", body)
	}
}

func TestPluginBuildServesEmptyTeamIndex(t *testing.T) {
	t.Parallel()

	site, _ := buildTeamSite(t)
	req := httptest.NewRequest(http.MethodGet, "/teams", nil)
	rec := httptest.NewRecorder()
	site.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "暂无 Team") {
		t.Fatalf("expected empty state body, got %q", rec.Body.String())
	}
}

func TestPluginBuildServesTeamDetailAndAPI(t *testing.T) {
	t.Parallel()

	site, root := buildTeamSite(t)
	store, err := teamcore.OpenStore(filepath.Join(root, "store"))
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}
	teamRoot := filepath.Join(root, "store", "team", "project-beta")
	if err := os.MkdirAll(teamRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(teamRoot, "team.json"), []byte(`{
  "team_id": "project-beta",
  "title": "Project Beta",
  "description": "Independent team module",
  "visibility": "private",
  "owner_agent_id": "agent://pc75/live-bravo",
  "owner_origin_public_key": "`+strings.Repeat("a", 64)+`",
  "owner_parent_public_key": "`+strings.Repeat("b", 64)+`",
  "channels": ["main"],
  "created_at": "2026-04-01T02:00:00Z",
  "updated_at": "2026-04-01T03:00:00Z"
}`), 0o644); err != nil {
		t.Fatalf("WriteFile(team.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(teamRoot, "members.json"), []byte(`[
  {"agent_id":"agent://pc75/live-bravo","role":"owner","status":"active"}
]`), 0o644); err != nil {
		t.Fatalf("WriteFile(members.json) error = %v", err)
	}
	if err := store.AppendMessage("project-beta", teamcore.Message{
		ChannelID:     "main",
		AuthorAgentID: "agent://pc75/live-bravo",
		MessageType:   "decision",
		Content:       "Team Beta decided to keep Team separate from Live.",
		CreatedAt:     time.Date(2026, 4, 1, 3, 30, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("AppendMessage error = %v", err)
	}
	if err := store.AppendTask("project-beta", teamcore.Task{
		CreatedBy: "agent://pc75/live-bravo",
		Title:     "Implement TeamTask",
		Status:    "doing",
		UpdatedAt: time.Date(2026, 4, 1, 3, 45, 0, 0, time.UTC),
		CreatedAt: time.Date(2026, 4, 1, 3, 40, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("AppendTask error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/teams/project-beta", nil)
	rec := httptest.NewRecorder()
	site.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Project Beta") || !strings.Contains(rec.Body.String(), "成员") || !strings.Contains(rec.Body.String(), "最近消息") || !strings.Contains(rec.Body.String(), "Team Beta decided to keep Team separate from Live.") || !strings.Contains(rec.Body.String(), "最近任务") || !strings.Contains(rec.Body.String(), "Implement TeamTask") {
		t.Fatalf("expected team detail body, got %q", rec.Body.String())
	}

	apiReq := httptest.NewRequest(http.MethodGet, "/api/teams/project-beta", nil)
	apiRec := httptest.NewRecorder()
	site.Handler.ServeHTTP(apiRec, apiReq)
	if apiRec.Code != http.StatusOK {
		t.Fatalf("api status = %d, body = %s", apiRec.Code, apiRec.Body.String())
	}
	if !strings.Contains(apiRec.Body.String(), "\"scope\": \"team-detail\"") || !strings.Contains(apiRec.Body.String(), "\"team_id\": \"project-beta\"") {
		t.Fatalf("expected team detail api body, got %q", apiRec.Body.String())
	}

	membersReq := httptest.NewRequest(http.MethodGet, "/api/teams/project-beta/members", nil)
	membersRec := httptest.NewRecorder()
	site.Handler.ServeHTTP(membersRec, membersReq)
	if membersRec.Code != http.StatusOK {
		t.Fatalf("members api status = %d, body = %s", membersRec.Code, membersRec.Body.String())
	}
	if !strings.Contains(membersRec.Body.String(), "\"scope\": \"team-members\"") || !strings.Contains(membersRec.Body.String(), "\"member_count\": 1") {
		t.Fatalf("expected team members api body, got %q", membersRec.Body.String())
	}

	messagesReq := httptest.NewRequest(http.MethodGet, "/api/teams/project-beta/messages", nil)
	messagesRec := httptest.NewRecorder()
	site.Handler.ServeHTTP(messagesRec, messagesReq)
	if messagesRec.Code != http.StatusOK {
		t.Fatalf("messages api status = %d, body = %s", messagesRec.Code, messagesRec.Body.String())
	}
	if !strings.Contains(messagesRec.Body.String(), "\"scope\": \"team-messages\"") || !strings.Contains(messagesRec.Body.String(), "Team Beta decided to keep Team separate from Live.") {
		t.Fatalf("expected team messages api body, got %q", messagesRec.Body.String())
	}

	tasksReq := httptest.NewRequest(http.MethodGet, "/api/teams/project-beta/tasks", nil)
	tasksRec := httptest.NewRecorder()
	site.Handler.ServeHTTP(tasksRec, tasksReq)
	if tasksRec.Code != http.StatusOK {
		t.Fatalf("tasks api status = %d, body = %s", tasksRec.Code, tasksRec.Body.String())
	}
	if !strings.Contains(tasksRec.Body.String(), "\"scope\": \"team-tasks\"") || !strings.Contains(tasksRec.Body.String(), "Implement TeamTask") {
		t.Fatalf("expected team tasks api body, got %q", tasksRec.Body.String())
	}
}

func buildTeamSite(t *testing.T) (*apphost.Site, string) {
	t.Helper()

	root := t.TempDir()
	site, err := Plugin{}.Build(context.Background(), apphost.Config{
		StoreRoot:        filepath.Join(root, "store"),
		Project:          "hao.news",
		Version:          "dev",
		ArchiveRoot:      filepath.Join(root, "archive"),
		RulesPath:        filepath.Join(root, "config", "subscriptions.json"),
		WriterPolicyPath: filepath.Join(root, "config", "writer_policy.json"),
		NetPath:          filepath.Join(root, "config", "haonews_net.inf"),
		Plugin:           "hao-news-team",
		Plugins:          []string{"hao-news-content", "hao-news-live", "hao-news-team", "hao-news-archive", "hao-news-governance", "hao-news-ops"},
	}, themehaonews.Theme{})
	if err != nil {
		t.Fatalf("Plugin.Build error = %v", err)
	}
	return site, root
}
