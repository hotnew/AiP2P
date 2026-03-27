package haonews

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
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
	store, err := OpenStore(filepath.Join(root, ".haonews"))
	if err != nil {
		t.Fatalf("OpenStore error = %v", err)
	}
	cfg := NetworkBootstrapConfig{
		Path:         filepath.Join(root, "config", "hao_news_net.inf"),
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
