//go:build linux

package server

import (
	"net"
	"os"
	"testing"
)

func TestRequirePeerUID_SameUID(t *testing.T) {
	a, b := socketPair(t)
	defer a.Close()
	defer b.Close()
	if err := requirePeerUID(a, uint32(os.Getuid())); err != nil {
		t.Fatalf("same-uid peer should pass, got %v", err)
	}
}

func TestRequirePeerUID_ForeignUID(t *testing.T) {
	a, b := socketPair(t)
	defer a.Close()
	defer b.Close()
	foreign := uint32(os.Getuid()) + 1
	if err := requirePeerUID(a, foreign); err == nil {
		t.Fatal("foreign-uid peer should be rejected")
	}
}

// socketPair returns two connected unix-domain sockets over which
// SO_PEERCRED reports the current process's uid on both ends.
func socketPair(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	dir := t.TempDir()
	ln, err := net.Listen("unix", dir+"/s.sock")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	type res struct {
		c   net.Conn
		err error
	}
	ch := make(chan res, 1)
	go func() {
		c, err := ln.Accept()
		ch <- res{c, err}
	}()
	client, err := net.Dial("unix", dir+"/s.sock")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	r := <-ch
	if r.err != nil {
		t.Fatalf("accept: %v", r.err)
	}
	return r.c, client
}
