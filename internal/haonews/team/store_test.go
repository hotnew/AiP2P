package team

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreListTeams(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	teamRoot := filepath.Join(root, "team", "project-alpha")
	if err := os.MkdirAll(teamRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(teamRoot, "team.json"), []byte(`{
  "team_id": "project-alpha",
  "title": "Project Alpha",
  "description": "Long-running multi-agent project",
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

	store, err := OpenStore(root)
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}
	teams, err := store.ListTeams()
	if err != nil {
		t.Fatalf("ListTeams error = %v", err)
	}
	if len(teams) != 1 {
		t.Fatalf("expected 1 team, got %d", len(teams))
	}
	if teams[0].Title != "Project Alpha" || teams[0].MemberCount != 2 || teams[0].ChannelCount != 2 {
		t.Fatalf("unexpected team summary: %#v", teams[0])
	}
}

func TestStoreLoadTeamDefaults(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	teamRoot := filepath.Join(root, "team", "demo-team")
	if err := os.MkdirAll(teamRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(teamRoot, "team.json"), []byte(`{
  "title": "Demo Team"
}`), 0o644); err != nil {
		t.Fatalf("WriteFile(team.json) error = %v", err)
	}
	store, err := OpenStore(root)
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}
	info, err := store.LoadTeam("demo-team")
	if err != nil {
		t.Fatalf("LoadTeam error = %v", err)
	}
	if info.TeamID != "demo-team" || info.Slug != "demo-team" || info.Visibility != "team" {
		t.Fatalf("unexpected info defaults: %#v", info)
	}
	if len(info.Channels) != 1 || info.Channels[0] != "main" {
		t.Fatalf("expected default main channel, got %#v", info.Channels)
	}
}

func TestNormalizeTeamID(t *testing.T) {
	t.Parallel()

	got := NormalizeTeamID("  Project / Alpha_Test  ")
	if got != "project-alpha-test" {
		t.Fatalf("NormalizeTeamID = %q", got)
	}
	if strings.Contains(got, "/") || strings.Contains(got, "_") {
		t.Fatalf("expected normalized team id, got %q", got)
	}
}

func TestStoreAppendAndLoadMessages(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	teamRoot := filepath.Join(root, "team", "project-gamma")
	if err := os.MkdirAll(teamRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(teamRoot, "team.json"), []byte(`{"team_id":"project-gamma","title":"Project Gamma"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(team.json) error = %v", err)
	}
	store, err := OpenStore(root)
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}
	if err := store.AppendMessage("project-gamma", Message{
		ChannelID:     "main",
		AuthorAgentID: "agent://pc75/live-alpha",
		MessageType:   "chat",
		Content:       "first team message",
		CreatedAt:     time.Date(2026, 4, 1, 1, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("AppendMessage(first) error = %v", err)
	}
	if err := store.AppendMessage("project-gamma", Message{
		ChannelID:     "main",
		AuthorAgentID: "agent://pc75/live-bravo",
		MessageType:   "decision",
		Content:       "second team message",
		CreatedAt:     time.Date(2026, 4, 1, 2, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("AppendMessage(second) error = %v", err)
	}

	messages, err := store.LoadMessages("project-gamma", "main", 10)
	if err != nil {
		t.Fatalf("LoadMessages error = %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Content != "second team message" || messages[1].Content != "first team message" {
		t.Fatalf("unexpected message order: %#v", messages)
	}
}

func TestStoreAppendAndLoadTasks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	teamRoot := filepath.Join(root, "team", "project-delta")
	if err := os.MkdirAll(teamRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(teamRoot, "team.json"), []byte(`{"team_id":"project-delta","title":"Project Delta"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(team.json) error = %v", err)
	}
	store, err := OpenStore(root)
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}
	if err := store.AppendTask("project-delta", Task{
		CreatedBy: "agent://pc75/openclaw01",
		Title:     "Prepare task model",
		Status:    "doing",
		UpdatedAt: time.Date(2026, 4, 1, 5, 0, 0, 0, time.UTC),
		CreatedAt: time.Date(2026, 4, 1, 4, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("AppendTask(first) error = %v", err)
	}
	if err := store.AppendTask("project-delta", Task{
		CreatedBy: "agent://pc75/live-alpha",
		Title:     "Review team message design",
		Status:    "open",
		UpdatedAt: time.Date(2026, 4, 1, 6, 0, 0, 0, time.UTC),
		CreatedAt: time.Date(2026, 4, 1, 5, 30, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("AppendTask(second) error = %v", err)
	}

	tasks, err := store.LoadTasks("project-delta", 10)
	if err != nil {
		t.Fatalf("LoadTasks error = %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].Title != "Review team message design" || tasks[1].Title != "Prepare task model" {
		t.Fatalf("unexpected task order: %#v", tasks)
	}
}
