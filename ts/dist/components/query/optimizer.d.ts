import { Query, Filter } from './knirvql';
import { Index, IndexType } from '../storage/index';
export interface QueryPlan {
    useIndex: boolean;
    indexName: string;
    indexType: IndexType;
    scanType: ScanType;
    filters: Filter[];
    limit: number;
    estimatedCost: number;
    estimatedRows: number;
    indexFilters: Filter[];
    postFilters: Filter[];
}
export declare enum ScanType {
    FullScan = 0,
    IndexScan = 1,
    IndexOnlyScan = 2
}
export interface CollectionStats {
    totalDocuments: number;
    indexStats: Map<string, IndexStats>;
}
export interface IndexStats {
    cardinality: number;
    selectivity: number;
    avgBucketSize: number;
}
export declare class QueryOptimizer {
    private collection;
    private indexes;
    private stats;
    constructor(collection: string, indexes: Index[], stats?: CollectionStats);
    optimize(query: Query): QueryPlan;
    private selectBestIndex;
    private analyzeIndexSuitability;
    private canUseIndex;
    private canUseBTreeIndex;
    private canUseGINIndex;
    private estimateIndexRows;
    private estimateFilterSelectivity;
    private estimateIndexCost;
    updateStats(documentCount: number, indexStats: Map<string, IndexStats>): void;
    getStats(): CollectionStats;
}
//# sourceMappingURL=optimizer.d.ts.map