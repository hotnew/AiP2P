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

func (s *Store) channelConfigDir(teamID string) string {
	return filepath.Join(s.root, NormalizeTeamID(teamID), "channel-configs")
}

func (s *Store) channelConfigPath(teamID, channelID string) string {
	return filepath.Join(s.channelConfigDir(teamID), normalizeChannelID(channelID)+".json")
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
	data, err := os.ReadFile(s.channelConfigPath(teamID, channelID))
	if errors.Is(err, os.ErrNotExist) {
		return ChannelConfig{ChannelID: channelID}, nil
	}
	if err != nil {
		return ChannelConfig{}, err
	}
	var cfg ChannelConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ChannelConfig{}, fmt.Errorf("invalid channel config json for %s/%s: %w", teamID, channelID, err)
	}
	cfg = normalizeChannelConfig(cfg)
	cfg.ChannelID = channelID
	return cfg, nil
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
		dir := s.channelConfigDir(teamID)
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
	entries, err := os.ReadDir(s.channelConfigDir(teamID))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	configs := make([]ChannelConfig, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		channelID := strings.TrimSuffix(entry.Name(), ".json")
		cfg, err := s.loadChannelConfigNoCtx(teamID, channelID)
		if err != nil {
			continue
		}
		configs = append(configs, cfg)
	}
	sort.SliceStable(configs, func(i, j int) bool {
		return configs[i].ChannelID < configs[j].ChannelID
	})
	return configs, nil
}
