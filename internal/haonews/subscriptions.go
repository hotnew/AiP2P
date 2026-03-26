package haonews

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"
)

const defaultMaxAgeDays = 99999999
const defaultMaxBundleMB = 10
const defaultMaxItemsPerDay int64 = 999999999999
const defaultHistoryDays = 7
const defaultHistoryMaxItems = 500

type SyncSubscriptions struct {
	Channels        []string `json:"channels"`
	Topics          []string `json:"topics"`
	Tags            []string `json:"tags"`
	Authors         []string `json:"authors,omitempty"`
	MaxAgeDays      int      `json:"max_age_days"`
	MaxBundleMB     int      `json:"max_bundle_mb"`
	MaxItemsPerDay  int64    `json:"max_items_per_day"`
	HistoryDays     int      `json:"history_days,omitempty"`
	HistoryMaxItems int      `json:"history_max_items,omitempty"`
	HistoryChannels []string `json:"history_channels,omitempty"`
	HistoryTopics   []string `json:"history_topics,omitempty"`
	HistoryAuthors  []string `json:"history_authors,omitempty"`
}

func LoadSyncSubscriptions(path string) (SyncSubscriptions, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return SyncSubscriptions{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SyncSubscriptions{}, nil
		}
		return SyncSubscriptions{}, err
	}
	var rules SyncSubscriptions
	if err := json.Unmarshal(data, &rules); err != nil {
		return SyncSubscriptions{}, err
	}
	rules.Normalize()
	return rules, nil
}

func (r *SyncSubscriptions) Normalize() {
	if r == nil {
		return
	}
	r.Channels = uniqueFold(r.Channels)
	r.Topics = uniqueFold(r.Topics)
	r.Tags = uniqueFold(r.Tags)
	r.Authors = uniqueFold(r.Authors)
	r.HistoryChannels = uniqueFold(r.HistoryChannels)
	r.HistoryTopics = uniqueFold(r.HistoryTopics)
	r.HistoryAuthors = uniqueFold(r.HistoryAuthors)
	if r.MaxAgeDays <= 0 {
		r.MaxAgeDays = defaultMaxAgeDays
	}
	if r.MaxBundleMB <= 0 {
		r.MaxBundleMB = defaultMaxBundleMB
	}
	if r.MaxItemsPerDay <= 0 {
		r.MaxItemsPerDay = defaultMaxItemsPerDay
	}
}

func (r SyncSubscriptions) Empty() bool {
	r.Normalize()
	return len(r.Channels) == 0 && len(r.Topics) == 0 && len(r.Tags) == 0 && len(r.Authors) == 0 &&
		len(r.HistoryChannels) == 0 && len(r.HistoryTopics) == 0 && len(r.HistoryAuthors) == 0 &&
		r.MaxAgeDays >= defaultMaxAgeDays && r.MaxBundleMB >= defaultMaxBundleMB && r.MaxItemsPerDay >= defaultMaxItemsPerDay
}

func (r SyncSubscriptions) historyDays() int {
	if r.HistoryDays <= 0 {
		return defaultHistoryDays
	}
	return r.HistoryDays
}

func (r SyncSubscriptions) historyMaxItems() int {
	if r.HistoryMaxItems <= 0 {
		return defaultHistoryMaxItems
	}
	return r.HistoryMaxItems
}

func (r SyncSubscriptions) hasHistorySelectors() bool {
	r.Normalize()
	return len(r.HistoryChannels) > 0 || len(r.HistoryTopics) > 0 || len(r.HistoryAuthors) > 0
}

func matchesHistoryAnnouncement(announcement SyncAnnouncement, rules SyncSubscriptions) bool {
	rules.Normalize()
	if !withinMaxAge(announcement.CreatedAt, rules.MaxAgeDays) {
		return false
	}
	if !withinMaxBundleSize(announcement.SizeBytes, rules.MaxBundleMB) {
		return false
	}
	if rules.hasHistorySelectors() {
		if containsFold(rules.HistoryTopics, reservedTopicAll) {
			return true
		}
		if containsFold(rules.HistoryChannels, announcement.Channel) {
			return true
		}
		if containsFold(rules.HistoryAuthors, announcement.Author) {
			return true
		}
		for _, topic := range announcement.Topics {
			if containsFold(rules.HistoryTopics, topic) {
				return true
			}
		}
		return false
	}
	return matchesAnnouncement(announcement, rules)
}

func uniqueFold(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func containsFold(items []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), target) {
			return true
		}
	}
	return false
}

func withinMaxAge(createdAt string, maxAgeDays int) bool {
	if maxAgeDays <= 0 {
		maxAgeDays = defaultMaxAgeDays
	}
	createdAt = strings.TrimSpace(createdAt)
	if createdAt == "" {
		return true
	}
	parsed, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return true
	}
	maxAge := time.Duration(maxAgeDays) * 24 * time.Hour
	return time.Since(parsed.UTC()) <= maxAge
}

func withinMaxBundleSize(sizeBytes int64, maxBundleMB int) bool {
	if maxBundleMB <= 0 {
		maxBundleMB = defaultMaxBundleMB
	}
	if sizeBytes <= 0 {
		return true
	}
	return sizeBytes <= int64(maxBundleMB)*1024*1024
}

func utcDayKey(createdAt string) string {
	createdAt = strings.TrimSpace(createdAt)
	if createdAt == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return ""
	}
	return parsed.UTC().Format("2006-01-02")
}
