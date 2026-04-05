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
