import { Index, IndexType } from './index';
export interface Storage {
    insert(collection: string, doc: Record<string, any>): Promise<void>;
    update(collection: string, id: string, update: Record<string, any>): Promise<number>;
    delete(collection: string, id: string): Promise<number>;
    find(collection: string, id: string): Promise<Record<string, any> | null>;
    findAll(collection: string): Promise<Record<string, any>[]>;
    createIndex(collection: string, name: string, indexType: IndexType, fields: string[], unique: boolean, partialExpr: string, options: Record<string, any>): Promise<void>;
    dropIndex(collection: string, name: string): Promise<void>;
    getIndex(collection: string, name: string): Index | null;
    getIndexesForCollection(collection: string): Index[];
    queryIndex(collection: string, indexName: string, query: Record<string, any>): Promise<string[]>;
}
export declare class FileStorage implements Storage {
    private baseDir;
    private indexManager;
    private encryptionMgr;
    constructor(baseDir: string);
    private getCollectionDir;
    private getDocPath;
    setMasterKey(keyPair: any): void;
    private isEncryptedCollection;
    insert(collection: string, doc: Record<string, any>): Promise<void>;
    update(collection: string, id: string, update: Record<string, any>): Promise<number>;
    delete(collection: string, id: string): Promise<number>;
    find(collection: string, id: string): Promise<Record<string, any> | null>;
    findAll(collection: string): Promise<Record<string, any>[]>;
    private saveBlob;
    private loadBlob;
    createIndex(collection: string, name: string, indexType: IndexType, fields: string[], unique: boolean, partialExpr: string, options: Record<string, any>): Promise<void>;
    dropIndex(collection: string, name: string): Promise<void>;
    getIndex(collection: string, name: string): Index | null;
    getIndexesForCollection(collection: string): Index[];
    queryIndex(collection: string, indexName: string, query: Record<string, any>): Promise<string[]>;
    private deepCopyDoc;
    private encryptDocument;
    private encryptPayload;
    private isSensitiveField;
    private decryptDocument;
    private decryptPayload;
}
//# sourceMappingURL=storage.d.ts.map