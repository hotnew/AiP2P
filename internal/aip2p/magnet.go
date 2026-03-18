package aip2p

import (
	"net/url"
	"strings"
)

func CanonicalMagnet(infoHash, displayName string) string {
	infoHash = strings.ToLower(strings.TrimSpace(infoHash))
	if infoHash == "" {
		return ""
	}
	values := url.Values{}
	values.Set("xt", "urn:btih:"+infoHash)
	displayName = strings.TrimSpace(displayName)
	if displayName != "" {
		values.Set("dn", displayName)
	}
	return "magnet:?" + values.Encode()
}

func CanonicalizeMagnet(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	ref, err := ParseSyncRef(raw)
	if err != nil {
		return raw
	}
	displayName := magnetDisplayName(raw)
	return CanonicalMagnet(ref.InfoHash, displayName)
}

func magnetDisplayName(raw string) string {
	uri, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	if !strings.EqualFold(uri.Scheme, "magnet") {
		return ""
	}
	return strings.TrimSpace(uri.Query().Get("dn"))
}

func canonicalMessageLink(link *MessageLink) *MessageLink {
	if link == nil {
		return nil
	}
	infoHash := strings.ToLower(strings.TrimSpace(link.InfoHash))
	magnet := CanonicalizeMagnet(link.Magnet)
	if infoHash == "" && magnet != "" {
		ref, err := ParseSyncRef(magnet)
		if err == nil {
			infoHash = ref.InfoHash
		}
	}
	if infoHash == "" && magnet == "" {
		return nil
	}
	if magnet == "" && infoHash != "" {
		magnet = CanonicalMagnet(infoHash, "")
	}
	return &MessageLink{
		InfoHash: infoHash,
		Magnet:   magnet,
	}
}
