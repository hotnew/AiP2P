package builtin

import (
	_ "embed"
	"fmt"
	"strings"

	"aip2p.org/internal/apphost"
	newsdemoarchive "aip2p.org/internal/plugins/newsdemoarchive"
	newsdemocontent "aip2p.org/internal/plugins/newsdemocontent"
	newsdemogovernance "aip2p.org/internal/plugins/newsdemogovernance"
	newsdemoops "aip2p.org/internal/plugins/newsdemoops"
	"aip2p.org/internal/themes/newsdemo"
)

//go:embed news-demo.app.json
var newsDemoAppJSON []byte

func DefaultRegistry() *apphost.Registry {
	registry := apphost.NewRegistry()
	registry.MustRegisterTheme(newsdemo.Theme{})
	registry.MustRegisterPlugin(newsdemocontent.Plugin{})
	registry.MustRegisterPlugin(newsdemoarchive.Plugin{})
	registry.MustRegisterPlugin(newsdemogovernance.Plugin{})
	registry.MustRegisterPlugin(newsdemoops.Plugin{})
	return registry
}

func DefaultApps() []apphost.AppManifest {
	return []apphost.AppManifest{
		apphost.MustLoadAppManifestJSON(newsDemoAppJSON),
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
