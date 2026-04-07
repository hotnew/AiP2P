package newsplugin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWrapLocalizedHandlerDefaultsToEnglish(t *testing.T) {
	t.Parallel()

	handler := WrapLocalizedHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html lang="zh-CN"><body><h1>首页</h1><p>返回信息流</p></body></html>`))
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `lang="en"`) {
		t.Fatalf("expected english html lang, got %q", body)
	}
	if !strings.Contains(body, ">Home<") {
		t.Fatalf("expected translated home label, got %q", body)
	}
	if !strings.Contains(body, "theme-locale-switcher") {
		t.Fatalf("expected locale switcher to be injected, got %q", body)
	}
}

func TestWrapLocalizedHandlerSupportsChineseToggle(t *testing.T) {
	t.Parallel()

	handler := WrapLocalizedHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html lang="zh-CN"><body><h1>首页</h1></body></html>`))
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.com/?lang=zh-CN", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `lang="zh-CN"`) {
		t.Fatalf("expected chinese html lang, got %q", body)
	}
	if !strings.Contains(body, ">首页<") {
		t.Fatalf("expected chinese content to stay untouched, got %q", body)
	}
	if got := rec.Result().Cookies(); len(got) == 0 || got[0].Name != localeCookieName {
		t.Fatalf("expected locale cookie to be written, got %#v", got)
	}
}
