package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	BlocksCommitted     prometheus.Counter
	BlockCommitDuration prometheus.Histogram
	MemoryStoreOps      prometheus.Counter
	MemoryRetrieveOps   prometheus.Counter
	CacheHits           prometheus.Counter
	CacheMisses         prometheus.Counter
	ActiveConnections   prometheus.Gauge
	NRNBalance          prometheus.Gauge
	QueryLatency        prometheus.Histogram
	ErrorCount          prometheus.Counter
	IndexSize           prometheus.Gauge
}

func NewMetrics() *Metrics {
	return &Metrics{
		BlocksCommitted: promauto.NewCounter(prometheus.CounterOpts{
			Name: "knirvbase_blocks_committed_total",
			Help: "Total number of blocks committed to the chain",
		}),
		BlockCommitDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "knirvbase_block_commit_duration_seconds",
			Help:    "Time taken to commit a block",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		}),
		MemoryStoreOps: promauto.NewCounter(prometheus.CounterOpts{
			Name: "knirvbase_memory_store_ops_total",
			Help: "Total number of memory store operations",
		}),
		MemoryRetrieveOps: promauto.NewCounter(prometheus.CounterOpts{
			Name: "knirvbase_memory_retrieve_ops_total",
			Help: "Total number of memory retrieve operations",
		}),
		CacheHits: promauto.NewCounter(prometheus.CounterOpts{
			Name: "knirvbase_cache_hits_total",
			Help: "Total number of cache hits",
		}),
		CacheMisses: promauto.NewCounter(prometheus.CounterOpts{
			Name: "knirvbase_cache_misses_total",
			Help: "Total number of cache misses",
		}),
		ActiveConnections: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "knirvbase_active_connections",
			Help: "Number of active connections",
		}),
		NRNBalance: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "knirvbase_nrn_balance",
			Help: "Current NRN token balance",
		}),
		QueryLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "knirvbase_query_latency_seconds",
			Help:    "Query latency distribution",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 10),
		}),
		ErrorCount: promauto.NewCounter(prometheus.CounterOpts{
			Name: "knirvbase_errors_total",
			Help: "Total number of errors",
		}),
		IndexSize: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "knirvbase_index_size_bytes",
			Help: "Size of the index in bytes",
		}),
	}
}
