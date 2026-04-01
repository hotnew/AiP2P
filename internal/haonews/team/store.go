package team

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Store struct {
	root string
}

type Info struct {
	TeamID               string    `json:"team_id"`
	Slug                 string    `json:"slug,omitempty"`
	Title                string    `json:"title"`
	Description          string    `json:"description,omitempty"`
	Visibility           string    `json:"visibility,omitempty"`
	OwnerAgentID         string    `json:"owner_agent_id,omitempty"`
	OwnerOriginPublicKey string    `json:"owner_origin_public_key,omitempty"`
	OwnerParentPublicKey string    `json:"owner_parent_public_key,omitempty"`
	Channels             []string  `json:"channels,omitempty"`
	CreatedAt            time.Time `json:"created_at,omitempty"`
	UpdatedAt            time.Time `json:"updated_at,omitempty"`
}

type Member struct {
	AgentID         string    `json:"agent_id"`
	OriginPublicKey string    `json:"origin_public_key,omitempty"`
	ParentPublicKey string    `json:"parent_public_key,omitempty"`
	Role            string    `json:"role,omitempty"`
	Status          string    `json:"status,omitempty"`
	JoinedAt        time.Time `json:"joined_at,omitempty"`
}

type Summary struct {
	Info
	MemberCount  int `json:"member_count"`
	ChannelCount int `json:"channel_count"`
}

type Message struct {
	MessageID       string         `json:"message_id"`
	TeamID          string         `json:"team_id"`
	ChannelID       string         `json:"channel_id"`
	AuthorAgentID   string         `json:"author_agent_id"`
	OriginPublicKey string         `json:"origin_public_key,omitempty"`
	ParentPublicKey string         `json:"parent_public_key,omitempty"`
	MessageType     string         `json:"message_type"`
	Content         string         `json:"content"`
	StructuredData  map[string]any `json:"structured_data,omitempty"`
	CreatedAt       time.Time      `json:"created_at,omitempty"`
}

type Task struct {
	TaskID          string    `json:"task_id"`
	TeamID          string    `json:"team_id"`
	Title           string    `json:"title"`
	Description     string    `json:"description,omitempty"`
	CreatedBy       string    `json:"created_by,omitempty"`
	Assignees       []string  `json:"assignees,omitempty"`
	Status          string    `json:"status,omitempty"`
	Priority        string    `json:"priority,omitempty"`
	Labels          []string  `json:"labels,omitempty"`
	OriginPublicKey string    `json:"origin_public_key,omitempty"`
	ParentPublicKey string    `json:"parent_public_key,omitempty"`
	CreatedAt       time.Time `json:"created_at,omitempty"`
	UpdatedAt       time.Time `json:"updated_at,omitempty"`
	ClosedAt        time.Time `json:"closed_at,omitempty"`
}

func OpenStore(storeRoot string) (*Store, error) {
	root := filepath.Join(strings.TrimSpace(storeRoot), "team")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}

func (s *Store) Root() string {
	if s == nil {
		return ""
	}
	return s.root
}

func NormalizeTeamID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "/", "-")
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, " ", "-")
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	return strings.Trim(value, "-")
}

func (s *Store) ListTeams() ([]Summary, error) {
	if s == nil {
		return nil, nil
	}
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	out := make([]Summary, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		teamID := NormalizeTeamID(entry.Name())
		if teamID == "" {
			continue
		}
		info, err := s.LoadTeam(teamID)
		if err != nil {
			continue
		}
		members, err := s.LoadMembers(teamID)
		if err != nil {
			continue
		}
		out = append(out, Summary{
			Info:         info,
			MemberCount:  len(members),
			ChannelCount: len(teamChannels(info)),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		return out[i].TeamID < out[j].TeamID
	})
	return out, nil
}

func (s *Store) LoadTeam(teamID string) (Info, error) {
	if s == nil {
		return Info{}, errors.New("nil team store")
	}
	teamID = NormalizeTeamID(teamID)
	if teamID == "" {
		return Info{}, errors.New("empty team id")
	}
	path := filepath.Join(s.root, teamID, "team.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return Info{}, err
	}
	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		return Info{}, err
	}
	if strings.TrimSpace(info.TeamID) == "" {
		info.TeamID = teamID
	}
	if strings.TrimSpace(info.Slug) == "" {
		info.Slug = teamID
	}
	info.TeamID = NormalizeTeamID(info.TeamID)
	info.Slug = NormalizeTeamID(info.Slug)
	if info.TeamID == "" {
		info.TeamID = teamID
	}
	if info.Slug == "" {
		info.Slug = info.TeamID
	}
	if strings.TrimSpace(info.Visibility) == "" {
		info.Visibility = "team"
	}
	info.Channels = teamChannels(info)
	return info, nil
}

