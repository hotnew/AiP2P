package aip2p

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	ma "github.com/multiformats/go-multiaddr"
)

func TestStartLibP2PRuntimeSharedModeConnectsResolvedRelayBootstrap(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	relayHost, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
		libp2p.EnableRelayService(),
		libp2p.ForceReachabilityPublic(),
	)
	if err != nil {
		t.Fatalf("libp2p.New(relay) error = %v", err)
	}
	defer relayHost.Close()

	relayBootstrap := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := lanBootstrapResponse{
			NetworkID: latestOrgNetworkID,
			PeerID:    relayHost.ID().String(),
			DialAddrs: relayDialAddrs(relayHost),
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer relayBootstrap.Close()

	root := t.TempDir()
	store, err := OpenStore(filepath.Join(root, ".aip2p"))
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}
	cfg := NetworkBootstrapConfig{
		Path:         filepath.Join(root, "config", "aip2p_net.inf"),
		NetworkID:    latestOrgNetworkID,
		NetworkMode:  networkModeShared,
		RelayPeers:   []string{relayBootstrap.URL},
		LibP2PListen: []string{"/ip4/127.0.0.1/tcp/0"},
	}

	rt, err := startLibP2PRuntime(ctx, cfg, store)
	if err != nil {
		t.Fatalf("startLibP2PRuntime error = %v", err)
	}
	defer rt.Close()

	deadline := time.Now().Add(12 * time.Second)
	var status SyncLibP2PStatus
	for time.Now().Before(deadline) {
		status = rt.Status(ctx)
		if status.AutoRelayEnabled &&
			status.HolePunchingEnabled &&
			status.ResolvedRelayPeers == 1 &&
			status.Reachability == "Private" &&
			len(status.Peers) == 1 &&
			status.Peers[0].PeerID == relayHost.ID().String() &&
			status.Peers[0].Connected &&
			status.Peers[0].Reachable {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("shared mode relay bootstrap not connected: %#v", status)
}

func relayDialAddrs(h host.Host) []string {
	out := make([]string, 0, len(h.Addrs()))
	for _, addr := range h.Addrs() {
		out = append(out, addr.String()+"/p2p/"+h.ID().String())
	}
	return out
}

func TestRewriteAdvertiseAddrsPreservesRelayCircuitAddr(t *testing.T) {
	t.Parallel()

	direct, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/50584")
	if err != nil {
		t.Fatalf("direct addr: %v", err)
	}
	relay, err := ma.NewMultiaddr("/dns/ai.jie.news/tcp/50584/p2p/12D3KooWKqit8ESTPbk9mrutVWpJwNPshMfN7tnQtrQVwLzz1L1r/p2p-circuit")
	if err != nil {
		t.Fatalf("relay addr: %v", err)
	}
	got := rewriteAdvertiseAddrs([]ma.Multiaddr{direct, relay}, "192.168.102.75")
	if len(got) != 2 {
		t.Fatalf("rewritten addrs len = %d, want 2", len(got))
	}
	if got[0].String() != "/ip4/192.168.102.75/tcp/50584" {
		t.Fatalf("direct addr = %q", got[0].String())
	}
	if got[1].String() != relay.String() {
		t.Fatalf("relay addr rewritten unexpectedly: %q", got[1].String())
	}
}

func TestDHTModeForConfig(t *testing.T) {
	t.Parallel()

	if got := DHTModeForConfig(NetworkBootstrapConfig{NetworkMode: networkModePublic}); got != kaddht.ModeAutoServer {
		t.Fatalf("public mode dht = %v, want %v", got, kaddht.ModeAutoServer)
	}
	if got := DHTModeForConfig(NetworkBootstrapConfig{NetworkMode: networkModeShared}); got != kaddht.ModeClient {
		t.Fatalf("shared mode dht = %v, want %v", got, kaddht.ModeClient)
	}
	if got := DHTModeForConfig(NetworkBootstrapConfig{NetworkMode: networkModeLAN}); got != kaddht.ModeClient {
		t.Fatalf("lan mode dht = %v, want %v", got, kaddht.ModeClient)
	}
}
