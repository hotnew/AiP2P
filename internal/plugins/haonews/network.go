package newsplugin

import (
	"fmt"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

const (
	networkModeLAN    = "lan"
	networkModePublic = "public"
	networkModeShared = "shared"
)

const networkIDFileName = "network_id.inf"

type NetworkBootstrapConfig struct {
	Path             string
	Exists           bool
	NetworkMode      string
	NetworkID        string
	LibP2PListen     []string
	LANPeers         []string
	PublicPeers      []string
	RelayPeers       []string
	LibP2PBootstrap  []string
	LibP2PRendezvous []string
}

func LoadNetworkBootstrapConfig(path string) (NetworkBootstrapConfig, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return NetworkBootstrapConfig{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NetworkBootstrapConfig{Path: path, NetworkMode: networkModeLAN}, nil
		}
		return NetworkBootstrapConfig{}, err
	}
	cfg := NetworkBootstrapConfig{Path: path}
	cfg.Exists = true
	seenListen := make(map[string]struct{})
	seenLAN := make(map[string]struct{})
	seenPublic := make(map[string]struct{})
	seenRelay := make(map[string]struct{})
	seenLibP2P := make(map[string]struct{})
	seenRendezvous := make(map[string]struct{})
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "//") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		switch key {
		case "network_mode":
			if cfg.NetworkMode == "" {
				cfg.NetworkMode = normalizeNetworkMode(value)
			}
		case "network_id":
			if cfg.NetworkID == "" {
				cfg.NetworkID = normalizeNetworkID(value)
			}
		case "libp2p_listen":
			if _, ok := seenListen[value]; ok {
				continue
			}
			seenListen[value] = struct{}{}
			cfg.LibP2PListen = append(cfg.LibP2PListen, value)
		case "lan_peer":
			if _, ok := seenLAN[value]; ok {
				continue
			}
			seenLAN[value] = struct{}{}
			cfg.LANPeers = append(cfg.LANPeers, value)
		case "public_peer", "public_http_peer", "public_sync_peer":
			if _, ok := seenPublic[value]; ok {
				continue
			}
			seenPublic[value] = struct{}{}
			cfg.PublicPeers = append(cfg.PublicPeers, value)
		case "relay_peer":
			if _, ok := seenRelay[value]; ok {
				continue
			}
			seenRelay[value] = struct{}{}
			cfg.RelayPeers = append(cfg.RelayPeers, value)
		case "libp2p_bootstrap":
			if _, ok := seenLibP2P[value]; ok {
				continue
			}
			seenLibP2P[value] = struct{}{}
			cfg.LibP2PBootstrap = append(cfg.LibP2PBootstrap, value)
		case "libp2p_rendezvous", "rendezvous":
			if _, ok := seenRendezvous[value]; ok {
				continue
			}
			seenRendezvous[value] = struct{}{}
			cfg.LibP2PRendezvous = append(cfg.LibP2PRendezvous, value)
		}
	}
	if cfg.NetworkMode == "" {
		cfg.NetworkMode = networkModeLAN
	}
	fileNetworkID, err := loadNetworkIDFile(networkIDFilePath(path))
	if err != nil {
		return NetworkBootstrapConfig{}, err
	}
	if fileNetworkID != "" {
		cfg.NetworkID = fileNetworkID
	}
	return cfg, nil
}

func (c NetworkBootstrapConfig) AllowsLANDiscovery() bool {
	mode := normalizeNetworkMode(c.NetworkMode)
	return mode == "" || mode == networkModeLAN || mode == networkModeShared
}

func (c NetworkBootstrapConfig) FileName() string {
	if c.Path == "" {
		return ""
	}
	return filepath.Base(c.Path)
}

func normalizeNetworkID(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if len(value) != 64 {
		return ""
	}
	if _, err := hex.DecodeString(value); err != nil {
		return ""
	}
	return value
}

func normalizeNetworkMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case networkModePublic:
		return networkModePublic
	case networkModeShared:
		return networkModeShared
	case networkModeLAN:
		return networkModeLAN
	default:
		return ""
	}
}

func networkIDFilePath(netPath string) string {
	netPath = strings.TrimSpace(netPath)
	if netPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(netPath), networkIDFileName)
}

func loadNetworkIDFile(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "//") {
			continue
		}
		if key, value, ok := strings.Cut(line, "="); ok {
			if strings.EqualFold(strings.TrimSpace(key), "network_id") {
				line = strings.TrimSpace(value)
			}
		}
		networkID := normalizeNetworkID(line)
		if networkID != "" {
			return networkID, nil
		}
		return "", fmt.Errorf("network_id could not be parsed from %s", filepath.Base(path))
	}
	return "", nil
}
