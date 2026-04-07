package live

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aip2p/internal/aip2p"
)

type lanLiveBootstrapResponse struct {
	NetworkID string   `json:"network_id"`
	PeerID    string   `json:"peer_id"`
	DialAddrs []string `json:"dial_addrs"`
}

func resolveLiveLANBootstrapPeers(ctx context.Context, cfg aip2p.NetworkBootstrapConfig) ([]string, error) {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(cfg.LANPeers))
	var errs []string
	for _, target := range cfg.LANPeers {
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}
		peers, err := fetchLiveLANBootstrapPeer(ctx, target, cfg.NetworkID)
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
	if len(out) > 0 {
		return out, nil
	}
	fallback, err := aip2p.ResolveLANBootstrapPeers(ctx, cfg)
	if err == nil {
		return fallback, nil
	}
	if len(errs) == 0 {
		return nil, err
	}
	errs = append(errs, err.Error())
	return nil, errors.New(strings.Join(errs, "; "))
}

func fetchLiveLANBootstrapPeer(ctx context.Context, target, expectedNetworkID string) ([]string, error) {
	endpoint, err := livePeerAPIEndpoint(target)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("live lan_peer %q request: %w", target, err)
	}
	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("live lan_peer %q query %s: %w", target, endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("live lan_peer %q query %s: status %d", target, endpoint, resp.StatusCode)
	}
	var payload lanLiveBootstrapResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("live lan_peer %q decode payload: %w", target, err)
	}
	if expected := strings.TrimSpace(expectedNetworkID); expected != "" && strings.TrimSpace(payload.NetworkID) != expected {
		return nil, fmt.Errorf("live lan_peer %q reported network_id %s, want %s", target, payload.NetworkID, expected)
	}
	if strings.TrimSpace(payload.PeerID) == "" {
		return nil, fmt.Errorf("live lan_peer %q returned no peer_id", target)
	}
	out := make([]string, 0, len(payload.DialAddrs))
	for _, addr := range payload.DialAddrs {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		if !strings.Contains(addr, "/p2p/") {
			addr += "/p2p/" + strings.TrimSpace(payload.PeerID)
		}
		out = append(out, addr)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("live lan_peer %q returned no dialable addresses", target)
	}
	return out, nil
}

func livePeerAPIEndpoint(target string) (string, error) {
	value := strings.TrimSpace(target)
	if value == "" {
		return "", fmt.Errorf("empty live bootstrap target")
	}
	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err != nil {
			return "", err
		}
		parsed.Path = "/api/live/bootstrap"
		parsed.RawQuery = ""
		return parsed.String(), nil
	}
	scheme := "http"
	host := value
	if !strings.Contains(host, ":") {
		host += ":51818"
	}
	return scheme + "://" + host + "/api/live/bootstrap", nil
}