func (s *Store) LoadMembers(teamID string) ([]Member, error) {
	if s == nil {
		return nil, errors.New("nil team store")
	}
	teamID = NormalizeTeamID(teamID)
	if teamID == "" {
		return nil, errors.New("empty team id")
	}
	path := filepath.Join(s.root, teamID, "members.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var members []Member
	if err := json.Unmarshal(data, &members); err != nil {
		return nil, err
	}
	sort.SliceStable(members, func(i, j int) bool {
		if members[i].Role != members[j].Role {
			return members[i].Role < members[j].Role
		}
		return members[i].AgentID < members[j].AgentID
	})
	return members, nil
}

func (s *Store) AppendMessage(teamID string, msg Message) error {
	if s == nil {
		return errors.New("nil team store")
	}
	teamID = NormalizeTeamID(teamID)
	if teamID == "" {
		return errors.New("empty team id")
	}
	channelID := normalizeChannelID(msg.ChannelID)
	if channelID == "" {
		channelID = "main"
	}
	if strings.TrimSpace(msg.TeamID) == "" {
		msg.TeamID = teamID
	}
	msg.TeamID = NormalizeTeamID(msg.TeamID)
	if msg.TeamID != teamID {
		return fmt.Errorf("team message team_id %q does not match %q", msg.TeamID, teamID)
	}
	msg.ChannelID = channelID
	msg.MessageType = strings.TrimSpace(msg.MessageType)
	if msg.MessageType == "" {
		msg.MessageType = "chat"
	}
	msg.Content = strings.TrimSpace(msg.Content)
	if msg.Content == "" {
		return errors.New("empty team message content")
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now().UTC()
	}
	if strings.TrimSpace(msg.MessageID) == "" {
		msg.MessageID = buildMessageID(msg)
	}
	path := s.channelPath(teamID, channelID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(body, '\n')); err != nil {
		return err
	}
	return nil
}

func (s *Store) LoadMessages(teamID, channelID string, limit int) ([]Message, error) {
	if s == nil {
		return nil, errors.New("nil team store")
	}
	teamID = NormalizeTeamID(teamID)
	if teamID == "" {
		return nil, errors.New("empty team id")
	}
	channelID = normalizeChannelID(channelID)
	if channelID == "" {
		channelID = "main"
	}
	path := s.channelPath(teamID, channelID)
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var out []Message
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg Message
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		out = append(out, msg)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if limit > 0 && len(out) > limit {
		out = append([]Message(nil), out[len(out)-limit:]...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].MessageID > out[j].MessageID
	})
	return out, nil
}

func (s *Store) AppendTask(teamID string, task Task) error {
	if s == nil {
		return errors.New("nil team store")
	}
	teamID = NormalizeTeamID(teamID)
	if teamID == "" {
		return errors.New("empty team id")
	}
	if strings.TrimSpace(task.TeamID) == "" {
		task.TeamID = teamID
	}
	task.TeamID = NormalizeTeamID(task.TeamID)
	if task.TeamID != teamID {
		return fmt.Errorf("team task team_id %q does not match %q", task.TeamID, teamID)
	}
	task.Title = strings.TrimSpace(task.Title)
	if task.Title == "" {
		return errors.New("empty team task title")
	}
	if task.Status == "" {
		task.Status = "open"
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now().UTC()
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = task.CreatedAt
	}
	if strings.TrimSpace(task.TaskID) == "" {
		task.TaskID = buildTaskID(task)
	}
	path := filepath.Join(s.root, teamID, "tasks.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	body, err := json.Marshal(task)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(body, '\n')); err != nil {
		return err
	}
	return nil
}

func (s *Store) LoadTasks(teamID string, limit int) ([]Task, error) {
	if s == nil {
		return nil, errors.New("nil team store")
	}
	teamID = NormalizeTeamID(teamID)
	if teamID == "" {
		return nil, errors.New("empty team id")
	}
	path := filepath.Join(s.root, teamID, "tasks.jsonl")
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var out []Task
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var task Task
		if err := json.Unmarshal([]byte(line), &task); err != nil {
			continue
		}
		out = append(out, task)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if limit > 0 && len(out) > limit {
		out = append([]Task(nil), out[len(out)-limit:]...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		return out[i].TaskID > out[j].TaskID
	})
	return out, nil
}

func teamChannels(info Info) []string {
	if len(info.Channels) == 0 {
		return []string{"main"}
	}
	out := make([]string, 0, len(info.Channels))
	seen := make(map[string]struct{}, len(info.Channels))
	for _, channel := range info.Channels {
		channel = NormalizeTeamID(channel)
		if channel == "" {
			continue
		}
		if _, ok := seen[channel]; ok {
			continue
		}
		seen[channel] = struct{}{}
		out = append(out, channel)
	}
	if len(out) == 0 {
		out = append(out, "main")
	}
	return out
}

func normalizeChannelID(value string) string {
	value = NormalizeTeamID(value)
	if value == "" {
		return "main"
	}
	return value
}

func (s *Store) channelPath(teamID, channelID string) string {
	return filepath.Join(s.root, teamID, "channels", normalizeChannelID(channelID)+".jsonl")
}

func buildMessageID(msg Message) string {
	return strings.Join([]string{
		strings.TrimSpace(msg.TeamID),
		normalizeChannelID(msg.ChannelID),
		strings.TrimSpace(msg.AuthorAgentID),
		msg.CreatedAt.UTC().Format(time.RFC3339Nano),
		strings.TrimSpace(msg.Content),
	}, ":")
}

func buildTaskID(task Task) string {
	return strings.Join([]string{
		strings.TrimSpace(task.TeamID),
		strings.TrimSpace(task.CreatedBy),
		task.CreatedAt.UTC().Format(time.RFC3339Nano),
		strings.TrimSpace(task.Title),
	}, ":")
}
