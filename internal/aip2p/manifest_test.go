package aip2p

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEnsureHistoryManifestsCreatesStableBundle(t *testing.T) {
	t.Parallel()

	store, err := OpenStore(filepath.Join(t.TempDir(), ".aip2p"))
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

	store, err := OpenStore(filepath.Join(t.TempDir(), ".aip2p"))
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
	queuePath, err := ensureSyncLayout(store, "")
	if err != nil {
		t.Fatalf("ensure sync layout: %v", err)
	}
	added, err := enqueueHistoryManifestRefs(store, queuePath, SyncSubscriptions{Topics: []string{"pc75"}}, latestOrgNetworkID)
	if err != nil {
		t.Fatalf("enqueue from manifest: %v", err)
	}
	if added != 1 {
		t.Fatalf("added = %d, want 1", added)
	}
	data, err := os.ReadFile(queuePath)
	if err != nil {
		t.Fatalf("read queue: %v", err)
	}
	if !containsText(string(data), ref.InfoHash) {
		t.Fatalf("queue does not include infohash %s: %s", ref.InfoHash, string(data))
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
