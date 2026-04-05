package planexchange

import (
	_ "embed"
	"net/http"

	teamcore "hao.news/internal/haonews/team"
	"hao.news/internal/plugins/haonewsteam/roomplugin"
)

//go:embed roomplugin.json
var manifestJSON []byte

type Plugin struct{}

func New() *Plugin {
	return &Plugin{}
}

func (p *Plugin) ID() string {
	return "plan-exchange"
}

func (p *Plugin) Manifest() roomplugin.Manifest {
	m, err := roomplugin.LoadManifestJSON(manifestJSON)
	if err != nil {
		return roomplugin.Manifest{
			ID:             "plan-exchange",
			Name:           "Plan Exchange",
			Version:        "1.0.0",
			MinTeamVersion: "0.2.0",
			Routes: roomplugin.RouteSet{
				Web: "/teams/{teamID}/r/plan-exchange",
				API: "/api/teams/{teamID}/r/plan-exchange",
			},
		}
	}
	return m
}

func (p *Plugin) Handler(store *teamcore.Store, teamID string) http.Handler {
	return newHandler(store, teamID)
}
