package haonews

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	lanPeerSuccessWindow   = 24 * time.Hour
	lanPeerFailureCooldown = 10 * time.Minute
)

type lanBootstrapResponse struct {
	NetworkID       string   `json:"network_id"`
	PeerID          string   `json:"peer_id"`
	DialAddrs       []string `json:"dial_addrs"`
	BitTorrentNodes []string `json:"bittorrent_nodes"`
	ExplainDetail   struct {
		PrimaryHost string `json:"primary_host"`
	} `json:"explain_detail"`
}

type lanHistoryManifestResponse struct {
	Project          string             `json:"project"`
	Version          string             `json:"version"`
	NetworkID        string             `json:"network_id"`
	ManifestInfoHash string             `json:"manifest_infohash"`
	GeneratedAt      string             `json:"generated_at"`
	Page             int                `json:"page,omitempty"`
	PageSize         int                `json:"page_size,omitempty"`
	TotalEntries     int                `json:"total_entries,omitempty"`
	TotalPages       int                `json:"total_pages,omitempty"`
	Cursor           string             `json:"cursor,omitempty"`
	NextCursor       string             `json:"next_cursor,omitempty"`
	HasMore          bool               `json:"has_more,omitempty"`
	EntryCount       int                `json:"entry_count"`
	Entries          []SyncAnnouncement `json:"entries"`
}

type lanPeerHealthCache struct {
	Entries map[string]lanPeerHealthEntry `json:"entries,omitempty"`
}

type lanPeerHealthEntry struct {
	LastSuccessAt       time.Time `json:"last_success_at,omitempty"`
	LastFailureAt       time.Time `json:"last_failure_at,omitempty"`
	ConsecutiveFailure  int       `json:"consecutive_failure,omitempty"`
	LastError           string    `json:"last_error,omitempty"`
	ObservedPrimaryHost string    `json:"observed_primary_host,omitempty"`
	ObservedPrimaryFrom string    `json:"observed_primary_from,omitempty"`
}

type LANPeerHealthStatus struct {
	Kind                string     `json:"kind"`
	Peer                string     `json:"peer"`
	State               string     `json:"state"`
	ObservedPrimaryHost string     `json:"observed_primary_host,omitempty"`
	ObservedPrimaryFrom string     `json:"observed_primary_from,omitempty"`
	LastSuccessAt       *time.Time `json:"last_success_at,omitempty"`
	LastFailureAt       *time.Time `json:"last_failure_at,omitempty"`
	ConsecutiveFailure  int        `json:"consecutive_failure,omitempty"`
	LastError           string     `json:"last_error,omitempty"`
}

func resolveLANBootstrapPeers(ctx context.Context, cfg NetworkBootstrapConfig) ([]string, error) {
	out := make([]string, 0, len(cfg.LANPeers))
	var errs []string
	seen := make(map[string]struct{})
	cache, err := loadLANPeerHealthCache(cfg)
	if err != nil {
		errs = append(errs, fmt.Sprintf("load lan peer health cache: %v", err))
		cache = &lanPeerHealthCache{}
	}
	for _, value := range sortLANPeerCandidates(cfg.LANPeers, cache, "lan_peer", time.Now().UTC()) {
		peers, observedHost, err := fetchLANBootstrapPeer(ctx, cache.bootstrapTargets("lan_peer", value), value, cfg.NetworkID)
		if err != nil {
			cache.recordFailure("lan_peer", value, err)
			errs = append(errs, err.Error())
			continue
		}
		cache.recordSuccess("lan_peer", value, observedHost)
		for _, peerValue := range peers {
			if _, ok := seen[peerValue]; ok {
				continue
			}
			seen[peerValue] = struct{}{}
			out = append(out, peerValue)
		}
	}
	if err := saveLANPeerHealthCache(cfg, cache); err != nil {
		errs = append(errs, fmt.Sprintf("save lan peer health cache: %v", err))
	}
	if len(errs) > 0 {
		return out, errors.New(strings.Join(errs, "; "))
	}
	return out, nil
}

