package haonews

import (
	"net"
	"strconv"
	"strings"
	"testing"
)

func TestResolveLibP2PListenAddrsIncrementsSharedPort(t *testing.T) {
	t.Parallel()

	tcpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer tcpListener.Close()
	host, portText, err := net.SplitHostPort(tcpListener.Addr().String())
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}
	udpConn, err := net.ListenPacket("udp", net.JoinHostPort(host, portText))
	if err != nil {
		t.Fatalf("ListenPacket() error = %v", err)
	}
	defer udpConn.Close()

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
