package distributed

import (
	"math"
	"sort"

	stor "github.com/knirv/knirvbase/internal/storage"
)

// QueryPlan represents an optimized execution plan for a query
type QueryPlan struct {
	// Execution strategy
	UseIndex  bool
	IndexName string
	IndexType stor.IndexType
	ScanType  ScanType // FullScan, IndexScan, IndexOnlyScan

	// Query parameters
	Filters   []Filter
	SortOrder SortOrder
	Limit     int

	// Cost estimation
	EstimatedCost float64
	EstimatedRows int

	// Execution details
	IndexFilters []Filter // Filters that can be pushed to index
	PostFilters  []Filter // Filters that need post-processing
}

// ScanType represents the type of scan to perform
type ScanType int

const (
	FullScan ScanType = iota
	IndexScan
	IndexOnlyScan
)

// SortOrder represents sorting requirements
type SortOrder struct {
	Field     string
	Ascending bool
}

// QueryOptimizer analyzes queries and creates optimal execution plans
type QueryOptimizer struct {
	collection string
	indexes    []*stor.Index
	stats      *CollectionStats
}

// CollectionStats holds statistics about a collection for cost estimation
type CollectionStats struct {
	TotalDocuments int
	IndexStats     map[string]*IndexStats
}

// IndexStats holds statistics about an index
type IndexStats struct {
	Cardinality   int     // Number of unique values
	Selectivity   float64 // Fraction of documents matching a filter (0-1)
	AvgBucketSize float64 // Average number of documents per index key
}

// NewQueryOptimizer creates a new query optimizer
func NewQueryOptimizer(collection string, indexes []*stor.Index, stats *CollectionStats) *QueryOptimizer {
	if stats == nil {
		stats = &CollectionStats{
			TotalDocuments: 1000, // Default estimate
			IndexStats:     make(map[string]*IndexStats),
		}
		// Initialize default stats for indexes
		for _, idx := range indexes {
			stats.IndexStats[idx.Name] = &IndexStats{
				Cardinality:   100,
				Selectivity:   0.1,
				AvgBucketSize: 10.0,
			}
		}
	}

	return &QueryOptimizer{
		collection: collection,
		indexes:    indexes,
		stats:      stats,
	}
}

// Optimize creates an optimal query plan for the given query
func (qo *QueryOptimizer) Optimize(query *Query) (*QueryPlan, error) {
	plan := &QueryPlan{
		Filters:       query.Filters,
		SortOrder:     SortOrder{}, // Not implemented yet
		Limit:         query.Limit,
		ScanType:      FullScan,
		EstimatedRows: qo.stats.TotalDocuments,
	}

	// If no filters, always do full scan
	if len(query.Filters) == 0 {
		plan.EstimatedCost = float64(qo.stats.TotalDocuments)
		return plan, nil
	}

	// Analyze filters and find best index
	bestIndex, indexFilters, postFilters := qo.selectBestIndex(query.Filters)

	if bestIndex != nil {
		plan.UseIndex = true
		plan.IndexName = bestIndex.Name
		plan.IndexType = bestIndex.Type
		plan.IndexFilters = indexFilters
		plan.PostFilters = postFilters

		// Estimate cost and rows for index scan
		plan.EstimatedRows = qo.estimateIndexRows(bestIndex, indexFilters)
		plan.EstimatedCost = qo.estimateIndexCost(bestIndex, indexFilters, plan.EstimatedRows)

		// Determine scan type
		if len(postFilters) == 0 {
			plan.ScanType = IndexOnlyScan
		} else {
			plan.ScanType = IndexScan
		}
	} else {
		// Full scan
		plan.EstimatedCost = float64(qo.stats.TotalDocuments)
		plan.PostFilters = query.Filters
	}

	return plan, nil
}

// selectBestIndex chooses the best index for the given filters
func (qo *QueryOptimizer) selectBestIndex(filters []Filter) (*stor.Index, []Filter, []Filter) {
	if len(filters) == 0 {
		return nil, nil, nil
	}

	type indexCandidate struct {
		index        *stor.Index
		indexFilters []Filter
		postFilters  []Filter
		cost         float64
		rows         int
	}

	var candidates []indexCandidate

	// Evaluate each index
	for _, idx := range qo.indexes {
		indexFilters, postFilters := qo.analyzeIndexSuitability(idx, filters)

		if len(indexFilters) > 0 {
			estimatedRows := qo.estimateIndexRows(idx, indexFilters)
			estimatedCost := qo.estimateIndexCost(idx, indexFilters, estimatedRows)

			candidates = append(candidates, indexCandidate{
				index:        idx,
				indexFilters: indexFilters,
				postFilters:  postFilters,
				cost:         estimatedCost,
				rows:         estimatedRows,
			})
		}
	}

	if len(candidates) == 0 {
		return nil, nil, filters
	}

	// Sort by cost (lowest first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].cost < candidates[j].cost
	})

	best := candidates[0]
	return best.index, best.indexFilters, best.postFilters
}

