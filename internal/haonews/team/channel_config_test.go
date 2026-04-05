package team

import (
	"context"
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
}
