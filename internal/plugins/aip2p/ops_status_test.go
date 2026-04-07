package newsplugin

import (
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"

	coreaip2p "aip2p/internal/aip2p"
)

func TestRedisNodeStatusDisabled(t *testing.T) {
	entry, card := redisNodeStatus(NetworkBootstrapConfig{})
	if entry.Value != "disabled" || card.Value != "disabled" {
		t.Fatalf("expected disabled redis status, got entry=%q card=%q", entry.Value, card.Value)
	}
	if entry.Tone != "warn" || card.Tone != "warn" {
		t.Fatalf("expected warn tone, got entry=%q card=%q", entry.Tone, card.Tone)
	}
}

func TestRedisNodeStatusOnline(t *testing.T) {
	mini, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run error = %v", err)
	}
	defer mini.Close()

	mini.Set("aip2p-test-sync:ann:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", `{"infohash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`)
	mini.ZAdd("aip2p-test-sync:channel:news", 1711933200, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	mini.ZAdd("aip2p-test-sync:topic:world", 1711933200, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	mini.RPush("aip2p-test-sync:queue:refs:realtime", "aip2p-sync://bundle/aaa?dn=one")
	mini.RPush("aip2p-test-sync:queue:refs:history", "aip2p-sync://bundle/bbb?dn=two")

	entry, card := redisNodeStatus(NetworkBootstrapConfig{
		Redis: coreaip2p.RedisConfig{
			Enabled:   true,
			Addr:      mini.Addr(),
			KeyPrefix: "aip2p-test-",
		},
	})
	if entry.Value != "online" || card.Value != "online" {
		t.Fatalf("expected online redis status, got entry=%q card=%q", entry.Value, card.Value)
	}
	if entry.Tone != "good" || card.Tone != "good" {
		t.Fatalf("expected good tone, got entry=%q card=%q", entry.Tone, card.Tone)
	}
	if !strings.Contains(entry.Detail, "aip2p-test-") {
		t.Fatalf("expected prefix in detail, got %q", entry.Detail)
	}
	for _, want := range []string{"ann=1", "channel=1", "topic=1", "realtime=1/history=1"} {
		if !strings.Contains(entry.Detail, want) {
			t.Fatalf("expected redis summary detail to contain %q, got %q", want, entry.Detail)
		}
	}
}