func fetchLANBootstrapPeer(ctx context.Context, targets []string, configuredValue, expectedNetworkID string) ([]string, string, error) {
	var errs []string
	for _, target := range targets {
		payload, err := fetchLANBootstrapPayload(ctx, target, configuredValue, expectedNetworkID, true)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		out := make([]string, 0, len(payload.DialAddrs))
		for _, addr := range payload.DialAddrs {
			addr = strings.TrimSpace(addr)
			if addr == "" {
				continue
			}
			if !strings.Contains(addr, "/p2p/") {
				addr += "/p2p/" + payload.PeerID
			}
			out = append(out, addr)
		}
		if len(out) == 0 {
			errs = append(errs, fmt.Sprintf("lan_peer %q returned no dialable addresses", configuredValue))
			continue
		}
		return out, normalizeObservedPrimaryHost(payload.ExplainDetail.PrimaryHost), nil
	}
	return nil, "", errors.New(strings.Join(errs, "; "))
}

func resolveLANTorrentRouters(ctx context.Context, cfg NetworkBootstrapConfig) ([]string, error) {
	out := make([]string, 0, len(cfg.LANTorrentPeers))
	var errs []string
	seen := make(map[string]struct{})
	cache, err := loadLANPeerHealthCache(cfg)
	if err != nil {
		errs = append(errs, fmt.Sprintf("load lan bt health cache: %v", err))
		cache = &lanPeerHealthCache{}
	}
	for _, value := range sortLANPeerCandidates(cfg.LANTorrentPeers, cache, "lan_bt_peer", time.Now().UTC()) {
		nodes, observedHost, err := fetchLANTorrentRouters(ctx, cache.bootstrapTargets("lan_bt_peer", value), value, cfg.NetworkID)
		if err != nil {
			cache.recordFailure("lan_bt_peer", value, err)
			errs = append(errs, err.Error())
			continue
		}
		cache.recordSuccess("lan_bt_peer", value, observedHost)
		for _, node := range nodes {
			if _, ok := seen[node]; ok {
				continue
			}
			seen[node] = struct{}{}
			out = append(out, node)
		}
	}
	if err := saveLANPeerHealthCache(cfg, cache); err != nil {
		errs = append(errs, fmt.Sprintf("save lan bt health cache: %v", err))
	}
	if len(errs) > 0 {
		return out, errors.New(strings.Join(errs, "; "))
	}
	return out, nil
}

func ReadLANPeerHealthStatus(cfg NetworkBootstrapConfig) ([]LANPeerHealthStatus, []LANPeerHealthStatus, error) {
	cache, err := loadLANPeerHealthCache(cfg)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now().UTC()
	return buildLANPeerHealthStatus(cfg.LANPeers, cache, "lan_peer", now), buildLANPeerHealthStatus(cfg.LANTorrentPeers, cache, "lan_bt_peer", now), nil
}

func lanPeerHealthCachePath(cfg NetworkBootstrapConfig) string {
	if strings.TrimSpace(cfg.Path) == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(cfg.Path), "lan_peer_health.json")
}

func loadLANPeerHealthCache(cfg NetworkBootstrapConfig) (*lanPeerHealthCache, error) {
	path := lanPeerHealthCachePath(cfg)
	if path == "" {
		return &lanPeerHealthCache{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &lanPeerHealthCache{}, nil
		}
		return nil, err
	}
	var cache lanPeerHealthCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	if cache.Entries == nil {
		cache.Entries = make(map[string]lanPeerHealthEntry)
	}
	return &cache, nil
}

