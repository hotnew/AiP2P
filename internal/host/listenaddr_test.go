package host

import (
	"net"
	"testing"
)

func TestResolveListenAddrIncrementsWhenPortIsOccupied(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()
	next, err := resolveListenAddr(addr)
	if err != nil {
		t.Fatalf("resolveListenAddr() error = %v", err)
	}
	if next == addr {
		t.Fatalf("resolveListenAddr() = %q, want incremented port", next)
	}
}

func TestResolveListenAddrKeepsEphemeralPort(t *testing.T) {
	t.Parallel()

	next, err := resolveListenAddr("127.0.0.1:0")
	if err != nil {
		t.Fatalf("resolveListenAddr() error = %v", err)
	}
	if next != "127.0.0.1:0" {
		t.Fatalf("resolveListenAddr() = %q, want unchanged", next)
	}
}
