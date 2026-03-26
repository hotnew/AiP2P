package haonews

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestEnsureHistoryManifestsCreatesStableBundle(t *testing.T) {
	t.Parallel()

	store, err := OpenStore(filepath.Join(t.TempDir(), ".haonews"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	_, err = PublishMessage(store, MessageInput{
		Author:    "agent://pc75/main",
		Kind:      "post",
		Channel:   "latest.org/world",
		Title:     "PC75 market note",
		Body:      "history body",
		CreatedAt: time.Date(2026, 3, 12, 12, 0, 0, 0, time.UTC),
		Extensions: map[string]any{
			"project":    "latest.org",
			"network_id": latestOrgNetworkID,
			"topics":     []string{"pc75", "world"},
		},
	})
	if err != nil {
		t.Fatalf("publish post: %v", err)
	}
	if err := ensureHistoryManifests(store, NetworkBootstrapConfig{NetworkID: latestOrgNetworkID}, nil); err != nil {
		t.Fatalf("ensure manifests: %v", err)
	}
	manifestDirs := collectManifestDirs(t, store)
	if len(manifestDirs) != 1 {
		t.Fatalf("manifest dirs = %d, want 1", len(manifestDirs))
	}
	msg, body, err := LoadMessage(manifestDirs[0])
	if err != nil {
		t.Fatalf("load manifest message: %v", err)
	}
	if !isHistoryManifestMessage(msg) {
		t.Fatalf("message kind = %q, want manifest history", msg.Kind)
	}
	manifest, err := parseHistoryManifest(body, msg)
	if err != nil {
		t.Fatalf("parse history manifest: %v", err)
	}
	if manifest.Project != "latest.org" {
		t.Fatalf("manifest project = %q", manifest.Project)
	}
	if manifest.NetworkID != latestOrgNetworkID {
		t.Fatalf("manifest network = %q", manifest.NetworkID)
	}
	if manifest.EntryCount != 1 || len(manifest.Entries) != 1 {
		t.Fatalf("manifest entries = %d/%d, want 1", manifest.EntryCount, len(manifest.Entries))
	}
	if manifest.Page != 1 || manifest.PageSize != historyManifestPageSize || manifest.TotalPages != 1 || manifest.TotalEntries != 1 {
		t.Fatalf("manifest paging = page=%d size=%d total_pages=%d total_entries=%d", manifest.Page, manifest.PageSize, manifest.TotalPages, manifest.TotalEntries)
	}
	if err := ensureHistoryManifests(store, NetworkBootstrapConfig{NetworkID: latestOrgNetworkID}, nil); err != nil {
		t.Fatalf("ensure manifests second pass: %v", err)
	}
	manifestDirs = collectManifestDirs(t, store)
	if len(manifestDirs) != 1 {
		t.Fatalf("manifest dirs after second pass = %d, want 1", len(manifestDirs))
	}
}

func TestEnqueueHistoryManifestRefsAddsMissingBundles(t *testing.T) {
	t.Parallel()

	store, err := OpenStore(filepath.Join(t.TempDir(), ".haonews"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	ref, err := ParseSyncRef("magnet:?xt=urn:btih:93a71a010a59022c8670e06e2c92fa279f98d974&dn=test-history")
	if err != nil {
		t.Fatalf("parse sync ref: %v", err)
	}
	manifestBody, err := json.MarshalIndent(HistoryManifest{
		Protocol:    ProtocolVersion,
		Type:        historyManifestType,
		Project:     "latest.org",
		NetworkID:   latestOrgNetworkID,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		EntryCount:  1,
		Entries: []SyncAnnouncement{{
			InfoHash:  ref.InfoHash,
			Magnet:    ref.Magnet,
			Kind:      "post",
			Author:    "agent://pc74/main",
			Project:   "latest.org",
			NetworkID: latestOrgNetworkID,
			Topics:    []string{"pc75"},
		}},
	}, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	_, err = PublishMessage(store, MessageInput{
		Author:    historyManifestAuthor,
		Kind:      historyManifestKind,
		Channel:   "latest.org/history",
		Title:     "latest.org history manifest",
		Body:      string(append(manifestBody, '\n')),
		CreatedAt: time.Now().UTC(),
		Extensions: map[string]any{
			"project":       "latest.org",
			"network_id":    latestOrgNetworkID,
			"manifest_type": historyManifestType,
			"topics":        []string{"all", "pc75"},
		},
	})
	if err != nil {
		t.Fatalf("publish manifest: %v", err)
	}
	queues, err := ensureSyncLayout(store, "")
	if err != nil {
		t.Fatalf("ensure sync layout: %v", err)
	}
	added, err := enqueueHistoryManifestRefs(store, queues.HistoryPath, SyncSubscriptions{Topics: []string{"pc75"}}, latestOrgNetworkID, 0)
	if err != nil {
		t.Fatalf("enqueue from manifest: %v", err)
	}
	if added != 1 {
		t.Fatalf("added = %d, want 1", added)
	}
	data, err := os.ReadFile(queues.HistoryPath)
	if err != nil {
		t.Fatalf("read queue: %v", err)
	}
	if !containsText(string(data), ref.InfoHash) {
		t.Fatalf("queue does not include infohash %s: %s", ref.InfoHash, string(data))
	}
}

func TestEnsureHistoryManifestsSplitsIntoPages(t *testing.T) {
	t.Parallel()

	store, err := OpenStore(filepath.Join(t.TempDir(), ".haonews"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	for i := 0; i < historyManifestPageSize+1; i++ {
		_, err := PublishMessage(store, MessageInput{
			Author:    "agent://pc75/main",
			Kind:      "post",
			Channel:   "latest.org/world",
			Title:     "PC75 market note " + strconv.Itoa(i),
			Body:      "history body",
			CreatedAt: time.Date(2026, 3, 12, 12, 0, i, 0, time.UTC),
			Extensions: map[string]any{
				"project":    "latest.org",
				"network_id": latestOrgNetworkID,
				"topics":     []string{"pc75", "world"},
			},
		})
		if err != nil {
			t.Fatalf("publish post %d: %v", i, err)
		}
	}
	if err := ensureHistoryManifests(store, NetworkBootstrapConfig{NetworkID: latestOrgNetworkID}, nil); err != nil {
		t.Fatalf("ensure manifests: %v", err)
	}
	manifestDirs := collectManifestDirs(t, store)
	if len(manifestDirs) != 2 {
		t.Fatalf("manifest dirs = %d, want 2", len(manifestDirs))
	}
	pages := map[int]HistoryManifest{}
	for _, dir := range manifestDirs {
		msg, body, err := LoadMessage(dir)
		if err != nil {
			t.Fatalf("load manifest: %v", err)
		}
		manifest, err := parseHistoryManifest(body, msg)
		if err != nil {
			t.Fatalf("parse manifest: %v", err)
		}
		pages[manifest.Page] = manifest
	}
	if pages[1].EntryCount != historyManifestPageSize || !pages[1].HasMore || pages[1].NextCursor != "2" || pages[1].TotalPages != 2 {
		t.Fatalf("page1 = %+v", pages[1])
	}
	if pages[2].EntryCount != 1 || pages[2].HasMore || pages[2].NextCursor != "" || pages[2].TotalPages != 2 {
		t.Fatalf("page2 = %+v", pages[2])
	}
}

func collectManifestDirs(t *testing.T, store *Store) []string {
	t.Helper()
	entries, err := os.ReadDir(store.DataDir)
	if err != nil {
		t.Fatalf("read data dir: %v", err)
	}
	var out []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		msg, _, err := LoadMessage(filepath.Join(store.DataDir, entry.Name()))
		if err != nil {
			continue
		}
		if isHistoryManifestMessage(msg) {
			out = append(out, filepath.Join(store.DataDir, entry.Name()))
		}
	}
	return out
}

func containsText(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
