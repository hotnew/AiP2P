package aip2plive

import (
	"io"
	"net/http"
	"net/http/httptest"
)

func newLocaleRequest(method, target, locale string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if locale == "" || req.URL == nil {
		return req
	}
	query := req.URL.Query()
	query.Set("lang", locale)
	req.URL.RawQuery = query.Encode()
	req.RequestURI = req.URL.RequestURI()
	return req
}

func newChineseRequest(method, target string, body io.Reader) *http.Request {
	return newLocaleRequest(method, target, "zh-CN", body)
}
