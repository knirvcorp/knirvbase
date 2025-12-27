package network

import (
	"context"
	"testing"

	"github.com/knirvcorp/knirvbase/go/internal/types"
)

func TestNewNetworkManager(t *testing.T) {
	ctx := context.Background()
	nm := NewNetworkManager(ctx)
	if nm == nil {
		t.Error("NetworkManager is nil")
	}
	if nm.peerID == "" {
		t.Error("PeerID is empty")
	}
}

func TestNetworkManagerGetPeerID(t *testing.T) {
	ctx := context.Background()
	nm := NewNetworkManager(ctx)
	id := nm.GetPeerID()
	if id == "" {
		t.Error("PeerID is empty")
	}
}

func TestNetworkManagerCreateNetwork(t *testing.T) {
	ctx := context.Background()
	nm := NewNetworkManager(ctx)
	cfg := types.NetworkConfig{NetworkID: "test-net", Name: "Test Network"}
	id, err := nm.CreateNetwork(cfg)
	if err != nil {
		t.Errorf("CreateNetwork failed: %v", err)
	}
	if id != "test-net" {
		t.Errorf("Expected id 'test-net', got %s", id)
	}
}

func TestNetworkManagerGetNetworkCollections(t *testing.T) {
	ctx := context.Background()
	nm := NewNetworkManager(ctx)
	collections := nm.GetNetworkCollections("nonexistent")
	if collections != nil {
		t.Errorf("Expected nil for nonexistent network, got %v", collections)
	}
}

func TestNetworkManagerGetNetworks(t *testing.T) {
	ctx := context.Background()
	nm := NewNetworkManager(ctx)
	networks := nm.GetNetworks()
	if networks == nil {
		t.Error("GetNetworks returned nil")
	}
}

func TestNetworkManagerShutdown(t *testing.T) {
	ctx := context.Background()
	nm := NewNetworkManager(ctx)
	err := nm.Shutdown()
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}
