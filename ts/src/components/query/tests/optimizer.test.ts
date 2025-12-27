import { describe, it, expect, beforeEach } from '@jest/globals';
import { QueryOptimizer, QueryPlan, ScanType } from '../optimizer';
import { Query, QueryType, Filter } from '../knirvql';
import { EntryType } from '../../types/types';
import { Index, IndexType } from '../../storage/index';

describe('QueryOptimizer', () => {
  let optimizer: QueryOptimizer;
  let mockIndexes: Index[];

  beforeEach(() => {
    mockIndexes = [
      {
        name: 'username',
        collection: 'users',
        type: IndexType.BTree,
        fields: ['username'],
        unique: true,
        partialExpr: '',
        options: {},
      } as Index,
      {
        name: 'email',
        collection: 'users',
        type: IndexType.BTree,
        fields: ['email'],
        unique: false,
        partialExpr: '',
        options: {},
      } as Index,
    ];

    optimizer = new QueryOptimizer('users', mockIndexes);
  });

  describe('optimize', () => {
    it('should return full scan plan when no filters', () => {
      const query: Query = {
        type: QueryType.Get,
        entryType: EntryType.Memory,
        collection: 'users',
        filters: [],
      };

      const plan = optimizer.optimize(query);

      expect(plan.useIndex).toBe(false);
      expect(plan.scanType).toBe(ScanType.FullScan);
      expect(plan.estimatedCost).toBeGreaterThan(0);
    });

    it('should choose best index for single filter', () => {
      const query: Query = {
        type: QueryType.Get,
        entryType: EntryType.Memory,
        collection: 'users',
        filters: [{ key: 'username', operator: '=', value: 'alice' }],
      };

      const plan = optimizer.optimize(query);

      expect(plan.useIndex).toBe(true);
      expect(plan.indexName).toBe('username');
      expect(plan.scanType).toBe(ScanType.IndexOnlyScan);
      expect(plan.indexFilters).toHaveLength(1);
      expect(plan.postFilters).toHaveLength(0);
    });

    it('should handle filters that cannot use index', () => {
      const query: Query = {
        type: QueryType.Get,
        entryType: EntryType.Memory,
        collection: 'users',
        filters: [{ key: 'age', operator: '>', value: 18 }],
      };

      const plan = optimizer.optimize(query);

      expect(plan.useIndex).toBe(false);
      expect(plan.scanType).toBe(ScanType.FullScan);
      expect(plan.postFilters).toHaveLength(1);
    });

    it('should prefer unique index over non-unique', () => {
      // Add a non-unique index on username
      const nonUniqueIndex: Index = {
        name: 'username_nonunique',
        collection: 'users',
        type: IndexType.BTree,
        fields: ['username'],
        unique: false,
        partialExpr: '',
        options: {},
      } as Index;

      const opt = new QueryOptimizer('users', [mockIndexes[0], nonUniqueIndex]);

      const query: Query = {
        type: QueryType.Get,
        entryType: EntryType.Memory,
        collection: 'users',
        filters: [{ key: 'username', operator: '=', value: 'alice' }],
      };

      const plan = opt.optimize(query);

      // Should choose the unique index (first one)
      expect(plan.useIndex).toBe(true);
      expect(plan.indexName).toBe('username');
    });

    it('should handle multiple filters with partial index usage', () => {
      const query: Query = {
        type: QueryType.Get,
        entryType: EntryType.Memory,
        collection: 'users',
        filters: [
          { key: 'username', operator: '=', value: 'alice' },
          { key: 'age', operator: '>', value: 18 },
        ],
      };

      const plan = optimizer.optimize(query);

      expect(plan.useIndex).toBe(true);
      expect(plan.indexName).toBe('username');
      expect(plan.scanType).toBe(ScanType.IndexScan);
      expect(plan.indexFilters).toHaveLength(1);
      expect(plan.postFilters).toHaveLength(1);
    });
  });

  describe('cost estimation', () => {
    it('should estimate lower cost for index scan vs full scan', () => {
      const query: Query = {
        type: QueryType.Get,
        entryType: EntryType.Memory,
        collection: 'users',
        filters: [{ key: 'username', operator: '=', value: 'alice' }],
      };

      const plan = optimizer.optimize(query);

      // Create a full scan plan manually for comparison
      const fullScanQuery: Query = {
        type: QueryType.Get,
        entryType: EntryType.Memory,
        collection: 'users',
        filters: [{ key: 'nonindexed', operator: '=', value: 'value' }],
      };

      const fullScanPlan = optimizer.optimize(fullScanQuery);

      expect(plan.estimatedCost).toBeLessThan(fullScanPlan.estimatedCost);
    });

    it('should consider limit in cost estimation', () => {
      const queryWithLimit: Query = {
        type: QueryType.Get,
        entryType: EntryType.Memory,
        collection: 'users',
        filters: [],
        limit: 10,
      };

      const plan = optimizer.optimize(queryWithLimit);

      expect(plan.limit).toBe(10);
      expect(plan.estimatedRows).toBeLessThanOrEqual(10);
    });
  });

  describe('index selection', () => {
    it('should select B-Tree index for exact match', () => {
      const filter: Filter = { key: 'username', operator: '=', value: 'alice' };
      const index = mockIndexes[0]; // B-Tree index on username

      const opt = new QueryOptimizer('users', [index]);
      const query: Query = {
        type: QueryType.Get,
        entryType: EntryType.Memory,
        collection: 'users',
        filters: [filter],
      };

      const plan = opt.optimize(query);

      expect(plan.useIndex).toBe(true);
      expect(plan.indexName).toBe('username');
    });

    it('should not select HNSW index for regular filters', () => {
      const hnswIndex: Index = {
        name: 'vector',
        collection: 'memory',
        type: IndexType.HNSW,
        fields: ['vector'],
        unique: false,
        partialExpr: '',
        options: { dimensions: 768 },
      } as Index;

      const opt = new QueryOptimizer('memory', [hnswIndex]);
      const query: Query = {
        type: QueryType.Get,
        entryType: EntryType.Memory,
        collection: 'memory',
        filters: [{ key: 'source', operator: '=', value: 'web' }],
      };

      const plan = opt.optimize(query);

      expect(plan.useIndex).toBe(false);
    });
  });

  describe('statistics', () => {
    it('should update and retrieve statistics', () => {
      const newStats = new Map([
        ['username', {
          cardinality: 500,
          selectivity: 0.002,
          avgBucketSize: 1.0,
        }],
      ]);

      optimizer.updateStats(10000, newStats);

      const stats = optimizer.getStats();
      expect(stats.totalDocuments).toBe(10000);
      expect(stats.indexStats.get('username')?.cardinality).toBe(500);
    });
  });
});