package host

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aip2p/internal/apphost"
	"aip2p/internal/builtin"
	"aip2p/internal/extensions"
	"aip2p/internal/themes/directorytheme"
	"aip2p/internal/workspace"
)

type Config struct {
	App              string
	Plugin           string
	Plugins          []string
	PluginDirs       []string
	PluginConfigs    map[string]map[string]any
	Theme            string
	ThemeDir         string
	AppDir           string
	ExtensionsRoot   string
	Project          string
	Version          string
	ListenAddr       string
	RuntimeRoot      string
	StoreRoot        string
	ArchiveRoot      string
	RulesPath        string
	WriterPolicyPath string
	NetPath          string
	SyncMode         string
	SyncBinaryPath   string
	SyncStaleAfter   time.Duration
	Logf             func(string, ...any)
}

type Instance struct {
	config   Config
	site     *apphost.Site
	server   *http.Server
	listener net.Listener
}

func New(ctx context.Context, cfg Config) (*Instance, error) {
	cfg = normalizeConfig(cfg)
	listener, resolvedListenAddr, err := reserveListenAddr(cfg.ListenAddr)
	if err != nil {
		return nil, err
	}
	cfg.ListenAddr = resolvedListenAddr
	registry := builtin.DefaultRegistry()
	store, err := extensions.Open(cfg.ExtensionsRoot)
	if err != nil {
		return nil, err
	}
	installedApps, err := store.RegisterIntoRegistry(registry, "", "", "")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.AppDir) == "" && strings.TrimSpace(cfg.App) != "" {
		if _, err := builtin.ResolveApp(cfg.App); err != nil {
			if installedApp, ok := installedApps[strings.ToLower(strings.TrimSpace(cfg.App))]; ok {
				cfg.AppDir = installedApp.Root
			}
		}
	}
	appDirExplicit := strings.TrimSpace(cfg.AppDir) != ""
	themeExplicit := strings.TrimSpace(cfg.Theme) != ""
	var bundle workspace.AppBundle
	if appDirExplicit {
		bundle, err = workspace.LoadAppBundle(cfg.AppDir)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(cfg.RuntimeRoot) == "" && strings.TrimSpace(bundle.Config.RuntimeRoot) == "" {
			cfg.RuntimeRoot = filepath.Join(bundle.Root, "runtime")
		}
		cfg, err = applyAppBundleConfig(cfg, bundle.Config)
		if err != nil {
			return nil, err
		}
		if len(cfg.PluginConfigs) == 0 && len(bundle.PluginConfigs) > 0 {
			cfg.PluginConfigs = bundle.PluginConfigs
		}
		if strings.TrimSpace(cfg.App) == "" {
			cfg.App = bundle.App.ID
		}
		if len(cfg.Plugins) == 0 && strings.TrimSpace(cfg.Plugin) == "" {
			cfg.Plugins = append([]string(nil), bundle.App.Plugins...)
		}
		if !themeExplicit && strings.TrimSpace(cfg.ThemeDir) == "" {
			cfg.Theme = bundle.App.Theme
		}
	}
	if strings.TrimSpace(cfg.App) != "" {
		app, err := builtin.ResolveApp(cfg.App)
		if err != nil {
			if !appDirExplicit || !strings.EqualFold(strings.TrimSpace(cfg.App), strings.TrimSpace(bundle.App.ID)) {
				return nil, err
			}
		} else {
			if len(cfg.Plugins) == 0 && strings.TrimSpace(cfg.Plugin) == "" {
				cfg.Plugins = append([]string(nil), app.Plugins...)
			}
			if !themeExplicit && strings.TrimSpace(cfg.ThemeDir) == "" && strings.TrimSpace(cfg.Theme) == "" {
				cfg.Theme = app.Theme
			}
		}
	}
	loadedPluginIDs := make([]string, 0)
	if appDirExplicit {
		plugins, manifests, err := workspace.LoadPlugins(filepath.Join(bundle.Root, "plugins"), registry)
		if err != nil {
			return nil, err
		}
		for idx, plugin := range plugins {
			if err := registry.RegisterPlugin(plugin); err != nil {
				return nil, err
			}
			loadedPluginIDs = append(loadedPluginIDs, manifests[idx].ID)
		}
	}
	for _, dir := range cfg.PluginDirs {
		plugin, manifest, err := workspace.LoadPluginDir(dir, registry)
		if err != nil {
			return nil, err
		}
		if err := registry.RegisterPlugin(plugin); err != nil {
			return nil, err
		}
		loadedPluginIDs = append(loadedPluginIDs, manifest.ID)
	}
	for _, theme := range bundle.Themes {
		if err := registry.RegisterTheme(theme); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(cfg.ThemeDir) != "" {
		theme, err := directorytheme.Load(cfg.ThemeDir)
		if err != nil {
			return nil, err
		}
		if err := registry.RegisterTheme(theme); err != nil {
			return nil, err
		}
		if !themeExplicit {
			cfg.Theme = theme.Manifest().ID
		}
	}
	if len(cfg.Plugins) == 0 && strings.TrimSpace(cfg.Plugin) == "" && strings.TrimSpace(cfg.App) == "" && len(loadedPluginIDs) > 0 {
		cfg.Plugins = append([]string(nil), loadedPluginIDs...)
	}
	site, err := registry.Build(ctx, apphost.Config{
		Plugin:           cfg.Plugin,
		Plugins:          cfg.Plugins,
		PluginConfigs:    cfg.PluginConfigs,
		Theme:            cfg.Theme,
		Project:          cfg.Project,
		Version:          cfg.Version,
		ListenAddr:       cfg.ListenAddr,
		RuntimeRoot:      cfg.RuntimeRoot,
		StoreRoot:        cfg.StoreRoot,
		ArchiveRoot:      cfg.ArchiveRoot,
		RulesPath:        cfg.RulesPath,
		WriterPolicyPath: cfg.WriterPolicyPath,
		NetPath:          cfg.NetPath,
		SyncMode:         cfg.SyncMode,
		SyncBinaryPath:   cfg.SyncBinaryPath,
		SyncStaleAfter:   cfg.SyncStaleAfter,
		Logf:             cfg.Logf,
	})
	if err != nil {
		return nil, err
	}
	return &Instance{
		config: cfg,
		site:   site,
		server: &http.Server{
			Addr:    cfg.ListenAddr,
			Handler: site.Handler,
		},
		listener: listener,
	}, nil
}

