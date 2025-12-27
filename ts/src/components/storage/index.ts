import * as fs from 'fs';
import * as path from 'path';

export enum IndexType {
  BTree = 'btree',
  GIN = 'gin',
  HNSW = 'hnsw',
}

export class BTreeIndex {
  data: Map<string, string[]> = new Map(); // value -> []documentID
}

export class GINIndex {
  data: Map<string, string[]> = new Map(); // token -> []documentID
}

export class HNSWIndex {
  dimensions: number;
  vectors: Map<string, number[]> = new Map(); // documentID -> vector
  neighbors: Map<string, string[]> = new Map(); // documentID -> []neighborIDs

  constructor(dimensions: number) {
    this.dimensions = dimensions;
  }
}

export class Index {
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

  constructor(
    name: string,
    collection: string,
    type: IndexType,
    fields: string[],
    unique: boolean,
    partialExpr: string,
    options: Record<string, any>
  ) {
    this.name = name;
    this.collection = collection;
    this.type = type;
    this.fields = fields;
    this.unique = unique;
    this.partialExpr = partialExpr;
    this.options = options;

    switch (type) {
      case IndexType.BTree:
        this.btreeIndex = new BTreeIndex();
        break;
      case IndexType.GIN:
        this.ginIndex = new GINIndex();
        break;
      case IndexType.HNSW:
        const dim = options.dimensions || 768;
        this.hnswIndex = new HNSWIndex(dim);
        break;
    }
  }
}

export class IndexManager {
  private baseDir: string;
  private indexes: Map<string, Index> = new Map();

  constructor(baseDir: string) {
    this.baseDir = baseDir;
  }

  async createIndex(
    collection: string,
    name: string,
    indexType: IndexType,
    fields: string[],
    unique: boolean,
    partialExpr: string,
    options: Record<string, any>
  ): Promise<void> {
    const key = `${collection}:${name}`;

    if (this.indexes.has(key)) {
      throw new Error(`index ${key} already exists`);
    }

    const index = new Index(name, collection, indexType, fields, unique, partialExpr, options);

    this.indexes.set(key, index);

    // Save index metadata
    await this.saveIndexMetadata(index);
  }

  async dropIndex(collection: string, name: string): Promise<void> {
    const key = `${collection}:${name}`;

    if (!this.indexes.has(key)) {
      throw new Error(`index ${key} does not exist`);
    }

    this.indexes.delete(key);

    // Remove index files
    const indexDir = path.join(this.baseDir, collection, 'indexes', name);
    fs.rmSync(indexDir, { recursive: true, force: true });
  }

  getIndex(collection: string, name: string): Index | null {
    const key = `${collection}:${name}`;
    return this.indexes.get(key) || null;
  }

  getIndexesForCollection(collection: string): Index[] {
    const indexes: Index[] = [];
    for (const index of this.indexes.values()) {
      if (index.collection === collection) {
        indexes.push(index);
      }
    }
    return indexes;
  }

  async insert(collection: string, doc: Record<string, any>): Promise<void> {
    const indexes = this.getIndexesForCollection(collection);

    for (const idx of indexes) {
      if (!this.matchesPartial(idx, doc)) {
        continue;
      }

      const docID = doc.id as string;

      switch (idx.type) {
        case IndexType.BTree:
          this.insertBTree(idx, docID, doc);
          break;
        case IndexType.GIN:
          this.insertGIN(idx, docID, doc);
          break;
        case IndexType.HNSW:
          this.insertHNSW(idx, docID, doc);
          break;
      }
    }
  }

  async delete(collection: string, docID: string): Promise<void> {
    const indexes = this.getIndexesForCollection(collection);

    for (const idx of indexes) {
      switch (idx.type) {
        case IndexType.BTree:
          this.deleteBTree(idx, docID);
          break;
        case IndexType.GIN:
          this.deleteGIN(idx, docID);
          break;
        case IndexType.HNSW:
          this.deleteHNSW(idx, docID);
          break;
      }
    }
  }

  async queryIndex(collection: string, indexName: string, query: Record<string, any>): Promise<string[]> {
    const idx = this.getIndex(collection, indexName);
    if (!idx) {
      throw new Error(`index ${collection}:${indexName} not found`);
    }

    switch (idx.type) {
      case IndexType.BTree:
        return this.queryBTree(idx, query);
      case IndexType.GIN:
        return this.queryGIN(idx, query);
      case IndexType.HNSW:
        return this.queryHNSW(idx, query);
      default:
        throw new Error(`unsupported index type: ${idx.type}`);
    }
  }

  loadIndexes(): void {
    const collectionsDir = this.baseDir;

    try {
      const entries = fs.readdirSync(collectionsDir, { withFileTypes: true });

      for (const entry of entries) {
        if (!entry.isDirectory()) continue;

        const collection = entry.name;
        const indexesDir = path.join(collectionsDir, collection, 'indexes');

        try {
          const indexEntries = fs.readdirSync(indexesDir, { withFileTypes: true });

          for (const idxEntry of indexEntries) {
            if (!idxEntry.isDirectory()) continue;

            const indexName = idxEntry.name;
            const metaPath = path.join(indexesDir, indexName, 'metadata.json');

            try {
              const data = fs.readFileSync(metaPath, 'utf8');
              const idxData = JSON.parse(data);

              const idx = new Index(
                idxData.name,
                idxData.collection,
                idxData.type,
                idxData.fields,
                idxData.unique,
                idxData.partialExpr,
                idxData.options
              );

              const key = `${collection}:${indexName}`;
              this.indexes.set(key, idx);
            } catch {}
          }
        } catch {}
      }
    } catch {}
  }

