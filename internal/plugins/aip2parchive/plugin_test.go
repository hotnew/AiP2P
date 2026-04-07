package aip2parchive

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aip2p/internal/apphost"
	"aip2p/internal/themes/aip2p"
)

func TestPluginBuildServesArchiveIndex(t *testing.T) {
	t.Parallel()

	site := buildArchiveSite(t)
	req := httptest.NewRequest(http.MethodGet, "/archive", nil)
	rec := httptest.NewRecorder()
	site.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Archive") {
		t.Fatalf("expected archive page, got %q", rec.Body.String())
	}
}

func TestPluginBuildServesTopicsArchiveIndexAlias(t *testing.T) {
	t.Parallel()

	site := buildArchiveSite(t)
	req := httptest.NewRequest(http.MethodGet, "/archive/topics", nil)
	rec := httptest.NewRecorder()
	site.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Topics Archive") {
		t.Fatalf("expected topics archive page, got %q", rec.Body.String())
	}
}

func TestPluginBuildHistoryListNotFoundOnEmptyStore(t *testing.T) {
	t.Parallel()

	site := buildArchiveSite(t)
	req := httptest.NewRequest(http.MethodGet, "/api/history/list", nil)
	rec := httptest.NewRecorder()
	site.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestPluginBuildTopicsArchiveAPIAliasesNotFoundOnEmptyStore(t *testing.T) {
	t.Parallel()

	site := buildArchiveSite(t)
	for _, path := range []string{"/api/archive/topics/list", "/api/archive/topics/manifest"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		site.Handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, body = %s", path, rec.Code, rec.Body.String())
		}
	}
}

func buildArchiveSite(t *testing.T) *apphost.Site {
	t.Helper()

	root := t.TempDir()
	cfg := apphost.Config{
		RuntimeRoot:      filepath.Join(root, "runtime"),
		StoreRoot:        filepath.Join(root, "store"),
		ArchiveRoot:      filepath.Join(root, "archive"),
		RulesPath:        filepath.Join(root, "config", "subscriptions.json"),
		WriterPolicyPath: filepath.Join(root, "config", "writer_policy.json"),
		NetPath:          filepath.Join(root, "config", "aip2p_net.inf"),
		Project:          "aip2p",
		Version:          "test",
	}
	site, err := Plugin{}.Build(context.Background(), cfg, aip2p.Theme{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	return site
}
