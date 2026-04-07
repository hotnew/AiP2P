package handoffroom

import (
	_ "embed"
	"net/http"

	teamcore "aip2p/internal/aip2p/team"
	"aip2p/internal/plugins/aip2pteam/roomplugin"
)

//go:embed roomplugin.json
var manifestJSON []byte

type Plugin struct{}

func New() *Plugin {
	return &Plugin{}
}

func (p *Plugin) ID() string {
	return "handoff-room"
}

func (p *Plugin) Manifest() roomplugin.Manifest {
	m, err := roomplugin.LoadManifestJSON(manifestJSON)
	if err != nil {
		return roomplugin.Manifest{
			ID:             "handoff-room",
			Name:           "Handoff Room",
			Version:        "1.0.0",
			MinTeamVersion: "0.2.0",
			Routes: roomplugin.RouteSet{
				Web: "/teams/{teamID}/r/handoff-room",
				API: "/api/teams/{teamID}/r/handoff-room",
			},
		}
	}
	return m
}

func (p *Plugin) Handler(store *teamcore.Store, teamID string) http.Handler {
	return newHandler(store, teamID)
}
