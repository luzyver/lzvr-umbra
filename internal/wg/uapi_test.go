package wg

import (
	"io"
	"net"
	"testing"
	"time"
)

func TestUAPICloseStopsIdleConnections(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	s := startUAPI(l, func(c net.Conn) { close(entered); _, _ = io.Copy(io.Discard, c) })
	c, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("handler UAPI tidak berjalan")
	}
	done := make(chan error, 1)
	go func() { done <- s.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown macet pada koneksi UAPI diam")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Read(make([]byte, 1)); err == nil {
		t.Fatal("koneksi UAPI masih terbuka")
	}
}
