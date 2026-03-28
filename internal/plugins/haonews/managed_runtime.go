package newsplugin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hao.news/internal/apphost"
)

func StartManagedSyncIfNeeded(parent context.Context, cfg apphost.Config, options AppOptions) (func(), error) {
	if !options.ContentRoutes {
		return func() {}, nil
	}
	if strings.HasSuffix(filepath.Base(os.Args[0]), ".test") {
		return func() {}, nil
	}
	mode, err := ParseSyncMode(cfg.SyncMode)
	if err != nil {
		return nil, err
	}
	if mode != SyncModeManaged {
		return func() {}, nil
	}
	runtimeRoot := strings.TrimSpace(cfg.RuntimeRoot)
	if runtimeRoot == "" {
		paths, err := DefaultRuntimePaths()
		if err != nil {
			return nil, err
		}
		runtimeRoot = paths.Root
	}
	supervisor, err := StartManagedSyncSupervisor(parent, ManagedSyncConfig{
		Runtime:          RuntimePathsFromRoot(runtimeRoot),
		BinaryPath:       cfg.SyncBinaryPath,
		StoreRoot:        cfg.StoreRoot,
		NetPath:          cfg.NetPath,
		RulesPath:        cfg.RulesPath,
		WriterPolicyPath: cfg.WriterPolicyPath,
		InitialDelay:     5 * time.Second,
		StaleAfter:       cfg.SyncStaleAfter,
		Logf:             cfg.Logf,
	})
	if err != nil {
		return nil, err
	}
	return supervisor.Stop, nil
}
