package live

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"aip2p/internal/aip2p"
)

func TestResolveLiveLANBootstrapPeersUsesLiveBootstrapEndpoint(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/live/bootstrap" {
			t.Fatalf("path = %q, want /api/live/bootstrap", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"network_id":"net-1","peer_id":"12D3KooWTestPeer","dial_addrs":["/ip4/127.0.0.1/tcp/51584"]}`))
	}))
	defer srv.Close()

	peers, err := resolveLiveLANBootstrapPeers(context.Background(), aip2p.NetworkBootstrapConfig{
		NetworkID: "net-1",
		LANPeers:  []string{srv.URL},
	})
	if err != nil {
		t.Fatalf("resolveLiveLANBootstrapPeers error = %v", err)
	}
	if len(peers) != 1 || peers[0] != "/ip4/127.0.0.1/tcp/51584/p2p/12D3KooWTestPeer" {
		t.Fatalf("peers = %#v", peers)
	}
}
