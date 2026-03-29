package haonewslive

import (
	"context"
	_ "embed"
	"io/fs"
	"net/http"
	"strings"
	"sync"

	"hao.news/internal/apphost"
	"hao.news/internal/haonews/live"
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
	options := newsplugin.OptionsForPlugins(newsplugin.AppOptions{}, cfg)
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
	store, err := live.OpenLocalStore(cfg.StoreRoot)
	if err != nil {
		return nil, err
	}
	logf := cfg.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}
	watchCtx, cancelWatch := context.WithCancel(ctx)
	var watcherMu sync.Mutex
	var watcher *live.AnnouncementWatcher
	go func() {
		startedWatcher, startErr := live.StartAnnouncementWatcher(watchCtx, cfg.StoreRoot, cfg.NetPath)
		if startErr != nil {
			logf("haonews live: announcement watcher disabled: %v", startErr)
			return
		}
		watcherMu.Lock()
		watcher = startedWatcher
		watcherMu.Unlock()
	}()
	staticFS, err := theme.StaticFS()
	if err != nil {
		cancelWatch()
		return nil, err
	}
	return &apphost.Site{
		Manifest: Plugin{}.Manifest(),
		Theme:    theme.Manifest(),
		Handler:  newHandler(app, store, staticFS),
		Close: func(context.Context) error {
			cancelWatch()
			watcherMu.Lock()
			startedWatcher := watcher
			watcher = nil
			watcherMu.Unlock()
			if startedWatcher != nil {
				return startedWatcher.Close()
			}
			return nil
		},
	}, nil
}

func newHandler(app *newsplugin.App, store *live.LocalStore, staticFS fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/live" {
			http.NotFound(w, r)
			return
		}
		handleLiveIndex(app, store, w, r)
	})
	mux.HandleFunc("/live/", func(w http.ResponseWriter, r *http.Request) {
		roomID := strings.TrimSpace(newsplugin.PathValue("/live/", r.URL.Path))
		if roomID == "" {
			http.NotFound(w, r)
			return
		}
		handleLiveRoom(app, store, roomID, w, r)
	})
	mux.HandleFunc("/api/live/rooms", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/live/rooms" {
			http.NotFound(w, r)
			return
		}
		handleAPILiveRooms(store, w, r)
	})
	mux.HandleFunc("/api/live/rooms/", func(w http.ResponseWriter, r *http.Request) {
		roomID := strings.TrimSpace(newsplugin.PathValue("/api/live/rooms/", r.URL.Path))
		if roomID == "" {
			http.NotFound(w, r)
			return
		}
		handleAPILiveRoom(store, roomID, w, r)
	})
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	return mux
}
