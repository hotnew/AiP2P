package aip2p

import (
	"os"
	"path/filepath"
	"strings"
)

func localBundleDayCounts(store *Store, excludeDir string) map[string]int64 {
	counts := make(map[string]int64)
	if store == nil {
		return counts
	}
	entries, err := os.ReadDir(store.DataDir)
	if err != nil {
		return counts
	}
	excludeDir = filepath.Clean(strings.TrimSpace(excludeDir))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(store.DataDir, entry.Name())
		if excludeDir != "" && filepath.Clean(dir) == excludeDir {
			continue
		}
		msg, _, err := LoadMessage(dir)
		if err != nil || isHistoryManifestMessage(msg) {
			continue
		}
		key := utcDayKey(msg.CreatedAt)
		if key == "" {
			continue
		}
		counts[key]++
	}
	return counts
}

func reserveDailyQuota(counts map[string]int64, createdAt string, maxItemsPerDay int64) bool {
	if maxItemsPerDay <= 0 {
		maxItemsPerDay = defaultMaxItemsPerDay
	}
	key := utcDayKey(createdAt)
	if key == "" {
		return true
	}
	if counts[key] >= maxItemsPerDay {
		return false
	}
	counts[key]++
	return true
}
