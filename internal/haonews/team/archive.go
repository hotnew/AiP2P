package team

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ArchiveSnapshot struct {
	ArchiveID     string        `json:"archive_id"`
	TeamID        string        `json:"team_id"`
	Kind          string        `json:"kind"`
	Label         string        `json:"label,omitempty"`
	ArchivedAt    time.Time     `json:"archived_at,omitempty"`
	Info          Info          `json:"info"`
	Policy        Policy        `json:"policy"`
	Members       []Member      `json:"members,omitempty"`
	Channels      []Channel     `json:"channels,omitempty"`
	Messages      []Message     `json:"messages,omitempty"`
	Tasks         []Task        `json:"tasks,omitempty"`
	Artifacts     []Artifact    `json:"artifacts,omitempty"`
	History       []ChangeEvent `json:"history,omitempty"`
	MessageCount  int           `json:"message_count"`
	TaskCount     int           `json:"task_count"`
	ArtifactCount int           `json:"artifact_count"`
	HistoryCount  int           `json:"history_count"`
}

func (s *Store) createManualArchiveNoCtx(teamID string, now time.Time) (*ArchiveSnapshot, error) {
	if s == nil {
		return nil, errors.New("nil team store")
	}
	teamID = NormalizeTeamID(teamID)
	if teamID == "" {
		return nil, errors.New("empty team id")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	var snapshot ArchiveSnapshot
	if err := s.withTeamLock(teamID, func() error {
		info, err := s.loadTeamNoCtx(teamID)
		if err != nil {
			return err
		}
		policy, err := s.loadPolicyNoCtx(teamID)
		if err != nil {
			return err
		}
		members, err := s.loadMembersNoCtx(teamID)
		if err != nil {
			return err
		}
		channelSummaries, err := s.listChannelsNoCtx(teamID)
		if err != nil {
			return err
		}
		channels := make([]Channel, 0, len(channelSummaries))
		messages := make([]Message, 0)
		for _, summary := range channelSummaries {
			channel, err := s.loadChannelNoCtx(teamID, summary.ChannelID)
			if err != nil {
				channel = summary.Channel
			}
			channels = append(channels, channel)
			items, err := s.loadMessagesNoCtx(teamID, summary.ChannelID, 0)
			if err != nil {
				return err
			}
			messages = append(messages, items...)
		}
		tasks, err := s.loadTasksCurrentLocked(teamID, 0)
		if err != nil {
			return err
		}
		artifacts, err := s.loadArtifactsCurrentLocked(teamID, 0)
		if err != nil {
			return err
		}
		history, err := s.loadHistoryNoCtx(teamID, 0)
		if err != nil {
			return err
		}
		snapshot = ArchiveSnapshot{
			ArchiveID:     fmt.Sprintf("manual-%s", now.UTC().Format("20060102T150405Z")),
			TeamID:        teamID,
			Kind:          "manual",
			Label:         "手动归档",
			ArchivedAt:    now.UTC(),
			Info:          info,
			Policy:        policy,
			Members:       append([]Member(nil), members...),
			Channels:      append([]Channel(nil), channels...),
			Messages:      append([]Message(nil), messages...),
			Tasks:         append([]Task(nil), tasks...),
			Artifacts:     append([]Artifact(nil), artifacts...),
			History:       append([]ChangeEvent(nil), history...),
			MessageCount:  len(messages),
			TaskCount:     len(tasks),
			ArtifactCount: len(artifacts),
			HistoryCount:  len(history),
		}
		return s.saveArchiveSnapshot(teamID, snapshot)
	}); err != nil {
		return nil, err
	}
	s.publish(TeamEvent{
		TeamID:    teamID,
		Kind:      "archive",
		Action:    "create",
		SubjectID: snapshot.ArchiveID,
		Metadata: map[string]any{
			"kind":          snapshot.Kind,
			"message_count": snapshot.MessageCount,
			"task_count":    snapshot.TaskCount,
		},
	})
	return &snapshot, nil
}

func (s *Store) listArchivesNoCtx(teamID string) ([]ArchiveSnapshot, error) {
	if s == nil {
		return nil, errors.New("nil team store")
	}
	teamID = NormalizeTeamID(teamID)
	if teamID == "" {
		return nil, errors.New("empty team id")
	}
	dir := filepath.Join(s.root, teamID, "archives")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]ArchiveSnapshot, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		archiveID := strings.TrimSuffix(entry.Name(), ".json")
		record, err := s.loadArchiveNoCtx(teamID, archiveID)
		if err != nil {
			continue
		}
		out = append(out, record)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].ArchivedAt.Equal(out[j].ArchivedAt) {
			return out[i].ArchivedAt.After(out[j].ArchivedAt)
		}
		return out[i].ArchiveID > out[j].ArchiveID
	})
	return out, nil
}

func (s *Store) loadArchiveNoCtx(teamID, archiveID string) (ArchiveSnapshot, error) {
	if s == nil {
		return ArchiveSnapshot{}, errors.New("nil team store")
	}
	teamID = NormalizeTeamID(teamID)
	archiveID = sanitizeArchiveID(archiveID)
	if teamID == "" {
		return ArchiveSnapshot{}, errors.New("empty team id")
	}
	if archiveID == "" {
		return ArchiveSnapshot{}, errors.New("empty archive id")
	}
	path := filepath.Join(s.root, teamID, "archives", archiveID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return ArchiveSnapshot{}, err
	}
	var snapshot ArchiveSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return ArchiveSnapshot{}, err
	}
	return snapshot, nil
}

func (s *Store) saveArchiveSnapshot(teamID string, snapshot ArchiveSnapshot) error {
	teamID = NormalizeTeamID(teamID)
	if teamID == "" {
		return errors.New("empty team id")
	}
	snapshot.ArchiveID = sanitizeArchiveID(snapshot.ArchiveID)
	if snapshot.ArchiveID == "" {
		return errors.New("empty archive id")
	}
	path := filepath.Join(s.root, teamID, "archives", snapshot.ArchiveID+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o644)
}