  private matchesPartial(idx: Index, doc: Record<string, any>): boolean {
    if (!idx.partialExpr) return true;

    // Simple partial expression evaluation
    const parts = idx.partialExpr.split('=');
    if (parts.length !== 2) return true;

    const field = parts[0].trim();
    const expected = parts[1].trim();

    const val = doc[field];
    return val !== undefined && String(val) === expected;
  }

  private insertBTree(idx: Index, docID: string, doc: Record<string, any>): void {
    const key = this.buildCompositeKey(idx.fields, doc);

    if (idx.unique) {
      if (idx.btreeIndex!.data.get(key)?.length) {
        return;
      }
    }

    const existing = idx.btreeIndex!.data.get(key) || [];
    existing.push(docID);
    idx.btreeIndex!.data.set(key, existing);
  }

  private deleteBTree(idx: Index, docID: string): void {
    for (const [key, docIDs] of idx.btreeIndex!.data) {
      const index = docIDs.indexOf(docID);
      if (index !== -1) {
        docIDs.splice(index, 1);
        if (docIDs.length === 0) {
          idx.btreeIndex!.data.delete(key);
        } else {
          idx.btreeIndex!.data.set(key, docIDs);
        }
        break;
      }
    }
  }

  private queryBTree(idx: Index, query: Record<string, any>): string[] {
    if (query.value !== undefined) {
      const key = String(query.value);
      return idx.btreeIndex!.data.get(key) || [];
    }
    return [];
  }

  private insertGIN(idx: Index, docID: string, doc: Record<string, any>): void {
    const tokens = this.tokenizeJSON(doc);
    for (const token of tokens) {
      const existing = idx.ginIndex!.data.get(token) || [];
      existing.push(docID);
      idx.ginIndex!.data.set(token, existing);
    }
  }

  private deleteGIN(idx: Index, docID: string): void {
    for (const [token, docIDs] of idx.ginIndex!.data) {
      const index = docIDs.indexOf(docID);
      if (index !== -1) {
        docIDs.splice(index, 1);
        if (docIDs.length === 0) {
          idx.ginIndex!.data.delete(token);
        } else {
          idx.ginIndex!.data.set(token, docIDs);
        }
      }
    }
  }

  private queryGIN(idx: Index, query: Record<string, any>): string[] {
    const token = query.token as string;
    if (token) {
      return idx.ginIndex!.data.get(token) || [];
    }
    return [];
  }

  private insertHNSW(idx: Index, docID: string, doc: Record<string, any>): void {
    const payload = doc.payload as Record<string, any>;
    if (payload?.vector) {
      const vec = (payload.vector as any[]).map(v => Number(v));
      idx.hnswIndex!.vectors.set(docID, vec);
      idx.hnswIndex!.neighbors.set(docID, []);
    }
  }

  private deleteHNSW(idx: Index, docID: string): void {
    idx.hnswIndex!.vectors.delete(docID);
    idx.hnswIndex!.neighbors.delete(docID);
  }

  private queryHNSW(idx: Index, query: Record<string, any>): string[] {
    const queryVec = query.vector as number[];
    if (queryVec) {
      const results: { id: string; score: number }[] = [];
      for (const [docID, vec] of idx.hnswIndex!.vectors) {
        const score = this.cosineSimilarity(queryVec, vec);
        results.push({ id: docID, score });
      }

      results.sort((a, b) => b.score - a.score);

      const limit = query.limit || 10;
      return results.slice(0, limit).map(r => r.id);
    }
    return [];
  }

  private buildCompositeKey(fields: string[], doc: Record<string, any>): string {
    const parts: string[] = [];
    for (const field of fields) {
      const val = doc[field];
      if (val !== undefined) {
        parts.push(String(val));
      }
    }
    return parts.join('|');
  }

  private tokenizeJSON(doc: Record<string, any>): string[] {
    const tokens: string[] = [];
    this.tokenizeValue(doc, tokens);
    return tokens;
  }

  private tokenizeValue(val: any, tokens: string[]): void {
    if (typeof val === 'string') {
      const words = val.toLowerCase().split(/\s+/);
      tokens.push(...words);
    } else if (typeof val === 'object' && val !== null) {
      if (Array.isArray(val)) {
        for (const item of val) {
          this.tokenizeValue(item, tokens);
        }
      } else {
        for (const value of Object.values(val)) {
          this.tokenizeValue(value, tokens);
        }
      }
    }
  }

  private cosineSimilarity(a: number[], b: number[]): number {
    if (a.length !== b.length) return 0;
    let dotProduct = 0;
    let normA = 0;
    let normB = 0;
    for (let i = 0; i < a.length; i++) {
      dotProduct += a[i] * b[i];
      normA += a[i] * a[i];
      normB += b[i] * b[i];
    }
    if (normA === 0 || normB === 0) return 0;
    return dotProduct / (Math.sqrt(normA) * Math.sqrt(normB));
  }

  private async saveIndexMetadata(idx: Index): Promise<void> {
    const indexDir = path.join(this.baseDir, idx.collection, 'indexes', idx.name);
    fs.mkdirSync(indexDir, { recursive: true });

    const metaPath = path.join(indexDir, 'metadata.json');
    const data = JSON.stringify({
      name: idx.name,
      collection: idx.collection,
      type: idx.type,
      fields: idx.fields,
      unique: idx.unique,
      partialExpr: idx.partialExpr,
      options: idx.options,
    });
    fs.writeFileSync(metaPath, data, 'utf8');
  }
}