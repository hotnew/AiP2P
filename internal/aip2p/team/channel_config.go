package team

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ChannelConfig struct {
	ChannelID       string            `json:"channel_id"`
	Plugin          string            `json:"plugin,omitempty"`
	Theme           string            `json:"theme,omitempty"`
	ThemeConfig     map[string]any    `json:"theme_config,omitempty"`
	AgentOnboarding string            `json:"agent_onboarding,omitempty"`
	Rules           []string          `json:"rules,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	CreatedAt       time.Time         `json:"created_at,omitempty"`
	UpdatedAt       time.Time         `json:"updated_at,omitempty"`
}

func (c ChannelConfig) PluginID() string {
	for i, ch := range c.Plugin {
		if ch == '@' {
			return c.Plugin[:i]
		}
	}
	return c.Plugin
}

func normalizeChannelConfig(cfg ChannelConfig) ChannelConfig {
	cfg.ChannelID = normalizeChannelID(cfg.ChannelID)
	cfg.Plugin = strings.TrimSpace(cfg.Plugin)
	cfg.Theme = strings.TrimSpace(cfg.Theme)
	cfg.AgentOnboarding = strings.TrimSpace(cfg.AgentOnboarding)
	cfg.Rules = normalizeStringList(cfg.Rules)
	if len(cfg.ThemeConfig) == 0 {
		cfg.ThemeConfig = nil
	}
	if len(cfg.Metadata) == 0 {
		cfg.Metadata = nil
	}
	return cfg
}

func (s *Store) channelConfigLegacyDir(teamID string) string {
	return filepath.Join(s.root, NormalizeTeamID(teamID), "channel-configs")
}

func (s *Store) channelConfigLegacyPath(teamID, channelID string) string {
	return filepath.Join(s.channelConfigLegacyDir(teamID), normalizeChannelID(channelID)+".json")
}

func (s *Store) channelConfigDir(teamID, channelID string) string {
	return filepath.Join(s.root, NormalizeTeamID(teamID), "channels", normalizeChannelID(channelID))
}

func (s *Store) channelConfigPath(teamID, channelID string) string {
	return filepath.Join(s.channelConfigDir(teamID, channelID), "channel_config.json")
}

func (s *Store) loadChannelConfigNoCtx(teamID, channelID string) (ChannelConfig, error) {
	if s == nil {
		return ChannelConfig{}, errors.New("nil team store")
	}
	teamID = NormalizeTeamID(teamID)
	channelID = normalizeChannelID(channelID)
	if teamID == "" || channelID == "" {
		return ChannelConfig{}, fmt.Errorf("empty team_id or channel_id")
	}
	paths := []string{
		s.channelConfigPath(teamID, channelID),
		s.channelConfigLegacyPath(teamID, channelID),
	}
	var readErr error
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			readErr = err
			break
		}
		var cfg ChannelConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			return ChannelConfig{}, fmt.Errorf("invalid channel config json for %s/%s: %w", teamID, channelID, err)
		}
		cfg = normalizeChannelConfig(cfg)
		cfg.ChannelID = channelID
		return cfg, nil
	}
	if readErr != nil {
		return ChannelConfig{}, readErr
	}
	return ChannelConfig{ChannelID: channelID}, nil
}

func (s *Store) saveChannelConfigNoCtx(teamID string, cfg ChannelConfig) error {
	if s == nil {
		return errors.New("nil team store")
	}
	teamID = NormalizeTeamID(teamID)
	cfg = normalizeChannelConfig(cfg)
	if teamID == "" || cfg.ChannelID == "" {
		return fmt.Errorf("empty team_id or channel_id")
	}
	return s.withTeamLock(teamID, func() error {
		now := time.Now().UTC()
		if cfg.CreatedAt.IsZero() {
			existing, _ := s.loadChannelConfigNoCtx(teamID, cfg.ChannelID)
			if !existing.CreatedAt.IsZero() {
				cfg.CreatedAt = existing.CreatedAt
			} else {
				cfg.CreatedAt = now
			}
		}
		cfg.UpdatedAt = now
		dir := s.channelConfigDir(teamID, cfg.ChannelID)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return err
		}
		data = append(data, '\n')
		return os.WriteFile(s.channelConfigPath(teamID, cfg.ChannelID), data, 0o644)
	})
}

func (s *Store) listChannelConfigsNoCtx(teamID string) ([]ChannelConfig, error) {
	if s == nil {
		return nil, errors.New("nil team store")
	}
	teamID = NormalizeTeamID(teamID)
	if teamID == "" {
		return nil, errors.New("empty team id")
	}
	seen := map[string]struct{}{}
	configs := make([]ChannelConfig, 0)

	channelEntries, err := os.ReadDir(filepath.Join(s.root, teamID, "channels"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for _, entry := range channelEntries {
		if !entry.IsDir() {
			continue
		}
		channelID := normalizeChannelID(entry.Name())
		if channelID == "" {
			continue
		}
		if _, ok := seen[channelID]; ok {
			continue
		}
		if _, err := os.Stat(s.channelConfigPath(teamID, channelID)); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		cfg, err := s.loadChannelConfigNoCtx(teamID, channelID)
		if err != nil {
			continue
		}
		seen[channelID] = struct{}{}
		configs = append(configs, cfg)
	}

	legacyEntries, err := os.ReadDir(s.channelConfigLegacyDir(teamID))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for _, entry := range legacyEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		channelID := strings.TrimSuffix(entry.Name(), ".json")
		channelID = normalizeChannelID(channelID)
		if channelID == "" {
			continue
		}
		if _, ok := seen[channelID]; ok {
			continue
		}
		cfg, err := s.loadChannelConfigNoCtx(teamID, channelID)
		if err != nil {
			continue
		}
		seen[channelID] = struct{}{}
		configs = append(configs, cfg)
	}
	sort.SliceStable(configs, func(i, j int) bool {
		return configs[i].ChannelID < configs[j].ChannelID
	})
	return configs, nil
}
