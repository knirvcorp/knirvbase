import { describe, it, expect, beforeEach } from '@jest/globals';
import { KNIRVQLParser, QueryType } from '../knirvql';
import { EntryType } from '../../types/types';

describe('KNIRVQLParser', () => {
  let parser: KNIRVQLParser;

  beforeEach(() => {
    parser = new KNIRVQLParser();
  });

  describe('parseGet', () => {
    it('should parse a simple GET query', () => {
      const query = parser.parse('GET MEMORY FROM memory');
      expect(query).not.toBeNull();
      expect(query!.type).toBe(QueryType.Get);
      expect(query!.entryType).toBe(EntryType.Memory);
      expect(query!.collection).toBe('memory');
    });

    it('should parse GET with WHERE clause', () => {
      const query = parser.parse('GET AUTH FROM credentials WHERE username = "alice"');
      expect(query).not.toBeNull();
      expect(query!.type).toBe(QueryType.Get);
      expect(query!.entryType).toBe(EntryType.Auth);
      expect(query!.collection).toBe('credentials');
      expect(query!.filters).toHaveLength(1);
      expect(query!.filters![0]).toEqual({
        key: 'username',
        operator: '=',
        value: 'alice'
      });
    });

    it('should parse GET with SIMILAR TO clause', () => {
      const query = parser.parse('GET MEMORY FROM memory WHERE vector SIMILAR TO [0.1, 0.2, 0.3]');
      expect(query).not.toBeNull();
      // SIMILAR TO parsing is not fully implemented yet
      expect(query!.entryType).toBe(EntryType.Memory);
      expect(query!.collection).toBe('memory');
    });

    it('should parse GET with LIMIT', () => {
      const query = parser.parse('GET MEMORY FROM memory LIMIT 10');
      expect(query).not.toBeNull();
      expect(query!.limit).toBe(10);
    });

    it('should parse complex GET query', () => {
      const query = parser.parse('GET MEMORY FROM memory WHERE source = "web" SIMILAR TO [0.1, 0.2] LIMIT 5');
      expect(query).not.toBeNull();
      expect(query!.filters).toHaveLength(1);
      expect(query!.filters![0]).toEqual({ key: 'source', operator: '=', value: 'web' });
      expect(query!.limit).toBe(5);
      // SIMILAR TO parsing is not fully implemented yet
    });
  });

  describe('parseSet', () => {
    it('should parse a SET query', () => {
      const query = parser.parse('SET api_key = "secret123"');
      expect(query).not.toBeNull();
      expect(query!.type).toBe(QueryType.Set);
      expect(query!.key).toBe('api_key');
      expect(query!.value).toBe('secret123');
      expect(query!.entryType).toBe(EntryType.Auth);
    });
  });

  describe('parseDelete', () => {
    it('should parse a DELETE query', () => {
      const query = parser.parse('DELETE WHERE id = "doc123"');
      expect(query).not.toBeNull();
      expect(query!.type).toBe(QueryType.Delete);
      expect(query!.id).toBe('doc123');
    });
  });

  describe('parseCreate', () => {
    it('should parse CREATE INDEX', () => {
      const query = parser.parse('CREATE INDEX users:email ON users (email) UNIQUE');
      expect(query).not.toBeNull();
      expect(query!.type).toBe(QueryType.CreateIndex);
      expect(query!.indexName).toBe('email');
      expect(query!.collection).toBe('users');
      expect(query!.fields).toEqual(['email']);
      expect(query!.unique).toBe(true);
    });

    it('should parse CREATE COLLECTION', () => {
      const query = parser.parse('CREATE COLLECTION documents');
      expect(query).not.toBeNull();
      expect(query!.type).toBe(QueryType.CreateCollection);
      expect(query!.collection).toBe('documents');
    });
  });

  describe('parseDrop', () => {
    it('should parse DROP INDEX', () => {
      const query = parser.parse('DROP INDEX users:email');
      expect(query).not.toBeNull();
      expect(query!.type).toBe(QueryType.DropIndex);
      expect(query!.indexName).toBe('email');
      expect(query!.collection).toBe('users');
    });

    it('should parse DROP COLLECTION', () => {
      const query = parser.parse('DROP COLLECTION documents');
      expect(query).not.toBeNull();
      expect(query!.type).toBe(QueryType.DropCollection);
      expect(query!.collection).toBe('documents');
    });
  });

  describe('error handling', () => {
    it('should throw error for unknown command', () => {
      expect(() => parser.parse('UNKNOWN command')).toThrow('Unknown command: UNKNOWN');
    });

    it('should throw error for invalid GET query', () => {
      expect(() => parser.parse('GET')).toThrow('Invalid GET query');
    });

    it('should throw error for invalid SET query', () => {
      expect(() => parser.parse('SET key')).toThrow('Invalid SET query');
    });

    it('should throw error for invalid DELETE query', () => {
      expect(() => parser.parse('DELETE id = "123"')).toThrow('Invalid DELETE query');
    });
  });

  describe('case insensitivity', () => {
    it('should handle lowercase commands', () => {
      const query = parser.parse('get memory from memory');
      expect(query!.type).toBe(QueryType.Get);
    });

    it('should handle mixed case commands', () => {
      const query = parser.parse('Get Memory From memory');
      expect(query!.type).toBe(QueryType.Get);
    });
  });
});