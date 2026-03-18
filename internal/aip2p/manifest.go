package aip2p

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/anacrolix/torrent/metainfo"
)

const (
	historyManifestKind   = "manifest"
	historyManifestType   = "history"
	historyManifestAuthor = "agent://aip2p-sync/history-manifest"
)

type HistoryManifest struct {
	Protocol    string             `json:"protocol"`
	Type        string             `json:"type"`
	Project     string             `json:"project,omitempty"`
	NetworkID   string             `json:"network_id,omitempty"`
	GeneratedAt string             `json:"generated_at"`
	EntryCount  int                `json:"entry_count"`
	Entries     []SyncAnnouncement `json:"entries"`
}

type historyManifestState struct {
	Project     string `json:"project"`
	NetworkID   string `json:"network_id"`
	BodySHA256  string `json:"body_sha256"`
	InfoHash    string `json:"infohash"`
	ContentDir  string `json:"content_dir"`
	TorrentFile string `json:"torrent_file"`
}

func ensureHistoryManifests(store *Store, netCfg NetworkBootstrapConfig, listenAddrs []net.Addr) error {
	announcements, err := localAnnouncements(store)
	if err != nil {
		return err
	}
	grouped := map[string][]SyncAnnouncement{}
	for _, announcement := range announcements {
		announcement = normalizeAnnouncement(announcement)
		if announcement.InfoHash == "" || announcement.Magnet == "" {
			continue
		}
		if strings.EqualFold(announcement.Kind, historyManifestKind) {
			continue
		}
		if netCfg.NetworkID != "" && announcement.NetworkID != "" && !strings.EqualFold(announcement.NetworkID, netCfg.NetworkID) {
			continue
		}
		project := strings.TrimSpace(announcement.Project)
		if project == "" {
			continue
		}
		if announcement.NetworkID == "" {
			announcement.NetworkID = netCfg.NetworkID
		}
		announcement.Magnet = withPeerHints(announcement.Magnet, listenAddrs, netCfg.LANPeers)
		grouped[project] = append(grouped[project], announcement)
	}
	for project, entries := range grouped {
		if err := ensureHistoryManifest(store, project, netCfg.NetworkID, entries); err != nil {
			return err
		}
	}
	return nil
}

func ensureHistoryManifest(store *Store, project, networkID string, entries []SyncAnnouncement) error {
	if len(entries) == 0 {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt < entries[j].CreatedAt
	})
	manifest := HistoryManifest{
		Protocol:    ProtocolVersion,
		Type:        historyManifestType,
		Project:     strings.TrimSpace(project),
		NetworkID:   normalizeNetworkID(networkID),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		EntryCount:  len(entries),
		Entries:     make([]SyncAnnouncement, 0, len(entries)),
	}
	topicsSeen := map[string]struct{}{reservedTopicAll: {}}
	topics := []string{reservedTopicAll}
	for _, entry := range entries {
		entry.NetworkID = manifest.NetworkID
		entry = normalizeAnnouncement(entry)
		manifest.Entries = append(manifest.Entries, entry)
		for _, topic := range entry.Topics {
			key := strings.ToLower(strings.TrimSpace(topic))
			if key == "" {
				continue
			}
			if _, ok := topicsSeen[key]; ok {
				continue
			}
			topicsSeen[key] = struct{}{}
			topics = append(topics, topic)
		}
	}
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	bodySHA := sha256.Sum256(body)
	bodyHash := hex.EncodeToString(bodySHA[:])
	statePath := historyManifestStatePath(store, manifest.Project, manifest.NetworkID)
	state, _ := loadHistoryManifestState(statePath)
	if state.BodySHA256 == bodyHash && state.ContentDir != "" && state.TorrentFile != "" {
		if _, err := os.Stat(state.ContentDir); err == nil {
			if _, err := os.Stat(state.TorrentFile); err == nil {
				return nil
			}
		}
	}
	result, err := PublishMessage(store, MessageInput{
		Kind:      historyManifestKind,
		Author:    historyManifestAuthor,
		Channel:   manifest.Project + "/history",
		Title:     manifest.Project + " history manifest",
		Body:      string(body),
		Tags:      []string{"history-manifest"},
		CreatedAt: time.Now().UTC(),
		Extensions: map[string]any{
			"project":       manifest.Project,
			"network_id":    manifest.NetworkID,
			"manifest_type": historyManifestType,
			"entry_count":   manifest.EntryCount,
			"topics":        topics,
		},
	})
	if err != nil {
		return err
	}
	if state.ContentDir != "" && state.ContentDir != result.ContentDir {
		_ = os.RemoveAll(state.ContentDir)
	}
	if state.TorrentFile != "" && state.TorrentFile != result.TorrentFile {
		_ = os.Remove(state.TorrentFile)
	}
	return writeHistoryManifestState(statePath, historyManifestState{
		Project:     manifest.Project,
		NetworkID:   manifest.NetworkID,
		BodySHA256:  bodyHash,
		InfoHash:    result.InfoHash,
		ContentDir:  result.ContentDir,
		TorrentFile: result.TorrentFile,
	})
}