func (i *Instance) ListenAndServe(ctx context.Context) error {
	if i == nil || i.server == nil || i.listener == nil {
		return errors.New("host instance is not initialized")
	}
	errCh := make(chan error, 1)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = i.server.Shutdown(shutdownCtx)
		_ = i.site.Shutdown(shutdownCtx)
	}()
	go func() {
		errCh <- i.server.Serve(i.listener)
	}()
	err := <-errCh
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (i *Instance) Site() *apphost.Site {
	if i == nil {
		return nil
	}
	return i.site
}

func (i *Instance) ListenAddr() string {
	if i == nil {
		return ""
	}
	if i.listener != nil {
		return i.listener.Addr().String()
	}
	if i.server == nil {
		return ""
	}
	return i.server.Addr
}

func normalizeConfig(cfg Config) Config {
	if strings.TrimSpace(cfg.AppDir) == "" && strings.TrimSpace(cfg.App) == "" && len(cfg.Plugins) == 0 && strings.TrimSpace(cfg.Plugin) == "" && len(cfg.PluginDirs) == 0 {
		cfg.App = "aip2p-app"
	}
	if strings.TrimSpace(cfg.ListenAddr) == "" {
		cfg.ListenAddr = "0.0.0.0:51818"
	}
	if strings.TrimSpace(cfg.Version) == "" {
		cfg.Version = "dev"
	}
	if cfg.SyncStaleAfter <= 0 {
		cfg.SyncStaleAfter = 2 * time.Minute
	}
	if cfg.Logf == nil {
		cfg.Logf = func(string, ...any) {}
	}
	return cfg
}

func resolveListenAddr(addr string) (string, error) {
	listener, resolved, err := reserveListenAddr(addr)
	if listener != nil {
		_ = listener.Close()
	}
	return resolved, err
}

func reserveListenAddr(addr string) (net.Listener, string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, "", errors.New("listen address is required")
	}
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			return nil, "", err
		}
		return listener, listener.Addr().String(), nil
	}
	port, err := strconv.Atoi(strings.TrimSpace(portText))
	if err != nil {
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			return nil, "", err
		}
		return listener, listener.Addr().String(), nil
	}
	if port <= 0 {
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			return nil, "", err
		}
		return listener, listener.Addr().String(), nil
	}
	for candidate := port; candidate <= 65535; candidate++ {
		next := net.JoinHostPort(host, strconv.Itoa(candidate))
		listener, err := net.Listen("tcp", next)
		if err != nil {
			if isAddrInUse(err) {
				continue
			}
			return nil, "", err
		}
		return listener, listener.Addr().String(), nil
	}
	return nil, "", fmt.Errorf("no available listen port found starting from %s", addr)
}

func isAddrInUse(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "address already in use")
}

func applyAppBundleConfig(cfg Config, appCfg workspace.AppConfig) (Config, error) {
	if strings.TrimSpace(cfg.Project) == "" {
		cfg.Project = appCfg.Project
	}
	if strings.TrimSpace(cfg.Version) == "" || cfg.Version == "dev" {
		if strings.TrimSpace(appCfg.Version) != "" {
			cfg.Version = appCfg.Version
		}
	}
	if strings.TrimSpace(cfg.Theme) == "" {
		cfg.Theme = appCfg.Theme
	}
	if strings.TrimSpace(cfg.RuntimeRoot) == "" {
		cfg.RuntimeRoot = appCfg.RuntimeRoot
	}
	if strings.TrimSpace(cfg.StoreRoot) == "" {
		cfg.StoreRoot = appCfg.StoreRoot
	}
	if strings.TrimSpace(cfg.ArchiveRoot) == "" {
		cfg.ArchiveRoot = appCfg.ArchiveRoot
	}
	if strings.TrimSpace(cfg.RulesPath) == "" {
		cfg.RulesPath = appCfg.RulesPath
	}
	if strings.TrimSpace(cfg.WriterPolicyPath) == "" {
		cfg.WriterPolicyPath = appCfg.WriterPolicyPath
	}
	if strings.TrimSpace(cfg.NetPath) == "" {
		cfg.NetPath = appCfg.NetPath
	}
	if strings.TrimSpace(cfg.SyncMode) == "" {
		cfg.SyncMode = appCfg.SyncMode
	}
	if strings.TrimSpace(cfg.SyncBinaryPath) == "" {
		cfg.SyncBinaryPath = appCfg.SyncBinaryPath
	}
	if cfg.SyncStaleAfter <= 0 {
		duration, err := appCfg.SyncStaleAfterDuration()
		if err != nil {
			return Config{}, err
		}
		if duration > 0 {
			cfg.SyncStaleAfter = duration
		}
	}
	return cfg, nil
}

func (i *Instance) String() string {
	if i == nil || i.site == nil {
		return "aip2p host"
	}
	return fmt.Sprintf("%s on %s", i.site.Manifest.Name, i.config.ListenAddr)
}
