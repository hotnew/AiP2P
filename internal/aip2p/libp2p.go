package aip2p

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	mdns "github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
)

type libp2pRuntime struct {
	host               host.Host
	dht                *kaddht.IpfsDHT
	ping               *ping.PingService
	mdns               mdns.Service
	mdnsTracker        *mdnsTracker
	networkID          string
	mdnsServiceName    string
	bootstraps         []peer.AddrInfo
	rendezvous         []string
	bootstrapWarning   string
	lastBootstrappedAt *time.Time
}

func startLibP2PRuntime(ctx context.Context, cfg NetworkBootstrapConfig) (*libp2pRuntime, error) {
	if len(cfg.LibP2PBootstrap) == 0 && len(cfg.LibP2PRendezvous) == 0 && len(cfg.LANPeers) == 0 {
		return nil, nil
	}

	h, err := libp2p.New(
		libp2p.Ping(true),
	)
	if err != nil {
		return nil, fmt.Errorf("create libp2p host: %w", err)
	}

	bootstrapValues := append([]string(nil), cfg.LibP2PBootstrap...)
	resolvedLANPeers, lanErr := resolveLANBootstrapPeers(ctx, cfg)
	bootstrapValues = append(bootstrapValues, resolvedLANPeers...)
	peers, err := parseBootstrapPeers(bootstrapValues)
	if err != nil {
		_ = h.Close()
		return nil, err
	}

	options := []kaddht.Option{kaddht.Mode(kaddht.ModeAutoServer)}
	if len(peers) > 0 {
		options = append(options, kaddht.BootstrapPeers(peers...))
	}
	dht, err := kaddht.New(ctx, h, options...)
	if err != nil {
		_ = h.Close()
		return nil, fmt.Errorf("create libp2p dht: %w", err)
	}
	if err := dht.Bootstrap(ctx); err != nil {
		_ = dht.Close()
		_ = h.Close()
		return nil, fmt.Errorf("bootstrap libp2p dht: %w", err)
	}
	mdnsTracker := newMDNSTracker(h)
	serviceName := mdnsServiceName(cfg.NetworkID)
	mdnsService := mdns.NewMdnsService(h, serviceName, mdnsTracker)
	if err := mdnsService.Start(); err != nil {
		_ = dht.Close()
		_ = h.Close()
		return nil, fmt.Errorf("start libp2p mdns: %w", err)
	}
	now := time.Now().UTC()
	return &libp2pRuntime{
		host:            h,
		dht:             dht,
		ping:            ping.NewPingService(h),
		mdns:            mdnsService,
		mdnsTracker:     mdnsTracker,
		networkID:       cfg.NetworkID,
		mdnsServiceName: serviceName,
		bootstraps:      peers,
		rendezvous:      append([]string(nil), cfg.LibP2PRendezvous...),
		bootstrapWarning: func() string {
			if lanErr != nil {
				return lanErr.Error()
			}
			return ""
		}(),
		lastBootstrappedAt: &now,
	}, nil
}

func (r *libp2pRuntime) Close() error {
	if r == nil {
		return nil
	}
	if r.mdns != nil {
		_ = r.mdns.Close()
	}
	if r.dht != nil {
		_ = r.dht.Close()
	}
	if r.host != nil {
		return r.host.Close()
	}
	return nil
}

