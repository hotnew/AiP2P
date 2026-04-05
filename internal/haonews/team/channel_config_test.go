package team

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestChannelConfigZeroValueWhenMissing(t *testing.T) {
	t.Parallel()

	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}
	cfg, err := store.LoadChannelConfigCtx(context.Background(), "team-alpha", "main")
	if err != nil {
		t.Fatalf("LoadChannelConfigCtx error = %v", err)
	}
	if cfg.ChannelID != "main" {
		t.Fatalf("ChannelID = %q, want main", cfg.ChannelID)
	}
	if cfg.Plugin != "" || cfg.Theme != "" || len(cfg.Rules) != 0 {
		t.Fatalf("expected zero-value config, got %#v", cfg)
	}
}

func TestChannelConfigSaveLoadAndList(t *testing.T) {
	t.Parallel()

	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}
	err = store.SaveChannelConfigCtx(context.Background(), "team-alpha", ChannelConfig{
		ChannelID:       "research",
		Plugin:          "plan-exchange@1.0",
		Theme:           "minimal",
		AgentOnboarding: "Be concise.",
		Rules:           []string{"Focus on plans"},
		Metadata:        map[string]string{"owner": "ops"},
	})
	if err != nil {
		t.Fatalf("SaveChannelConfigCtx error = %v", err)
	}

	cfg, err := store.LoadChannelConfigCtx(context.Background(), "team-alpha", "research")
	if err != nil {
		t.Fatalf("LoadChannelConfigCtx error = %v", err)
	}
	if cfg.ChannelID != "research" || cfg.PluginID() != "plan-exchange" || cfg.Theme != "minimal" {
		t.Fatalf("unexpected config %#v", cfg)
	}
	if cfg.CreatedAt.IsZero() || cfg.UpdatedAt.IsZero() {
		t.Fatalf("expected created/updated timestamps, got %#v", cfg)
	}

	configs, err := store.ListChannelConfigsCtx(context.Background(), "team-alpha")
	if err != nil {
		t.Fatalf("ListChannelConfigsCtx error = %v", err)
	}
	if len(configs) != 1 || configs[0].ChannelID != "research" {
		t.Fatalf("unexpected configs %#v", configs)
	}
	if _, err := os.Stat(store.channelConfigPath("team-alpha", "research")); err != nil {
		t.Fatalf("expected canonical channel config path to exist, got %v", err)
	}
	if store.isShardedChannel("team-alpha", "research") {
		t.Fatal("channel config directory alone should not mark channel as sharded")
	}
}

func TestChannelConfigLoadsLegacyPathAndListsOnce(t *testing.T) {
	t.Parallel()

	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}
	teamID := "team-alpha"
	channelID := "legacy-room"
	if err := os.MkdirAll(store.channelConfigLegacyDir(teamID), 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	legacyPath := store.channelConfigLegacyPath(teamID, channelID)
	if err := os.WriteFile(legacyPath, []byte(`{
  "channel_id":"legacy-room",
  "plugin":"plan-exchange@1.0",
  "theme":"minimal",
  "agent_onboarding":"Read legacy config first.",
  "rules":["Legacy only"]
}`), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	cfg, err := store.LoadChannelConfigCtx(context.Background(), teamID, channelID)
	if err != nil {
		t.Fatalf("LoadChannelConfigCtx error = %v", err)
	}
	if cfg.PluginID() != "plan-exchange" || cfg.Theme != "minimal" {
		t.Fatalf("unexpected legacy-loaded config %#v", cfg)
	}

	if err := store.SaveChannelConfigCtx(context.Background(), teamID, cfg); err != nil {
		t.Fatalf("SaveChannelConfigCtx error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(store.channelConfigDir(teamID, channelID), "channel_config.json")); err != nil {
		t.Fatalf("expected canonical file after save, got %v", err)
	}

	configs, err := store.ListChannelConfigsCtx(context.Background(), teamID)
	if err != nil {
		t.Fatalf("ListChannelConfigsCtx error = %v", err)
	}
	if len(configs) != 1 || configs[0].ChannelID != channelID {
		t.Fatalf("unexpected configs %#v", configs)
	}
}