func enqueueHistoryManifestRefs(store *Store, queuePath string, subscriptions SyncSubscriptions, networkID string) (int, error) {
	entries, err := os.ReadDir(store.DataDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	added := 0
	dayCounts := localBundleDayCounts(store, "")
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(store.DataDir, entry.Name())
		msg, body, err := LoadMessage(dir)
		if err != nil || !isHistoryManifestMessage(msg) {
			continue
		}
		manifest, err := parseHistoryManifest(body, msg)
		if err != nil {
			continue
		}
		if networkID != "" && manifest.NetworkID != "" && !strings.EqualFold(manifest.NetworkID, networkID) {
			continue
		}
		for _, announcement := range manifest.Entries {
			announcement = normalizeAnnouncement(announcement)
			if announcement.NetworkID == "" {
				announcement.NetworkID = manifest.NetworkID
			}
			if networkID != "" && announcement.NetworkID != "" && !strings.EqualFold(announcement.NetworkID, networkID) {
				continue
			}
			if !matchesAnnouncement(announcement, subscriptions) {
				continue
			}
			ref, err := syncRefFromAnnouncement(announcement)
			if err != nil || ref.InfoHash == "" {
				continue
			}
			if hasLocalTorrent(store, ref.InfoHash) {
				continue
			}
			if !reserveDailyQuota(dayCounts, announcement.CreatedAt, subscriptions.MaxItemsPerDay) {
				continue
			}
			enqueued, err := enqueueSyncRef(queuePath, ref)
			if err != nil {
				return added, err
			}
			if enqueued {
				added++
			}
		}
	}
	return added, nil
}

func isHistoryManifestMessage(msg Message) bool {
	if !strings.EqualFold(strings.TrimSpace(msg.Kind), historyManifestKind) {
		return false
	}
	return strings.EqualFold(nestedString(msg.Extensions, "manifest_type"), historyManifestType)
}

func parseHistoryManifest(body string, msg Message) (HistoryManifest, error) {
	var manifest HistoryManifest
	if err := json.Unmarshal([]byte(body), &manifest); err != nil {
		return HistoryManifest{}, err
	}
	manifest.Protocol = strings.TrimSpace(manifest.Protocol)
	manifest.Type = strings.TrimSpace(manifest.Type)
	manifest.Project = strings.TrimSpace(manifest.Project)
	manifest.NetworkID = normalizeNetworkID(manifest.NetworkID)
	if manifest.Project == "" {
		manifest.Project = nestedString(msg.Extensions, "project")
	}
	if manifest.NetworkID == "" {
		manifest.NetworkID = nestedString(msg.Extensions, "network_id")
	}
	if !strings.EqualFold(manifest.Type, historyManifestType) {
		return HistoryManifest{}, errors.New("unsupported manifest type")
	}
	for index := range manifest.Entries {
		manifest.Entries[index].Project = manifest.Project
		if manifest.Entries[index].NetworkID == "" {
			manifest.Entries[index].NetworkID = manifest.NetworkID
		}
		manifest.Entries[index] = normalizeAnnouncement(manifest.Entries[index])
	}
	return manifest, nil
}

func historyManifestStatePath(store *Store, project, networkID string) string {
	name := slugify(project)
	if name == "" {
		name = "project"
	}
	if networkID != "" {
		name += "-" + networkID[:12]
	}
	return filepath.Join(store.Root, "sync", "manifests", name+".json")
}

func loadHistoryManifestState(path string) (historyManifestState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return historyManifestState{}, err
	}
	var state historyManifestState
	if err := json.Unmarshal(data, &state); err != nil {
		return historyManifestState{}, err
	}
	return state, nil
}

func writeHistoryManifestState(path string, state historyManifestState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func syncRefFromAnnouncement(announcement SyncAnnouncement) (SyncRef, error) {
	if strings.TrimSpace(announcement.Magnet) != "" {
		return ParseSyncRef(announcement.Magnet)
	}
	return ParseSyncRef(announcement.InfoHash)
}

func hasLocalTorrent(store *Store, infoHash string) bool {
	if strings.TrimSpace(infoHash) == "" {
		return false
	}
	_, err := os.Stat(store.TorrentPath(infoHash))
	return err == nil
}

func hasCompleteLocalBundle(store *Store, infoHash string) bool {
	infoHash = strings.TrimSpace(strings.ToLower(infoHash))
	if infoHash == "" {
		return false
	}
	torrentPath := store.TorrentPath(infoHash)
	if _, err := os.Stat(torrentPath); err != nil {
		return false
	}
	mi, err := metainfo.LoadFromFile(torrentPath)
	if err != nil {
		return false
	}
	info, err := mi.UnmarshalInfo()
	if err != nil {
		return false
	}
	contentDir := filepath.Join(store.DataDir, info.BestName())
	_, _, err = LoadMessage(contentDir)
	return err == nil
}
