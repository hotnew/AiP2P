package aip2p

import (
	"net"
	"strconv"
	"strings"
	"testing"
)

func TestResolveLibP2PListenAddrsIncrementsSharedPort(t *testing.T) {
	t.Parallel()

	tcpListener, udpConn, err := listenSharedTCPUDP("127.0.0.1")
	if err != nil {
		t.Fatalf("listenSharedTCPUDP() error = %v", err)
	}
	defer tcpListener.Close()
	defer udpConn.Close()
	_, portText, err := net.SplitHostPort(tcpListener.Addr().String())
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}

	addrs := []string{
		"/ip4/127.0.0.1/tcp/" + portText,
		"/ip4/127.0.0.1/udp/" + portText + "/quic-v1",
	}
	resolved, err := resolveLibP2PListenAddrs(addrs)
	if err != nil {
		t.Fatalf("resolveLibP2PListenAddrs() error = %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("len(resolved) = %d, want 2", len(resolved))
	}
	if resolved[0] == addrs[0] || resolved[1] == addrs[1] {
		t.Fatalf("resolved addrs = %#v, want incremented shared port", resolved)
	}
	tcpPort := strings.TrimPrefix(resolved[0], "/ip4/127.0.0.1/tcp/")
	udpPort := strings.TrimPrefix(strings.TrimSuffix(resolved[1], "/quic-v1"), "/ip4/127.0.0.1/udp/")
	if tcpPort != udpPort {
		t.Fatalf("resolved ports differ: tcp=%q udp=%q", tcpPort, udpPort)
	}
	if _, err := strconv.Atoi(tcpPort); err != nil {
		t.Fatalf("resolved port = %q, want integer", tcpPort)
	}
}

func listenSharedTCPUDP(host string) (net.Listener, net.PacketConn, error) {
	for attempt := 0; attempt < 32; attempt++ {
		tcpListener, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
		if err != nil {
			return nil, nil, err
		}
		_, portText, err := net.SplitHostPort(tcpListener.Addr().String())
		if err != nil {
			_ = tcpListener.Close()
			return nil, nil, err
		}
		udpConn, err := net.ListenPacket("udp", net.JoinHostPort(host, portText))
		if err == nil {
			return tcpListener, udpConn, nil
		}
		_ = tcpListener.Close()
	}
	return nil, nil, &net.AddrError{Err: "no shared tcp/udp port found", Addr: host}
}
