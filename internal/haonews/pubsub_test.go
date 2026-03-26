package haonews

import (
	"testing"
	"time"
)

func TestSubscribedAnnouncementTopics(t *testing.T) {
	t.Parallel()

	networkID := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	topics := subscribedAnnouncementTopics(networkID, SyncSubscriptions{
		Topics: []string{"world", "WORLD"},
		Tags:   []string{"breaking"},
	})
	if len(topics) != 2 {
		t.Fatalf("topics len = %d, want 2", len(topics))
	}
	if topics[0] != "haonews/announce/"+networkID+"/topic/world" && topics[1] != "haonews/announce/"+networkID+"/topic/world" {
		t.Fatalf("missing topic subscription: %v", topics)
	}
	if topics[0] != "haonews/announce/"+networkID+"/tag/breaking" && topics[1] != "haonews/announce/"+networkID+"/tag/breaking" {
		t.Fatalf("missing tag subscription: %v", topics)
	}
}

func TestMatchesAnnouncement(t *testing.T) {
	t.Parallel()

	announcement := SyncAnnouncement{
		Channel:   "latest.org/world",
		Author:    "agent://pc75/openclaw01",
		NetworkID: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Topics:    []string{"world", "pc75"},
		Tags:      []string{"breaking"},
	}
	if !matchesAnnouncement(announcement, SyncSubscriptions{Topics: []string{"pc75"}}) {
		t.Fatal("expected topic match")
	}
	if !matchesAnnouncement(announcement, SyncSubscriptions{Channels: []string{"latest.org/world"}}) {
		t.Fatal("expected channel match")
	}
	if !matchesAnnouncement(announcement, SyncSubscriptions{Authors: []string{"agent://pc75/openclaw01"}}) {
		t.Fatal("expected author match")
	}
	if matchesAnnouncement(announcement, SyncSubscriptions{Topics: []string{"markets"}}) {
		t.Fatal("unexpected topic match")
	}
}

func TestMatchesHistoryAnnouncementUsesHistorySelectors(t *testing.T) {
	t.Parallel()

	announcement := SyncAnnouncement{
		Channel:   "hao.news/world",
		Author:    "agent://pc75/openclaw01",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Topics:    []string{"world", "energy"},
	}
	if !matchesHistoryAnnouncement(announcement, SyncSubscriptions{HistoryAuthors: []string{"agent://pc75/openclaw01"}}) {
		t.Fatal("expected history author match")
	}
	if matchesHistoryAnnouncement(announcement, SyncSubscriptions{HistoryAuthors: []string{"agent://pc76/main"}}) {
		t.Fatal("unexpected history author match")
	}
	if !matchesHistoryAnnouncement(announcement, SyncSubscriptions{HistoryTopics: []string{"energy"}}) {
		t.Fatal("expected history topic match")
	}
}

func TestMatchesAnnouncementFiltersByMaxAgeDays(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	announcement := SyncAnnouncement{
		Channel:   "latest.org/world",
		CreatedAt: now.Add(-48 * time.Hour).Format(time.RFC3339),
		Topics:    []string{"world", "pc75"},
	}
	if matchesAnnouncement(announcement, SyncSubscriptions{Topics: []string{"all"}, MaxAgeDays: 1}) {
		t.Fatal("expected stale announcement to be filtered")
	}
	if !matchesAnnouncement(announcement, SyncSubscriptions{Topics: []string{"all"}, MaxAgeDays: 3}) {
		t.Fatal("expected announcement within max age")
	}
}

func TestMatchesAnnouncementFiltersByMaxBundleMB(t *testing.T) {
	t.Parallel()

	announcement := SyncAnnouncement{
		SizeBytes: 12 * 1024 * 1024,
		Topics:    []string{"world"},
	}
	if matchesAnnouncement(announcement, SyncSubscriptions{Topics: []string{"all"}, MaxBundleMB: 10}) {
		t.Fatal("expected oversized announcement to be filtered")
	}
	if !matchesAnnouncement(announcement, SyncSubscriptions{Topics: []string{"all"}, MaxBundleMB: 20}) {
		t.Fatal("expected announcement within size limit")
	}
}
