import { EntryType } from '../types/types';
export declare class KNIRVQLParser {
    /**
     * Parse parses a KNIRVQL query
     */
    parse(query: string): Query | null;
    private parseGet;
    private parseSet;
    private parseDelete;
    private parseCreate;
    private parseCreateIndex;
    private parseCreateCollection;
    private parseDrop;
    private parseDropIndex;
    private parseDropCollection;
}
export interface Query {
    type: QueryType;
    entryType?: EntryType;
    collection?: string;
    id?: string;
    key?: string;
    value?: string;
    filters?: Filter[];
    similarTo?: number[];
    limit?: number;
    indexName?: string;
    fields?: string[];
    unique?: boolean;
}
export declare enum QueryType {
    Get = 0,
    Set = 1,
    Delete = 2,
    CreateIndex = 3,
    CreateCollection = 4,
    DropIndex = 5,
    DropCollection = 6
}
export interface Filter {
    key: string;
    operator: string;
    value: any;
}
export interface QueryResult {
    success: boolean;
    data?: any;
    error?: string;
}
export interface QueryExecutor {
    execute(query: Query): Promise<QueryResult>;
}
//# sourceMappingURL=knirvql.d.ts.map