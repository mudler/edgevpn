package config

import (
	"context"
	"testing"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/mudler/edgevpn/pkg/discovery"
	"github.com/mudler/edgevpn/pkg/node"
)

// discoveryDHT returns the DHT discovery that ToOpts sets up for c.
func discoveryDHT(t *testing.T, c Config) *discovery.DHT {
	t.Helper()
	c.NetworkToken = node.GenerateNewConnectionData().Base64()
	c.Discovery.DHT = true
	opts, _, err := c.ToOpts(nil)
	if err != nil {
		t.Fatalf("ToOpts() failed: %v", err)
	}
	var nc node.Config
	// Options that need a running node can fail here; only the discovery
	// services matter for this check.
	for _, o := range opts {
		_ = o(&nc)
	}
	for _, s := range nc.ServiceDiscovery {
		if d, ok := s.(*discovery.DHT); ok {
			return d
		}
	}
	t.Fatal("no DHT discovery")
	return nil
}

func TestToOptsPassesTheDHTOptions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := discoveryDHT(t, Config{Discovery: Discovery{DHTOptions: []dht.Option{dht.Mode(dht.ModeServer)}}})
	h, err := libp2p.New(libp2p.NoListenAddrs, d.Option(ctx))
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if got := d.IpfsDHT.Mode(); got != dht.ModeServer {
		t.Fatalf("DHT mode = %v, want server", got)
	}
}

func TestToOptsPassesPauseAndWake(t *testing.T) {
	paused := false
	wake := make(chan struct{})
	d := discoveryDHT(t, Config{Discovery: Discovery{
		Pause: func() bool { paused = true; return true },
		Wake:  wake,
	}})
	if d.Pause == nil || !d.Pause() || !paused {
		t.Fatal("the DHT discovery does not ask the configured Pause")
	}
	if d.Wake != (<-chan struct{})(wake) {
		t.Fatal("the DHT discovery does not listen on the configured Wake")
	}
}

func TestToOptsLeavesDiscoveryUnpausedByDefault(t *testing.T) {
	d := discoveryDHT(t, Config{})
	if d.Pause != nil || d.Wake != nil {
		t.Fatal("Pause or Wake set without being configured")
	}
}
