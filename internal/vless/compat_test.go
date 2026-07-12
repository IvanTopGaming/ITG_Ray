package vless

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeRejectsBadReality(t *testing.T) {
	cases := []struct {
		name string
		c    Config
	}{
		{"reality on ws", Config{Security: SecurityReality, Transport: TransportWS, RealityPublicKey: "k", SNI: "s"}},
		{"reality no pbk", Config{Security: SecurityReality, Transport: TransportTCP, SNI: "s"}},
		{"reality no sni", Config{Security: SecurityReality, Transport: TransportTCP, RealityPublicKey: "k"}},
	}
	for _, tc := range cases {
		c := tc.c
		if _, err := c.Normalize(); !errors.Is(err, ErrIncompatible) {
			t.Errorf("%s: want ErrIncompatible, got %v", tc.name, err)
		}
	}
}

func TestNormalizeDropsIncompatibleFlow(t *testing.T) {
	c := Config{Security: SecurityTLS, Transport: TransportWS, Flow: "xtls-rprx-vision"}
	msgs, err := c.Normalize()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if c.Flow != "" {
		t.Fatalf("flow should be dropped, got %q", c.Flow)
	}
	if len(msgs) == 0 || !strings.Contains(msgs[0], "flow") {
		t.Fatalf("expected a flow-drop message, got %v", msgs)
	}
}

func TestNormalizeKeepsValidVisionAndAcceptsRealityTransports(t *testing.T) {
	valid := Config{Security: SecurityReality, Transport: TransportTCP, Flow: "xtls-rprx-vision", RealityPublicKey: "k", SNI: "s"}
	if _, err := valid.Normalize(); err != nil {
		t.Fatalf("valid vision rejected: %v", err)
	}
	if valid.Flow != "xtls-rprx-vision" {
		t.Fatalf("valid flow dropped")
	}
	for _, tr := range []Transport{TransportTCP, TransportGRPC, TransportXHTTP} {
		c := Config{Security: SecurityReality, Transport: tr, RealityPublicKey: "k", SNI: "s"}
		if _, err := c.Normalize(); err != nil {
			t.Errorf("reality+%s rejected: %v", tr, err)
		}
	}
}

func TestNormalizeSoftWarnings(t *testing.T) {
	tls := Config{Security: SecurityTLS, Transport: TransportWS}
	msgs, err := tls.Normalize()
	if err != nil || len(msgs) == 0 {
		t.Fatalf("tls no-sni: want warning no error, got msgs=%v err=%v", msgs, err)
	}
	quic := Config{Security: SecurityNone, Transport: TransportQUIC}
	msgs, err = quic.Normalize()
	if err != nil || len(msgs) == 0 {
		t.Fatalf("quic+none: want warning no error, got msgs=%v err=%v", msgs, err)
	}
}
