package haonewsteam

import (
	"context"
	_ "embed"
	"io/fs"
	"net/http"
	"strings"

	"hao.news/internal/apphost"
	teamcore "hao.news/internal/haonews/team"
	newsplugin "hao.news/internal/plugins/haonews"
)

type Plugin struct{}

//go:embed haonews.plugin.json
var pluginManifestJSON []byte

func (Plugin) Manifest() apphost.PluginManifest {
	return apphost.MustLoadPluginManifestJSON(pluginManifestJSON)
}

func (Plugin) Build(_ context.Context, cfg apphost.Config, theme apphost.WebTheme) (*apphost.Site, error) {
	cfg = newsplugin.ApplyDefaultConfig(cfg)
	app, err := newsplugin.NewWithThemeAndOptions(
		cfg.StoreRoot,
		cfg.Project,
		cfg.Version,
		cfg.ArchiveRoot,
		cfg.RulesPath,
		cfg.WriterPolicyPath,
		cfg.NetPath,
		theme,
		newsplugin.OptionsForPlugins(newsplugin.TeamOnlyAppOptions(), cfg),
	)
	if err != nil {
		return nil, err
	}
	store, err := teamcore.OpenStore(cfg.StoreRoot)
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
		Handler:  newHandler(app, store, staticFS),
	}, nil
}

func newHandler(app *newsplugin.App, store *teamcore.Store, staticFS fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/teams", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/teams" {
			http.NotFound(w, r)
			return
		}
		handleTeamIndex(app, store, w, r)
	})
	mux.HandleFunc("/teams/", func(w http.ResponseWriter, r *http.Request) {
		teamID := teamcore.NormalizeTeamID(strings.TrimSpace(newsplugin.PathValue("/teams/", r.URL.Path)))
		if teamID == "" {
			http.NotFound(w, r)
			return
		}
		handleTeam(app, store, teamID, w, r)
	})
	mux.HandleFunc("/api/teams", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/teams" {
			http.NotFound(w, r)
			return
		}
		handleAPITeamIndex(store, w, r)
	})
	mux.HandleFunc("/api/teams/", func(w http.ResponseWriter, r *http.Request) {
		trimmed := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/teams/"), "/")
		if trimmed == "" {
			http.NotFound(w, r)
			return
		}
		parts := strings.Split(trimmed, "/")
		if len(parts) > 2 {
			http.NotFound(w, r)
			return
		}
		teamID := teamcore.NormalizeTeamID(parts[0])
		if teamID == "" {
			http.NotFound(w, r)
			return
		}
		if len(parts) == 1 {
			handleAPITeam(store, teamID, w, r)
			return
		}
		if parts[1] == "members" {
			handleAPITeamMembers(store, teamID, w, r)
			return
		}
		if parts[1] == "messages" {
			handleAPITeamMessages(store, teamID, w, r)
			return
		}
		if parts[1] == "tasks" {
			handleAPITeamTasks(store, teamID, w, r)
			return
		}
		http.NotFound(w, r)
	})
	mux.Handle("/static/", newsplugin.NoStoreStaticHandler(staticFS))
	return mux
}
