package monitoring

import (
	"testing"
)

func TestNewMetrics(t *testing.T) {
	metrics := NewMetrics()
	if metrics == nil {
		t.Fatal("Expected Metrics, got nil")
	}

	// Test that all metrics are initialized
	if metrics.BlocksCommitted == nil {
		t.Error("Expected BlocksCommitted to be initialized")
	}
	if metrics.BlockCommitDuration == nil {
		t.Error("Expected BlockCommitDuration to be initialized")
	}
	if metrics.MemoryStoreOps == nil {
		t.Error("Expected MemoryStoreOps to be initialized")
	}
	if metrics.MemoryRetrieveOps == nil {
		t.Error("Expected MemoryRetrieveOps to be initialized")
	}
	if metrics.CacheHits == nil {
		t.Error("Expected CacheHits to be initialized")
	}
	if metrics.CacheMisses == nil {
		t.Error("Expected CacheMisses to be initialized")
	}
	if metrics.ActiveConnections == nil {
		t.Error("Expected ActiveConnections to be initialized")
	}
	if metrics.NRNBalance == nil {
		t.Error("Expected NRNBalance to be initialized")
	}
	if metrics.QueryLatency == nil {
		t.Error("Expected QueryLatency to be initialized")
	}
	if metrics.ErrorCount == nil {
		t.Error("Expected ErrorCount to be initialized")
	}
	if metrics.IndexSize == nil {
		t.Error("Expected IndexSize to be initialized")
	}
}
