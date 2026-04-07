package builtin

import (
	_ "embed"
	"fmt"
	"strings"

	"aip2p/internal/apphost"
	aip2parchive "aip2p/internal/plugins/aip2parchive"
	aip2pcontent "aip2p/internal/plugins/aip2pcontent"
	aip2pgovernance "aip2p/internal/plugins/aip2pgovernance"
	aip2plive "aip2p/internal/plugins/aip2plive"
	aip2pops "aip2p/internal/plugins/aip2pops"
	aip2pteam "aip2p/internal/plugins/aip2pteam"
	"aip2p/internal/themes/aip2p"
)

//go:embed aip2p-app.app.json
var publicAppJSON []byte

func DefaultRegistry() *apphost.Registry {
	registry := apphost.NewRegistry()
	registry.MustRegisterTheme(aip2p.Theme{})
	registry.MustRegisterPlugin(aip2pcontent.Plugin{})
	registry.MustRegisterPlugin(aip2plive.Plugin{})
	registry.MustRegisterPlugin(aip2pteam.Plugin{})
	registry.MustRegisterPlugin(aip2parchive.Plugin{})
	registry.MustRegisterPlugin(aip2pgovernance.Plugin{})
	registry.MustRegisterPlugin(aip2pops.Plugin{})
	return registry
}

func DefaultApps() []apphost.AppManifest {
	return []apphost.AppManifest{
		apphost.MustLoadAppManifestJSON(publicAppJSON),
	}
}

func ResolveApp(id string) (apphost.AppManifest, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, app := range DefaultApps() {
		if strings.ToLower(strings.TrimSpace(app.ID)) == id {
			return app, nil
		}
	}
	return apphost.AppManifest{}, fmt.Errorf("app %q not found", id)
}
