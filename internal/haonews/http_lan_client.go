package haonews

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func newLANHTTPClient(timeout time.Duration, lanPeers []string) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if localIP := preferredLocalBindIP(lanPeers); localIP != nil {
		dialer := &net.Dialer{
			Timeout:   timeout,
			KeepAlive: 30 * time.Second,
			LocalAddr: &net.TCPAddr{IP: localIP},
		}
		transport.DialContext = dialer.DialContext
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

func doLANHTTPRequest(req *http.Request, timeout time.Duration, lanPeers []string) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("request is required")
	}
	if resp, err := doLANHTTPWithCurl(req, timeout); err == nil {
		return resp, nil
	}
	if localIP := preferredLocalBindIP(lanPeers); localIP != nil {
		resp, err := newLANHTTPClient(timeout, lanPeers).Do(req)
		if err == nil {
			return resp, nil
		}
		if !shouldFallbackLANHTTP(err) {
			return nil, err
		}
		clone, cloneErr := cloneHTTPRequest(req)
		if cloneErr != nil {
			return nil, err
		}
		resp, plainErr := plainHTTPClient(timeout).Do(clone)
		if plainErr == nil {
			return resp, nil
		}
		if !shouldFallbackLANHTTP(plainErr) {
			return nil, plainErr
		}
		curlResp, curlErr := doLANHTTPWithCurl(req, timeout)
		if curlErr == nil {
			return curlResp, nil
		}
		return nil, plainErr
	}
	resp, err := plainHTTPClient(timeout).Do(req)
	if err == nil {
		return resp, nil
	}
	if !shouldFallbackLANHTTP(err) {
		return nil, err
	}
	curlResp, curlErr := doLANHTTPWithCurl(req, timeout)
	if curlErr == nil {
		return curlResp, nil
	}
	return nil, err
}

func preferredLocalBindIP(lanPeers []string) net.IP {
	for _, host := range localPeerHosts(lanPeers) {
		ip := net.ParseIP(host)
		if ip == nil {
			continue
		}
		if ip4 := ip.To4(); ip4 != nil {
			return ip4
		}
	}
	return nil
}

func plainHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: http.DefaultTransport.(*http.Transport).Clone(),
	}
}

func cloneHTTPRequest(req *http.Request) (*http.Request, error) {
	clone := req.Clone(req.Context())
	if req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		clone.Body = body
	}
	return clone, nil
}

func shouldFallbackLANHTTP(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	return false
}

func doLANHTTPWithCurl(req *http.Request, timeout time.Duration) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("request is required")
	}
	if req.Method != "" && !strings.EqualFold(req.Method, http.MethodGet) {
		return nil, errors.New("curl fallback only supports GET")
	}
	if req.URL == nil {
		return nil, errors.New("request URL is required")
	}
	host := req.URL.Hostname()
	ip := net.ParseIP(host)
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.To4() == nil || !isRFC1918IPv4(ip.To4()) {
		return nil, errors.New("curl fallback only supports private IPv4 hosts")
	}
	seconds := int(timeout / time.Second)
	if seconds <= 0 {
		seconds = 5
	}
	const marker = "\n__HAONEWS_STATUS__:"
	cmd := exec.CommandContext(req.Context(), "curl",
		"--silent",
		"--show-error",
		"--location",
		"--max-time", strconv.Itoa(seconds),
		"--output", "-",
		"--write-out", marker+"%{http_code}",
		req.URL.String(),
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	idx := bytes.LastIndex(out, []byte(marker))
	if idx < 0 {
		return nil, errors.New("curl fallback missing status marker")
	}
	body := out[:idx]
	codeValue := strings.TrimSpace(string(out[idx+len(marker):]))
	statusCode, err := strconv.Atoi(codeValue)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode:    statusCode,
		Status:        strconv.Itoa(statusCode) + " " + http.StatusText(statusCode),
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Header:        make(http.Header),
		Request:       req,
	}, nil
}