func saveLANPeerHealthCache(cfg NetworkBootstrapConfig, cache *lanPeerHealthCache) error {
	path := lanPeerHealthCachePath(cfg)
	if path == "" || cache == nil {
		return nil
	}
	if cache.Entries == nil {
		cache.Entries = make(map[string]lanPeerHealthEntry)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func sortLANPeerCandidates(values []string, cache *lanPeerHealthCache, kind string, now time.Time) []string {
	type candidate struct {
		value       string
		index       int
		group       int
		lastSuccess time.Time
		lastFailure time.Time
	}

	items := make([]candidate, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for idx, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		entry := cache.entry(kind, value)
		group := 1
		if hasRecentSuccess(entry, now) {
			group = 0
		} else if isInFailureCooldown(entry, now) {
			group = 2
		}
		items = append(items, candidate{
			value:       value,
			index:       idx,
			group:       group,
			lastSuccess: entry.LastSuccessAt,
			lastFailure: entry.LastFailureAt,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		if left.group != right.group {
			return left.group < right.group
		}
		if left.group == 0 && !left.lastSuccess.Equal(right.lastSuccess) {
			return left.lastSuccess.After(right.lastSuccess)
		}
		if left.group == 2 && !left.lastFailure.Equal(right.lastFailure) {
			return left.lastFailure.Before(right.lastFailure)
		}
		return left.index < right.index
	})
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.value)
	}
	return out
}

func buildLANPeerHealthStatus(values []string, cache *lanPeerHealthCache, kind string, now time.Time) []LANPeerHealthStatus {
	ordered := sortLANPeerCandidates(values, cache, kind, now)
	out := make([]LANPeerHealthStatus, 0, len(ordered))
	for _, value := range ordered {
		entry := cache.entry(kind, value)
		item := LANPeerHealthStatus{
			Kind:                kind,
			Peer:                value,
			State:               lanPeerHealthState(entry, now),
			ObservedPrimaryHost: normalizeObservedPrimaryHost(entry.ObservedPrimaryHost),
			ObservedPrimaryFrom: strings.TrimSpace(entry.ObservedPrimaryFrom),
			ConsecutiveFailure:  entry.ConsecutiveFailure,
			LastError:           strings.TrimSpace(entry.LastError),
		}
		if !entry.LastSuccessAt.IsZero() {
			ts := entry.LastSuccessAt
			item.LastSuccessAt = &ts
		}
		if !entry.LastFailureAt.IsZero() {
			ts := entry.LastFailureAt
			item.LastFailureAt = &ts
		}
		out = append(out, item)
	}
	return out
}

func (c *lanPeerHealthCache) entry(kind, value string) lanPeerHealthEntry {
	if c == nil || c.Entries == nil {
		return lanPeerHealthEntry{}
	}
	return c.Entries[lanPeerHealthKey(kind, value)]
}

func (c *lanPeerHealthCache) recordSuccess(kind, value, observedHost string) {
	if c == nil {
		return
	}
	if c.Entries == nil {
		c.Entries = make(map[string]lanPeerHealthEntry)
	}
	entry := c.Entries[lanPeerHealthKey(kind, value)]
	entry.LastSuccessAt = time.Now().UTC()
	entry.ConsecutiveFailure = 0
	entry.LastError = ""
	if observedHost = normalizeObservedPrimaryHost(observedHost); observedHost != "" {
		entry.ObservedPrimaryHost = observedHost
		entry.ObservedPrimaryFrom = strings.TrimSpace(kind)
		c.propagateObservedPrimaryHost(kind, value, observedHost)
	}
	c.Entries[lanPeerHealthKey(kind, value)] = entry
}

func (c *lanPeerHealthCache) recordFailure(kind, value string, err error) {
	if c == nil {
		return
	}
	if c.Entries == nil {
		c.Entries = make(map[string]lanPeerHealthEntry)
	}
	entry := c.Entries[lanPeerHealthKey(kind, value)]
	entry.LastFailureAt = time.Now().UTC()
	entry.ConsecutiveFailure++
	if err != nil {
		entry.LastError = err.Error()
	}
	c.Entries[lanPeerHealthKey(kind, value)] = entry
}

func lanPeerHealthKey(kind, value string) string {
	return strings.TrimSpace(kind) + "|" + strings.TrimSpace(value)
}

func hasRecentSuccess(entry lanPeerHealthEntry, now time.Time) bool {
	if entry.LastSuccessAt.IsZero() {
		return false
	}
	if !entry.LastFailureAt.IsZero() && !entry.LastSuccessAt.After(entry.LastFailureAt) {
		return false
	}
	return now.Sub(entry.LastSuccessAt) <= lanPeerSuccessWindow
}

func isInFailureCooldown(entry lanPeerHealthEntry, now time.Time) bool {
	if entry.LastFailureAt.IsZero() {
		return false
	}
	if !entry.LastSuccessAt.IsZero() && entry.LastSuccessAt.After(entry.LastFailureAt) {
		return false
	}
	return now.Sub(entry.LastFailureAt) <= lanPeerFailureCooldown
}

func lanPeerHealthState(entry lanPeerHealthEntry, now time.Time) string {
	switch {
	case hasRecentSuccess(entry, now):
		return "preferred"
	case isInFailureCooldown(entry, now):
		return "cooldown"
	case !entry.LastFailureAt.IsZero():
		return "degraded"
	default:
		return "new"
	}
}

func fetchLANTorrentRouters(ctx context.Context, targets []string, configuredValue, expectedNetworkID string) ([]string, string, error) {
	var errs []string
	for _, target := range targets {
		payload, err := fetchLANBootstrapPayload(ctx, target, configuredValue, expectedNetworkID, false)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		out := make([]string, 0, len(payload.BitTorrentNodes))
		for _, node := range payload.BitTorrentNodes {
			if node = strings.TrimSpace(node); node != "" {
				out = append(out, node)
			}
		}
		if len(out) == 0 {
			errs = append(errs, fmt.Sprintf("lan_bt_peer %q returned no bittorrent_nodes", configuredValue))
			continue
		}
		return out, normalizeObservedPrimaryHost(payload.ExplainDetail.PrimaryHost), nil
	}
	return nil, "", errors.New(strings.Join(errs, "; "))
}

func fetchLANBootstrapPayload(ctx context.Context, target, configuredValue, expectedNetworkID string, requirePeerID bool) (lanBootstrapResponse, error) {
	endpoint, err := lanBootstrapEndpoint(target)
	if err != nil {
		return lanBootstrapResponse{}, fmt.Errorf("lan_peer %q: %w", configuredValue, err)
	}
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return lanBootstrapResponse{}, fmt.Errorf("lan_peer %q request: %w", configuredValue, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return lanBootstrapResponse{}, fmt.Errorf("lan_peer %q query %s: %w", configuredValue, endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return lanBootstrapResponse{}, fmt.Errorf("lan_peer %q query %s: status %d", configuredValue, endpoint, resp.StatusCode)
	}
	var payload lanBootstrapResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return lanBootstrapResponse{}, fmt.Errorf("lan_peer %q decode bootstrap payload: %w", configuredValue, err)
	}
	if normalizeNetworkID(expectedNetworkID) != "" && payload.NetworkID != "" && payload.NetworkID != expectedNetworkID {
		return lanBootstrapResponse{}, fmt.Errorf("lan_peer %q reported network_id %s, want %s", configuredValue, payload.NetworkID, expectedNetworkID)
	}
	if requirePeerID && strings.TrimSpace(payload.PeerID) == "" {
		return lanBootstrapResponse{}, fmt.Errorf("lan_peer %q returned no peer_id", configuredValue)
	}
	return payload, nil
}

func (c *lanPeerHealthCache) bootstrapTargets(kind, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	out := make([]string, 0, 3)
	seen := make(map[string]struct{}, 3)
	appendTarget := func(target string) {
		target = normalizeObservedPrimaryHost(target)
		if target == "" || target == value {
			return
		}
		if _, ok := seen[target]; ok {
			return
		}
		seen[target] = struct{}{}
		out = append(out, target)
	}
	entry := c.entry(kind, value)
	appendTarget(entry.ObservedPrimaryHost)
	for _, siblingKind := range siblingLANPeerKinds(kind) {
		appendTarget(c.entry(siblingKind, value).ObservedPrimaryHost)
	}
	out = append(out, value)
	return out
}

func (c *lanPeerHealthCache) propagateObservedPrimaryHost(kind, value, observedHost string) {
	if c == nil || c.Entries == nil {
		return
	}
	observedHost = normalizeObservedPrimaryHost(observedHost)
	if observedHost == "" {
		return
	}
	for _, siblingKind := range siblingLANPeerKinds(kind) {
		key := lanPeerHealthKey(siblingKind, value)
		entry := c.Entries[key]
		entry.ObservedPrimaryHost = observedHost
		entry.ObservedPrimaryFrom = strings.TrimSpace(kind)
		c.Entries[key] = entry
	}
}

func siblingLANPeerKinds(kind string) []string {
	switch strings.TrimSpace(kind) {
	case "lan_peer":
		return []string{"lan_bt_peer"}
	case "lan_bt_peer":
		return []string{"lan_peer"}
	default:
		return nil
	}
}

func normalizeObservedPrimaryHost(value string) string {
	value = strings.Trim(strings.TrimSpace(value), "[]")
	if ip := net.ParseIP(value); ip != nil && !ip.IsLoopback() && !ip.IsUnspecified() {
		return ip.String()
	}
	return ""
}

func lanBootstrapEndpoint(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("empty lan_peer")
	}
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}
	u, err := url.Parse(value)
	if err != nil {
		return "", err
	}
	host := strings.TrimSpace(u.Host)
	if host == "" {
		host = strings.TrimSpace(u.Path)
		u.Path = ""
	}
	if host == "" {
		return "", fmt.Errorf("missing host")
	}
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(strings.Trim(host, "[]"), "51818")
	}
	u.Scheme = "http"
	u.Host = host
	u.Path = "/api/network/bootstrap"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func fetchLANHistoryManifest(ctx context.Context, value, cursor, expectedNetworkID string) (lanHistoryManifestResponse, error) {
	endpoint, err := lanHistoryManifestEndpoint(value, cursor)
	if err != nil {
		return lanHistoryManifestResponse{}, fmt.Errorf("lan_peer %q: %w", value, err)
	}
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return lanHistoryManifestResponse{}, fmt.Errorf("lan_peer %q request: %w", value, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return lanHistoryManifestResponse{}, fmt.Errorf("lan_peer %q query %s: %w", value, endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return lanHistoryManifestResponse{}, fmt.Errorf("lan_peer %q query %s: status %d", value, endpoint, resp.StatusCode)
	}
	var payload lanHistoryManifestResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return lanHistoryManifestResponse{}, fmt.Errorf("lan_peer %q decode history manifest payload: %w", value, err)
	}
	if normalizeNetworkID(expectedNetworkID) != "" && payload.NetworkID != "" && payload.NetworkID != expectedNetworkID {
		return lanHistoryManifestResponse{}, fmt.Errorf("lan_peer %q reported network_id %s, want %s", value, payload.NetworkID, expectedNetworkID)
	}
	return payload, nil
}

func lanHistoryManifestEndpoint(value, cursor string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("empty lan_peer")
	}
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}
	u, err := url.Parse(value)
	if err != nil {
		return "", err
	}
	host := strings.TrimSpace(u.Host)
	if host == "" {
		host = strings.TrimSpace(u.Path)
		u.Path = ""
	}
	if host == "" {
		return "", fmt.Errorf("missing host")
	}
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(strings.Trim(host, "[]"), "51818")
	}
	u.Scheme = "http"
	u.Host = host
	u.Path = "/api/history/list"
	q := u.Query()
	if strings.TrimSpace(cursor) != "" {
		q.Set("cursor", strings.TrimSpace(cursor))
	}
	u.RawQuery = q.Encode()
	u.Fragment = ""
	return u.String(), nil
}
