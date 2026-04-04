package team

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type TaskIndexEntry struct {
	TaskID    string    `json:"task_id"`
	Offset    int64     `json:"offset"`
	Length    int       `json:"length"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	DueAt     time.Time `json:"due_at,omitempty"`
	ChannelID string    `json:"channel_id,omitempty"`
	ContextID string    `json:"context_id,omitempty"`
	Status    string    `json:"status,omitempty"`
	Priority  string    `json:"priority,omitempty"`
	Deleted   bool      `json:"deleted,omitempty"`
}

type ArtifactIndexEntry struct {
	ArtifactID string    `json:"artifact_id"`
	Offset     int64     `json:"offset"`
	Length     int       `json:"length"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
	ChannelID  string    `json:"channel_id,omitempty"`
	TaskID     string    `json:"task_id,omitempty"`
	Kind       string    `json:"kind,omitempty"`
	Deleted    bool      `json:"deleted,omitempty"`
}

func (s *Store) hasTaskIndex(teamID string) bool {
	_, errIndex := os.Stat(s.taskIndexPath(teamID))
	_, errData := os.Stat(s.taskDataPath(teamID))
	return errIndex == nil && errData == nil
}

func (s *Store) hasArtifactIndex(teamID string) bool {
	_, errIndex := os.Stat(s.artifactIndexPath(teamID))
	_, errData := os.Stat(s.artifactDataPath(teamID))
	return errIndex == nil && errData == nil
}