// analyzeIndexSuitability determines which filters can use an index
func (qo *QueryOptimizer) analyzeIndexSuitability(idx *stor.Index, filters []Filter) ([]Filter, []Filter) {
	var indexFilters []Filter
	var postFilters []Filter

	for _, filter := range filters {
		if qo.canUseIndex(idx, filter) {
			indexFilters = append(indexFilters, filter)
		} else {
			postFilters = append(postFilters, filter)
		}
	}

	return indexFilters, postFilters
}

// canUseIndex checks if a filter can use the given index
func (qo *QueryOptimizer) canUseIndex(idx *stor.Index, filter Filter) bool {
	switch idx.Type {
	case stor.IndexTypeBTree:
		// B-Tree indexes support exact matches and range queries
		return qo.canUseBTreeIndex(idx, filter)
	case stor.IndexTypeGIN:
		// GIN indexes support text search and JSON queries
		return qo.canUseGINIndex(idx, filter)
	case stor.IndexTypeHNSW:
		// HNSW indexes are for vector similarity
		return false // Not for regular filters
	default:
		return false
	}
}

// canUseBTreeIndex checks if a B-Tree index can be used for a filter
func (qo *QueryOptimizer) canUseBTreeIndex(idx *stor.Index, filter Filter) bool {
	// Check if the filter field is indexed
	for _, field := range idx.Fields {
		if filter.Key == field {
			return true
		}
	}
	return false
}

// canUseGINIndex checks if a GIN index can be used for a filter
func (qo *QueryOptimizer) canUseGINIndex(_ *stor.Index, _ Filter) bool {
	// GIN indexes can be used for text search
	// For now, support CONTAINS operations
	return true // Simplified - assume GIN can handle most text filters
}

// estimateIndexRows estimates the number of rows returned by an index scan
func (qo *QueryOptimizer) estimateIndexRows(idx *stor.Index, filters []Filter) int {
	if len(filters) == 0 {
		return qo.stats.TotalDocuments
	}

	stats := qo.stats.IndexStats[idx.Name]
	if stats == nil {
		// Default estimate
		return int(float64(qo.stats.TotalDocuments) * 0.1)
	}

	// Start with total documents
	estimatedRows := float64(qo.stats.TotalDocuments)

	// Apply selectivity for each filter
	for _, filter := range filters {
		selectivity := qo.estimateFilterSelectivity(idx, filter)
		estimatedRows *= selectivity
	}

	return int(math.Max(1, estimatedRows))
}

// estimateFilterSelectivity estimates the selectivity of a filter
func (qo *QueryOptimizer) estimateFilterSelectivity(idx *stor.Index, _ Filter) float64 {
	stats := qo.stats.IndexStats[idx.Name]
	if stats == nil {
		return 0.1 // Default selectivity
	}

	// For B-Tree indexes, use cardinality
	if idx.Type == stor.IndexTypeBTree {
		if stats.Cardinality > 0 {
			return 1.0 / float64(stats.Cardinality)
		}
	}

	// For GIN indexes, assume lower selectivity for text search
	if idx.Type == stor.IndexTypeGIN {
		return 0.01 // Very selective for text search
	}

	return stats.Selectivity
}

// estimateIndexCost estimates the cost of using an index
func (qo *QueryOptimizer) estimateIndexCost(idx *stor.Index, _ []Filter, estimatedRows int) float64 {
	stats := qo.stats.IndexStats[idx.Name]
	if stats == nil {
		stats = &IndexStats{AvgBucketSize: 10.0}
	}

	// Cost model: index lookup cost + data retrieval cost
	indexLookupCost := 1.0 // Cost of index lookup
	dataRetrievalCost := float64(estimatedRows) * stats.AvgBucketSize

	return indexLookupCost + dataRetrievalCost
}

// UpdateStats updates collection statistics (called after inserts/updates/deletes)
func (qo *QueryOptimizer) UpdateStats(documentCount int, indexStats map[string]*IndexStats) {
	qo.stats.TotalDocuments = documentCount
	qo.stats.IndexStats = indexStats
}

// GetStats returns current collection statistics
func (qo *QueryOptimizer) GetStats() *CollectionStats {
	return qo.stats
}
