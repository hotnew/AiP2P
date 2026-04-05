package roomplugin

import (
	"net/http"
	"testing"

	teamcore "hao.news/internal/haonews/team"
)

type stubPlugin struct {
	id       string
	manifest Manifest
}

func (p stubPlugin) ID() string { return p.id }

func (p stubPlugin) Manifest() Manifest { return p.manifest }

func (p stubPlugin) Handler(store *teamcore.Store, teamID string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}

func TestRegistryRegisterGetAndDuplicate(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	plugin := stubPlugin{
		id: "plan-exchange",
		manifest: Manifest{
			ID:      "plan-exchange",
			Name:    "Plan Exchange",
			Version: "1.0.0",
		},
	}
	if err := registry.Register(plugin); err != nil {
		t.Fatalf("Register error = %v", err)
	}
	if _, ok := registry.Get("plan-exchange"); !ok {
		t.Fatal("expected plugin to be registered")
	}
	if err := registry.Register(plugin); err == nil {
		t.Fatal("expected duplicate plugin registration to fail")
	}
	ids := registry.IDs()
	if len(ids) != 1 || ids[0] != "plan-exchange" {
		t.Fatalf("IDs = %#v", ids)
	}
	manifests := registry.Manifests()
	if len(manifests) != 1 || manifests[0].ID != "plan-exchange" {
		t.Fatalf("Manifests = %#v", manifests)
	}
}

func TestLoadManifestJSONIncludesRoutesAndMinTeamVersion(t *testing.T) {
	t.Parallel()

	manifest, err := LoadManifestJSON([]byte(`{
  "id":"plan-exchange",
  "name":"Plan Exchange",
  "version":"1.0.0",
  "minTeamVersion":"0.2.0",
  "routes":{"web":"/teams/{teamID}/r/plan-exchange","api":"/api/teams/{teamID}/r/plan-exchange"}
}`))
	if err != nil {
		t.Fatalf("LoadManifestJSON error = %v", err)
	}
	if manifest.MinTeamVersion != "0.2.0" {
		t.Fatalf("MinTeamVersion = %q", manifest.MinTeamVersion)
	}
	if manifest.Routes.Web == "" || manifest.Routes.API == "" {
		t.Fatalf("Routes = %#v", manifest.Routes)
	}
}