func (s *Store) loadTaskIndex(teamID string) ([]TaskIndexEntry, error) {
	data, err := os.ReadFile(s.taskIndexPath(teamID))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []TaskIndexEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (s *Store) saveTaskIndex(teamID string, entries []TaskIndexEntry) error {
	path := s.taskIndexPath(teamID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o644)
}

func (s *Store) loadArtifactIndex(teamID string) ([]ArtifactIndexEntry, error) {
	data, err := os.ReadFile(s.artifactIndexPath(teamID))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []ArtifactIndexEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (s *Store) saveArtifactIndex(teamID string, entries []ArtifactIndexEntry) error {
	path := s.artifactIndexPath(teamID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o644)
}

func appendJSONLRecord(path string, value any) (int64, int, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, 0, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()
	offset, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, 0, err
	}
	body, err := json.Marshal(value)
	if err != nil {
		return 0, 0, err
	}
	body = append(body, '\n')
	if _, err := file.Write(body); err != nil {
		return 0, 0, err
	}
	return offset, len(body), nil
}

func readJSONRecordAt(path string, offset int64, length int, dest any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	buf := make([]byte, length)
	if _, err := file.ReadAt(buf, offset); err != nil {
		return err
	}
	return json.Unmarshal([]byte(strings.TrimSpace(string(buf))), dest)
}

func activeTaskIndexEntries(entries []TaskIndexEntry) []TaskIndexEntry {
	out := make([]TaskIndexEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Deleted || strings.TrimSpace(entry.TaskID) == "" {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func activeArtifactIndexEntries(entries []ArtifactIndexEntry) []ArtifactIndexEntry {
	out := make([]ArtifactIndexEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Deleted || strings.TrimSpace(entry.ArtifactID) == "" {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func taskIndexEntryFromTask(task Task, offset int64, length int) TaskIndexEntry {
	return TaskIndexEntry{
		TaskID:    task.TaskID,
		Offset:    offset,
		Length:    length,
		CreatedAt: task.CreatedAt,
		UpdatedAt: task.UpdatedAt,
		DueAt:     task.DueAt,
		ChannelID: task.ChannelID,
		ContextID: task.ContextID,
		Status:    task.Status,
		Priority:  task.Priority,
	}
}

func artifactIndexEntryFromArtifact(artifact Artifact, offset int64, length int) ArtifactIndexEntry {
	return ArtifactIndexEntry{
		ArtifactID: artifact.ArtifactID,
		Offset:     offset,
		Length:     length,
		CreatedAt:  artifact.CreatedAt,
		UpdatedAt:  artifact.UpdatedAt,
		ChannelID:  artifact.ChannelID,
		TaskID:     artifact.TaskID,
		Kind:       artifact.Kind,
	}
}

func (s *Store) appendTaskIndexedLocked(teamID string, task Task) error {
	offset, length, err := appendJSONLRecord(s.taskDataPath(teamID), task)
	if err != nil {
		return err
	}
	entries, err := s.loadTaskIndex(teamID)
	if err != nil {
		return err
	}
	entry := taskIndexEntryFromTask(task, offset, length)
	updated := false
	for i := range entries {
		if entries[i].TaskID == task.TaskID {
			entries[i] = entry
			updated = true
			break
		}
	}
	if !updated {
		entries = append(entries, entry)
	}
	return s.saveTaskIndex(teamID, entries)
}

func (s *Store) appendArtifactIndexedLocked(teamID string, artifact Artifact) error {
	offset, length, err := appendJSONLRecord(s.artifactDataPath(teamID), artifact)
	if err != nil {
		return err
	}
	entries, err := s.loadArtifactIndex(teamID)
	if err != nil {
		return err
	}
	entry := artifactIndexEntryFromArtifact(artifact, offset, length)
	updated := false
	for i := range entries {
		if entries[i].ArtifactID == artifact.ArtifactID {
			entries[i] = entry
			updated = true
			break
		}
	}
	if !updated {
		entries = append(entries, entry)
	}
	return s.saveArtifactIndex(teamID, entries)
}

func (s *Store) loadTasksFromIndex(teamID string, limit int) ([]Task, error) {
	entries, err := s.loadTaskIndex(teamID)
	if err != nil {
		return nil, err
	}
	entries = activeTaskIndexEntries(entries)
	sort.SliceStable(entries, func(i, j int) bool {
		if !entries[i].UpdatedAt.Equal(entries[j].UpdatedAt) {
			return entries[i].UpdatedAt.After(entries[j].UpdatedAt)
		}
		return entries[i].TaskID > entries[j].TaskID
	})
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	out := make([]Task, 0, len(entries))
	for _, entry := range entries {
		var task Task
		if err := readJSONRecordAt(s.taskDataPath(teamID), entry.Offset, entry.Length, &task); err != nil {
			return nil, err
		}
		out = append(out, task)
	}
	return out, nil
}

func (s *Store) loadTaskFromIndex(teamID, taskID string) (Task, error) {
	entries, err := s.loadTaskIndex(teamID)
	if err != nil {
		return Task{}, err
	}
	for _, entry := range entries {
		if entry.Deleted || entry.TaskID != taskID {
			continue
		}
		var task Task
		if err := readJSONRecordAt(s.taskDataPath(teamID), entry.Offset, entry.Length, &task); err != nil {
			return Task{}, err
		}
		return task, nil
	}
	return Task{}, os.ErrNotExist
}

func (s *Store) loadArtifactsFromIndex(teamID string, limit int) ([]Artifact, error) {
	entries, err := s.loadArtifactIndex(teamID)
	if err != nil {
		return nil, err
	}
	entries = activeArtifactIndexEntries(entries)
	sort.SliceStable(entries, func(i, j int) bool {
		if !entries[i].UpdatedAt.Equal(entries[j].UpdatedAt) {
			return entries[i].UpdatedAt.After(entries[j].UpdatedAt)
		}
		return entries[i].ArtifactID > entries[j].ArtifactID
	})
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	out := make([]Artifact, 0, len(entries))
	for _, entry := range entries {
		var artifact Artifact
		if err := readJSONRecordAt(s.artifactDataPath(teamID), entry.Offset, entry.Length, &artifact); err != nil {
			return nil, err
		}
		out = append(out, artifact)
	}
	return out, nil
}

func (s *Store) loadArtifactFromIndex(teamID, artifactID string) (Artifact, error) {
	entries, err := s.loadArtifactIndex(teamID)
	if err != nil {
		return Artifact{}, err
	}
	for _, entry := range entries {
		if entry.Deleted || entry.ArtifactID != artifactID {
			continue
		}
		var artifact Artifact
		if err := readJSONRecordAt(s.artifactDataPath(teamID), entry.Offset, entry.Length, &artifact); err != nil {
			return Artifact{}, err
		}
		return artifact, nil
	}
	return Artifact{}, os.ErrNotExist
}

func (s *Store) rewriteTaskIndexLocked(teamID string, tasks []Task) error {
	path := s.taskDataPath(teamID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	items := append([]Task(nil), tasks...)
	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].UpdatedAt.Before(items[j].UpdatedAt)
		}
		return items[i].TaskID < items[j].TaskID
	})
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	index := make([]TaskIndexEntry, 0, len(items))
	var offset int64
	for _, task := range items {
		body, err := json.Marshal(task)
		if err != nil {
			_ = tmp.Close()
			return err
		}
		body = append(body, '\n')
		if _, err := tmp.Write(body); err != nil {
			_ = tmp.Close()
			return err
		}
		index = append(index, taskIndexEntryFromTask(task, offset, len(body)))
		offset += int64(len(body))
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return s.saveTaskIndex(teamID, index)
}

func (s *Store) rewriteArtifactIndexLocked(teamID string, artifacts []Artifact) error {
	path := s.artifactDataPath(teamID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	items := append([]Artifact(nil), artifacts...)
	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].UpdatedAt.Before(items[j].UpdatedAt)
		}
		return items[i].ArtifactID < items[j].ArtifactID
	})
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	index := make([]ArtifactIndexEntry, 0, len(items))
	var offset int64
	for _, artifact := range items {
		body, err := json.Marshal(artifact)
		if err != nil {
			_ = tmp.Close()
			return err
		}
		body = append(body, '\n')
		if _, err := tmp.Write(body); err != nil {
			_ = tmp.Close()
			return err
		}
		index = append(index, artifactIndexEntryFromArtifact(artifact, offset, len(body)))
		offset += int64(len(body))
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return s.saveArtifactIndex(teamID, index)
}

func (s *Store) MigrateTasksToIndex(teamID string) error {
	if s == nil {
		return errors.New("nil team store")
	}
	teamID = NormalizeTeamID(teamID)
	if teamID == "" {
		return errors.New("empty team id")
	}
	return s.withTeamLock(teamID, func() error {
		return s.ensureTaskIndexLocked(teamID)
	})
}

func (s *Store) MigrateArtifactsToIndex(teamID string) error {
	if s == nil {
		return errors.New("nil team store")
	}
	teamID = NormalizeTeamID(teamID)
	if teamID == "" {
		return errors.New("empty team id")
	}
	return s.withTeamLock(teamID, func() error {
		return s.ensureArtifactIndexLocked(teamID)
	})
}

func (s *Store) CompactTasks(teamID string) error {
	if s == nil {
		return errors.New("nil team store")
	}
	teamID = NormalizeTeamID(teamID)
	if teamID == "" {
		return errors.New("empty team id")
	}
	return s.withTeamLock(teamID, func() error {
		if err := s.ensureTaskIndexLocked(teamID); err != nil {
			return err
		}
		tasks, err := s.loadTasksFromIndex(teamID, 0)
		if err != nil {
			return err
		}
		return s.rewriteTaskIndexLocked(teamID, tasks)
	})
}

func (s *Store) CompactArtifacts(teamID string) error {
	if s == nil {
		return errors.New("nil team store")
	}
	teamID = NormalizeTeamID(teamID)
	if teamID == "" {
		return errors.New("empty team id")
	}
	return s.withTeamLock(teamID, func() error {
		if err := s.ensureArtifactIndexLocked(teamID); err != nil {
			return err
		}
		artifacts, err := s.loadArtifactsFromIndex(teamID, 0)
		if err != nil {
			return err
		}
		return s.rewriteArtifactIndexLocked(teamID, artifacts)
	})
}

func (s *Store) ensureTaskIndex(teamID string) error {
	if s == nil {
		return errors.New("nil team store")
	}
	teamID = NormalizeTeamID(teamID)
	if teamID == "" {
		return errors.New("empty team id")
	}
	return s.withTeamLock(teamID, func() error {
		return s.ensureTaskIndexLocked(teamID)
	})
}

func (s *Store) ensureTaskIndexLocked(teamID string) error {
	if s.hasTaskIndex(teamID) {
		return nil
	}
	tasks, err := s.loadLegacyTasks(teamID)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return s.rewriteTaskIndexLocked(teamID, tasks)
}

func (s *Store) ensureArtifactIndex(teamID string) error {
	if s == nil {
		return errors.New("nil team store")
	}
	teamID = NormalizeTeamID(teamID)
	if teamID == "" {
		return errors.New("empty team id")
	}
	return s.withTeamLock(teamID, func() error {
		return s.ensureArtifactIndexLocked(teamID)
	})
}

func (s *Store) ensureArtifactIndexLocked(teamID string) error {
	if s.hasArtifactIndex(teamID) {
		return nil
	}
	artifacts, err := s.loadLegacyArtifacts(teamID)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return s.rewriteArtifactIndexLocked(teamID, artifacts)
}
