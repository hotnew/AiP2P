package aip2p

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type lanBootstrapResponse struct {
	NetworkID       string   `json:"network_id"`
	PeerID          string   `json:"peer_id"`
	DialAddrs       []string `json:"dial_addrs"`
	BitTorrentNodes []string `json:"bittorrent_nodes"`
}

type lanHistoryManifestResponse struct {
	Project          string             `json:"project"`
	Version          string             `json:"version"`
	NetworkID        string             `json:"network_id"`
	ManifestInfoHash string             `json:"manifest_infohash"`
	GeneratedAt      string             `json:"generated_at"`
	EntryCount       int                `json:"entry_count"`
	Entries          []SyncAnnouncement `json:"entries"`
}

func resolveLANBootstrapPeers(ctx context.Context, cfg NetworkBootstrapConfig) ([]string, error) {
	out := make([]string, 0, len(cfg.LANPeers))
	var errs []string
	seen := make(map[string]struct{})
	for _, value := range cfg.LANPeers {
		peers, err := fetchLANBootstrapPeer(ctx, value, cfg.NetworkID)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		for _, peerValue := range peers {
			if _, ok := seen[peerValue]; ok {
				continue
			}
			seen[peerValue] = struct{}{}
			out = append(out, peerValue)
		}
	}
	if len(errs) > 0 {
		return out, errors.New(strings.Join(errs, "; "))
	}
	return out, nil
}

func fetchLANBootstrapPeer(ctx context.Context, value, expectedNetworkID string) ([]string, error) {
	endpoint, err := lanBootstrapEndpoint(value)
	if err != nil {
		return nil, fmt.Errorf("lan_peer %q: %w", value, err)
	}
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("lan_peer %q request: %w", value, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lan_peer %q query %s: %w", value, endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lan_peer %q query %s: status %d", value, endpoint, resp.StatusCode)
	}
	var payload lanBootstrapResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("lan_peer %q decode bootstrap payload: %w", value, err)
	}
	if normalizeNetworkID(expectedNetworkID) != "" && payload.NetworkID != "" && payload.NetworkID != expectedNetworkID {
		return nil, fmt.Errorf("lan_peer %q reported network_id %s, want %s", value, payload.NetworkID, expectedNetworkID)
	}
	if strings.TrimSpace(payload.PeerID) == "" {
		return nil, fmt.Errorf("lan_peer %q returned no peer_id", value)
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
		return nil, fmt.Errorf("lan_peer %q returned no dialable addresses", value)
	}
	return out, nil
}

func resolveLANTorrentRouters(ctx context.Context, cfg NetworkBootstrapConfig) ([]string, error) {
	out := make([]string, 0, len(cfg.LANTorrentPeers))
	var errs []string
	seen := make(map[string]struct{})
	for _, value := range cfg.LANTorrentPeers {
		nodes, err := fetchLANTorrentRouters(ctx, value, cfg.NetworkID)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		for _, node := range nodes {
			if _, ok := seen[node]; ok {
				continue
			}
			seen[node] = struct{}{}
			out = append(out, node)
		}
	}
	if len(errs) > 0 {
		return out, errors.New(strings.Join(errs, "; "))
	}
	return out, nil
}

func fetchLANTorrentRouters(ctx context.Context, value, expectedNetworkID string) ([]string, error) {
	endpoint, err := lanBootstrapEndpoint(value)
	if err != nil {
		return nil, fmt.Errorf("lan_bt_peer %q: %w", value, err)
	}
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("lan_bt_peer %q request: %w", value, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lan_bt_peer %q query %s: %w", value, endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lan_bt_peer %q query %s: status %d", value, endpoint, resp.StatusCode)
	}
	var payload lanBootstrapResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("lan_bt_peer %q decode bootstrap payload: %w", value, err)
	}
	if normalizeNetworkID(expectedNetworkID) != "" && payload.NetworkID != "" && payload.NetworkID != expectedNetworkID {
		return nil, fmt.Errorf("lan_bt_peer %q reported network_id %s, want %s", value, payload.NetworkID, expectedNetworkID)
	}
	out := make([]string, 0, len(payload.BitTorrentNodes))
	for _, node := range payload.BitTorrentNodes {
		if node = strings.TrimSpace(node); node != "" {
			out = append(out, node)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("lan_bt_peer %q returned no bittorrent_nodes", value)
	}
	return out, nil
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

func fetchLANHistoryManifest(ctx context.Context, value, expectedNetworkID string) (lanHistoryManifestResponse, error) {
	endpoint, err := lanHistoryManifestEndpoint(value)
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

func lanHistoryManifestEndpoint(value string) (string, error) {
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
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}