func (r *libp2pRuntime) Status(ctx context.Context) SyncLibP2PStatus {
	if r == nil {
		return SyncLibP2PStatus{}
	}

	status := SyncLibP2PStatus{
		Enabled:              true,
		PeerID:               r.host.ID().String(),
		ConfiguredBootstrap:  len(r.bootstraps),
		ConfiguredRendezvous: len(r.rendezvous),
		MDNS: SyncMDNSStatus{
			Enabled:     r.mdns != nil,
			ServiceName: r.mdnsServiceName,
		},
		LastBootstrapAt: r.lastBootstrappedAt,
	}
	if r.bootstrapWarning != "" {
		status.LastError = r.bootstrapWarning
	}
	for _, addr := range r.host.Addrs() {
		status.ListenAddrs = append(status.ListenAddrs, addr.String())
	}

	peerStates := make([]SyncPeerRef, 0, len(r.bootstraps))
	for _, info := range r.bootstraps {
		state := SyncPeerRef{
			PeerID:  info.ID.String(),
			Address: firstPeerAddr(info),
		}
		if len(r.host.Network().ConnsToPeer(info.ID)) == 0 {
			connectCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
			err := r.host.Connect(connectCtx, info)
			cancel()
			if err != nil {
				state.Error = err.Error()
				peerStates = append(peerStates, state)
				continue
			}
			now := time.Now().UTC()
			r.lastBootstrappedAt = &now
			status.LastBootstrapAt = r.lastBootstrappedAt
		}
		state.Connected = true
		status.ConnectedBootstrap++

		pingCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		result := <-r.ping.Ping(pingCtx, info.ID)
		cancel()
		if result.Error != nil {
			state.Error = result.Error.Error()
		} else {
			state.Reachable = true
			state.RTT = result.RTT.String()
			status.ReachableBootstrap++
		}
		peerStates = append(peerStates, state)
	}

	status.ConnectedPeers = len(r.host.Network().Peers())
	if r.dht != nil {
		status.RoutingTablePeers = r.dht.RoutingTable().Size()
	}
	status.Peers = peerStates
	if r.mdnsTracker != nil {
		status.MDNS = r.mdnsTracker.status(r.host)
	}
	return status
}

func parseBootstrapPeers(values []string) ([]peer.AddrInfo, error) {
	out := make([]peer.AddrInfo, 0, len(values))
	seen := make(map[string]struct{})
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		info, err := peer.AddrInfoFromString(value)
		if err != nil {
			return nil, fmt.Errorf("parse libp2p bootstrap peer %q: %w", value, err)
		}
		key := info.ID.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, *info)
	}
	return out, nil
}

func firstPeerAddr(info peer.AddrInfo) string {
	if len(info.Addrs) == 0 {
		return ""
	}
	return info.Addrs[0].String()
}

func mdnsServiceName(networkID string) string {
	networkID = normalizeNetworkID(networkID)
	if len(networkID) >= 12 {
		return "_aip2p-" + networkID[:12] + "._udp"
	}
	return "_aip2p._udp"
}

type mdnsPeerState struct {
	SyncPeerRef
	LastSeen time.Time
}

type mdnsTracker struct {
	host        host.Host
	mu          sync.RWMutex
	lastError   string
	lastFoundAt *time.Time
	peers       map[string]mdnsPeerState
}

func newMDNSTracker(h host.Host) *mdnsTracker {
	return &mdnsTracker{
		host:  h,
		peers: make(map[string]mdnsPeerState),
	}
}

func (m *mdnsTracker) HandlePeerFound(info peer.AddrInfo) {
	if info.ID == m.host.ID() {
		return
	}
	now := time.Now().UTC()
	state := mdnsPeerState{
		SyncPeerRef: SyncPeerRef{
			PeerID:  info.ID.String(),
			Address: firstPeerAddr(info),
		},
		LastSeen: now,
	}

	connectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err := m.host.Connect(connectCtx, info)
	cancel()
	if err != nil {
		state.Error = err.Error()
	} else {
		state.Connected = true
		state.Reachable = true
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if prev, ok := m.peers[state.PeerID]; ok && state.Address == "" {
		state.Address = prev.Address
	}
	m.peers[state.PeerID] = state
	m.lastFoundAt = &now
	if err != nil {
		m.lastError = err.Error()
	}
}

func (m *mdnsTracker) status(h host.Host) SyncMDNSStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	status := SyncMDNSStatus{
		Enabled:          true,
		ServiceName:      "_aip2p._udp",
		DiscoveredPeers:  len(m.peers),
		LastDiscoveredAt: m.lastFoundAt,
		LastError:        m.lastError,
		Peers:            make([]SyncPeerRef, 0, len(m.peers)),
	}
	for _, state := range m.peers {
		ref := state.SyncPeerRef
		if len(h.Network().ConnsToPeer(peer.ID(state.PeerID))) > 0 {
			ref.Connected = true
			ref.Reachable = true
			status.ConnectedPeers++
		}
		status.Peers = append(status.Peers, ref)
	}
	return status
}
