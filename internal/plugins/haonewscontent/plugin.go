package haonewscontent

import (
	"context"
	_ "embed"
	"os"
	"path/filepath"
	"strings"

	"hao.news/internal/apphost"
	newsplugin "hao.news/internal/plugins/haonews"
)

type Plugin struct{}

//go:embed haonews.plugin.json
var pluginManifestJSON []byte

func (Plugin) Manifest() apphost.PluginManifest {
	return apphost.MustLoadPluginManifestJSON(pluginManifestJSON)
}

func (Plugin) Build(ctx context.Context, cfg apphost.Config, theme apphost.WebTheme) (*apphost.Site, error) {
	cfg = newsplugin.ApplyDefaultConfig(cfg)
	options := newsplugin.OptionsForPlugins(newsplugin.ContentOnlyAppOptions(), cfg)
	app, err := newsplugin.NewWithThemeAndOptions(
		cfg.StoreRoot,
		cfg.Project,
		cfg.Version,
		cfg.ArchiveRoot,
		cfg.RulesPath,
		cfg.WriterPolicyPath,
		cfg.NetPath,
		theme,
		options,
	)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(filepath.Base(os.Args[0]), ".test") {
		if index, err := app.Index(); err == nil {
			_ = app.NodeStatus(index)
		}
	}
	stopSync, err := newsplugin.StartManagedSyncIfNeeded(ctx, cfg, options)
	if err != nil {
		return nil, err
	}
	staticFS, err := theme.StaticFS()
	if err != nil {
		return nil, err
	}
	return &apphost.Site{
		Manifest: Plugin{}.Manifest(),
		Theme:    theme.Manifest(),
		Handler:  newHandler(app, staticFS),
		Close: func(context.Context) error {
			stopSync()
			return nil
		},
	}, nil
}
