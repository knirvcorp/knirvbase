import { IndexType } from '../storage/index';
// ScanType represents the type of scan to perform
export var ScanType;
(function (ScanType) {
    ScanType[ScanType["FullScan"] = 0] = "FullScan";
    ScanType[ScanType["IndexScan"] = 1] = "IndexScan";
    ScanType[ScanType["IndexOnlyScan"] = 2] = "IndexOnlyScan";
})(ScanType || (ScanType = {}));
// QueryOptimizer analyzes queries and creates optimal execution plans
export class QueryOptimizer {
    constructor(collection, indexes, stats) {
        this.collection = collection;
        this.indexes = indexes;
        this.stats = stats || {
            totalDocuments: 1000, // Default estimate
            indexStats: new Map(),
        };
        // Initialize default stats for indexes
        for (const idx of this.indexes) {
            if (!this.stats.indexStats.has(idx.name)) {
                this.stats.indexStats.set(idx.name, {
                    cardinality: 100,
                    selectivity: 0.1,
                    avgBucketSize: 10.0,
                });
            }
        }
    }
    // Optimize creates an optimal query plan for the given query
    optimize(query) {
        const plan = {
            useIndex: false,
            indexName: '',
            indexType: IndexType.BTree,
            scanType: ScanType.FullScan,
            filters: query.filters || [],
            limit: query.limit || 0,
            estimatedCost: 0,
            estimatedRows: this.stats.totalDocuments,
            indexFilters: [],
            postFilters: [],
        };
        // If no filters, always do full scan
        if (!query.filters || query.filters.length === 0) {
            plan.estimatedRows = (query.limit && query.limit > 0) ? Math.min(this.stats.totalDocuments, query.limit) : this.stats.totalDocuments;
            plan.estimatedCost = plan.estimatedRows;
            return plan;
        }
        // Analyze filters and find best index
        const bestIndex = this.selectBestIndex(query.filters);
        if (bestIndex) {
            plan.useIndex = true;
            plan.indexName = bestIndex.index.name;
            plan.indexType = bestIndex.index.type;
            plan.indexFilters = bestIndex.indexFilters;
            plan.postFilters = bestIndex.postFilters;
            // Estimate cost and rows for index scan
            plan.estimatedRows = this.estimateIndexRows(bestIndex.index, bestIndex.indexFilters);
            plan.estimatedCost = this.estimateIndexCost(bestIndex.index, bestIndex.indexFilters, plan.estimatedRows);
            // Determine scan type
            if (bestIndex.postFilters.length === 0) {
                plan.scanType = ScanType.IndexOnlyScan;
            }
            else {
                plan.scanType = ScanType.IndexScan;
            }
        }
        else {
            // Full scan
            plan.estimatedCost = this.stats.totalDocuments;
            plan.postFilters = query.filters;
        }
        // Apply limit to estimated rows if specified
        if (plan.limit > 0 && plan.estimatedRows > plan.limit) {
            plan.estimatedRows = plan.limit;
        }
        return plan;
    }
    // selectBestIndex chooses the best index for the given filters
    selectBestIndex(filters) {
        if (filters.length === 0) {
            return null;
        }
        const candidates = [];
        // Evaluate each index
        for (const idx of this.indexes) {
            const { indexFilters, postFilters } = this.analyzeIndexSuitability(idx, filters);
            if (indexFilters.length > 0) {
                const estimatedRows = this.estimateIndexRows(idx, indexFilters);
                const estimatedCost = this.estimateIndexCost(idx, indexFilters, estimatedRows);
                candidates.push({
                    index: idx,
                    indexFilters,
                    postFilters,
                    cost: estimatedCost,
                    rows: estimatedRows,
                });
            }
        }
        if (candidates.length === 0) {
            return null;
        }
        // Sort by cost (lowest first)
        candidates.sort((a, b) => a.cost - b.cost);
        const best = candidates[0];
        return {
            index: best.index,
            indexFilters: best.indexFilters,
            postFilters: best.postFilters,
        };
    }
    // analyzeIndexSuitability determines which filters can use an index
    analyzeIndexSuitability(idx, filters) {
        const indexFilters = [];
        const postFilters = [];
        for (const filter of filters) {
            if (this.canUseIndex(idx, filter)) {
                indexFilters.push(filter);
            }
            else {
                postFilters.push(filter);
            }
        }
        return { indexFilters, postFilters };
    }
    // canUseIndex checks if a filter can use the given index
    canUseIndex(idx, filter) {
        switch (idx.type) {
            case IndexType.BTree:
                return this.canUseBTreeIndex(idx, filter);
            case IndexType.GIN:
                return this.canUseGINIndex(idx, filter);
            case IndexType.HNSW:
                return false; // Not for regular filters
            default:
                return false;
        }
    }
    // canUseBTreeIndex checks if a B-Tree index can be used for a filter
    canUseBTreeIndex(idx, filter) {
        // Check if the filter field is indexed
        return idx.fields.includes(filter.key);
    }
    // canUseGINIndex checks if a GIN index can be used for a filter
    canUseGINIndex(_idx, _filter) {
        // GIN indexes can be used for text search
        // For now, support CONTAINS operations
        return true; // Simplified - assume GIN can handle most text filters
    }
    // estimateIndexRows estimates the number of rows returned by an index scan
    estimateIndexRows(idx, filters) {
        if (filters.length === 0) {
            return this.stats.totalDocuments;
        }
        const stats = this.stats.indexStats.get(idx.name);
        if (!stats) {
            return Math.floor(this.stats.totalDocuments * 0.1);
        }
        // Start with total documents
        let estimatedRows = this.stats.totalDocuments;
        // Apply selectivity for each filter
        for (const filter of filters) {
            const selectivity = this.estimateFilterSelectivity(idx, filter);
            estimatedRows *= selectivity;
        }
        return Math.max(1, Math.floor(estimatedRows));
    }
    // estimateFilterSelectivity estimates the selectivity of a filter
    estimateFilterSelectivity(idx, _filter) {
        const stats = this.stats.indexStats.get(idx.name);
        if (!stats) {
            return 0.1; // Default selectivity
        }
        // For B-Tree indexes, use cardinality
        if (idx.type === IndexType.BTree) {
            if (stats.cardinality > 0) {
                return 1.0 / stats.cardinality;
            }
        }
        // For GIN indexes, assume lower selectivity for text search
        if (idx.type === IndexType.GIN) {
            return 0.01; // Very selective for text search
        }
        return stats.selectivity;
    }
    // estimateIndexCost estimates the cost of using an index
    estimateIndexCost(idx, _filters, estimatedRows) {
        const stats = this.stats.indexStats.get(idx.name);
        const avgBucketSize = stats ? stats.avgBucketSize : 10.0;
        // Cost model: index lookup cost + data retrieval cost
        const indexLookupCost = 1.0;
        const dataRetrievalCost = estimatedRows * avgBucketSize;
        return indexLookupCost + dataRetrievalCost;
    }
    // UpdateStats updates collection statistics (called after inserts/updates/deletes)
    updateStats(documentCount, indexStats) {
        this.stats.totalDocuments = documentCount;
        this.stats.indexStats = indexStats;
    }
    // GetStats returns current collection statistics
    getStats() {
        return this.stats;
    }
}
//# sourceMappingURL=optimizer.js.map