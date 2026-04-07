package team

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (s *Store) CountTasksByStatusCtx(ctx context.Context, teamID string) (map[string]int, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	teamID = NormalizeTeamID(teamID)
	if teamID == "" {
		return nil, errors.New("empty team id")
	}
	counts := make(map[string]int)
	if s.hasTaskIndex(teamID) {
		entries, err := s.loadTaskIndex(teamID)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.Deleted {
				continue
			}
			status := normalizeTaskStatus(entry.Status)
			if status == "" {
				status = TaskStateOpen
			}
			counts[status]++
		}
		return counts, nil
	}
	tasks, err := s.loadTasksNoCtx(teamID, 0)
	if err != nil {
		return nil, err
	}
	for _, task := range tasks {
		status := normalizeTaskStatus(task.Status)
		if status == "" {
			status = TaskStateOpen
		}
		counts[status]++
	}
	return counts, nil
}

func (s *Store) CountRecentMessagesCtx(ctx context.Context, teamID, channelID string, since time.Time) (int, error) {
	if err := ctxErr(ctx); err != nil {
		return 0, err
	}
	teamID = NormalizeTeamID(teamID)
	channelID = normalizeChannelID(channelID)
	if teamID == "" {
		return 0, errors.New("empty team id")
	}
	if channelID == "" {
		channelID = "main"
	}
	if since.IsZero() {
		return 0, nil
	}
	if s.isShardedChannel(teamID, channelID) {
		return s.countRecentMessagesFromShards(teamID, channelID, since)
	}
	return s.countRecentMessagesFromJSONL(s.channelPath(teamID, channelID), since)
}

func (s *Store) LoadRecentDeliveriesCtx(ctx context.Context, teamID string, limit int) ([]WebhookDeliveryRecord, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	teamID = NormalizeTeamID(teamID)
	if teamID == "" {
		return nil, errors.New("empty team id")
	}
	records, err := s.loadWebhookDeliveriesNoCtx(teamID)
	if err != nil {
		return nil, err
	}
	sortWebhookDeliveries(records)
	if limit > 0 && len(records) > limit {
		records = records[:limit]
	}
	return records, nil
}

func (s *Store) countRecentMessagesFromShards(teamID, channelID string, since time.Time) (int, error) {
	dir := s.channelShardDir(teamID, channelID)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	sort.Slice(paths, func(i, j int) bool {
		return filepath.Base(paths[i]) > filepath.Base(paths[j])
	})
	total := 0
	for _, path := range paths {
		lastAt, err := latestMessageTimestampFromJSONL(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if !lastAt.IsZero() && lastAt.Before(since) {
			break
		}
		count, err := s.countRecentMessagesFromJSONL(path, since)
		if err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

func (s *Store) countRecentMessagesFromJSONL(path string, since time.Time) (int, error) {
	lines, err := readLastJSONLLines(path, 512)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	count, allRecent, err := countRecentMessagesInLines(lines, since, path)
	if err != nil {
		return 0, err
	}
	if !allRecent || len(lines) < 512 {
		return count, nil
	}
	lines, err = readAllJSONLLines(path)
	if err != nil {
		return 0, err
	}
	count, _, err = countRecentMessagesInLines(lines, since, path)
	return count, err
}

func countRecentMessagesInLines(lines []string, since time.Time, path string) (count int, allRecent bool, err error) {
	allRecent = len(lines) > 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var msg Message
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			logTeamEvent("corrupt_jsonl_line", "path", path, "error", err)
			continue
		}
		if msg.CreatedAt.IsZero() || msg.CreatedAt.Before(since) {
			allRecent = false
			continue
		}
		count++
	}
	return count, allRecent, nil
}
