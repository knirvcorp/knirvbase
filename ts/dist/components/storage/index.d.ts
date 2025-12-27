export declare enum IndexType {
    BTree = "btree",
    GIN = "gin",
    HNSW = "hnsw"
}
export declare class BTreeIndex {
    data: Map<string, string[]>;
}
export declare class GINIndex {
    data: Map<string, string[]>;
}
export declare class HNSWIndex {
    dimensions: number;
    vectors: Map<string, number[]>;
    neighbors: Map<string, string[]>;
    constructor(dimensions: number);
}
export declare class Index {
    name: string;
    collection: string;
    type: IndexType;
    fields: string[];
    unique: boolean;
    partialExpr: string;
    options: Record<string, any>;
    btreeIndex?: BTreeIndex;
    ginIndex?: GINIndex;
    hnswIndex?: HNSWIndex;
    constructor(name: string, collection: string, type: IndexType, fields: string[], unique: boolean, partialExpr: string, options: Record<string, any>);
}
export declare class IndexManager {
    private baseDir;
    private indexes;
    constructor(baseDir: string);
    createIndex(collection: string, name: string, indexType: IndexType, fields: string[], unique: boolean, partialExpr: string, options: Record<string, any>): Promise<void>;
    dropIndex(collection: string, name: string): Promise<void>;
    getIndex(collection: string, name: string): Index | null;
    getIndexesForCollection(collection: string): Index[];
    insert(collection: string, doc: Record<string, any>): Promise<void>;
    delete(collection: string, docID: string): Promise<void>;
    queryIndex(collection: string, indexName: string, query: Record<string, any>): Promise<string[]>;
    loadIndexes(): void;
    private matchesPartial;
    private insertBTree;
    private deleteBTree;
    private queryBTree;
    private insertGIN;
    private deleteGIN;
    private queryGIN;
    private insertHNSW;
    private deleteHNSW;
    private queryHNSW;
    private buildCompositeKey;
    private tokenizeJSON;
    private tokenizeValue;
    private cosineSimilarity;
    private saveIndexMetadata;
}
//# sourceMappingURL=index.d.ts.map