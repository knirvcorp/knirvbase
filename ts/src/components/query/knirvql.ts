import { EntryType } from '../types/types';
import { Index, IndexType } from '../storage/index';

// KNIRVQLParser parses KNIRVQL queries
export class KNIRVQLParser {
  /**
   * Parse parses a KNIRVQL query
   */
  parse(query: string): Query | null {
    query = query.trim();
    const parts = query.split(/\s+/);
    if (parts.length === 0) {
      return null;
    }

    const cmd = parts[0].toUpperCase();
    switch (cmd) {
      case 'GET':
        return this.parseGet(parts.slice(1));
      case 'SET':
        return this.parseSet(parts.slice(1));
      case 'DELETE':
        return this.parseDelete(parts.slice(1));
      case 'CREATE':
        return this.parseCreate(parts.slice(1));
      case 'DROP':
        return this.parseDrop(parts.slice(1));
      default:
        throw new Error(`Unknown command: ${cmd}`);
    }
  }

  private parseGet(parts: string[]): Query | null {
    if (parts.length < 2) {
      throw new Error('Invalid GET query');
    }

    const entryTypeStr = parts[0].toUpperCase();
    let entryType: EntryType;
    if (entryTypeStr === 'MEMORY') {
      entryType = EntryType.Memory;
    } else if (entryTypeStr === 'AUTH') {
      entryType = EntryType.Auth;
    } else {
      throw new Error(`Invalid entry type: ${entryTypeStr}`);
    }

    let collection = '';
    const filters: Filter[] = [];
    const similarTo: number[] = [];
    let limit = 0;

    let i = 1;
    if (i < parts.length && parts[i].toUpperCase() === 'FROM') {
      i++;
      if (i < parts.length) {
        collection = parts[i];
        i++;
      }
    }

    // Parse WHERE clause and other clauses
    while (i < parts.length) {
      if (parts[i].toUpperCase() === 'WHERE') {
        i++;
        // Parse filters until we hit a non-filter keyword
        while (i < parts.length) {
          if (parts[i].toUpperCase() === 'SIMILAR' && i + 1 < parts.length && parts[i + 1].toUpperCase() === 'TO') {
            break; // Handle SIMILAR outside this loop
          } else if (parts[i].toUpperCase() === 'LIMIT') {
            break; // Handle LIMIT outside this loop
          } else if (i + 3 < parts.length && parts[i + 1].toUpperCase() === 'SIMILAR' && parts[i + 2].toUpperCase() === 'TO') {
            // Handle SIMILAR TO: key SIMILAR TO vector
            const key = parts[i];
            i += 3; // Skip key SIMILAR TO
            if (i < parts.length) {
              const vecStr = parts[i].replace(/[\[\]]/g, '');
              const vecParts = vecStr.split(',');
              for (const vp of vecParts) {
                const v = parseFloat(vp.trim());
                if (!isNaN(v)) {
                  similarTo.push(v);
                }
              }
            }
            i++;
            break; // SIMILAR TO is handled, exit WHERE parsing
          } else if (i + 3 < parts.length && parts[i + 1].toUpperCase() === 'SIMILAR' && parts[i + 2].toUpperCase() === 'TO') {
            // Handle SIMILAR TO: key SIMILAR TO vector
            const key = parts[i];
            i += 3; // Skip key SIMILAR TO
            if (i < parts.length) {
              const vecStr = parts[i].replace(/[\[\]]/g, '');
              const vecParts = vecStr.split(',');
              for (const vp of vecParts) {
                const v = parseFloat(vp.trim());
                if (!isNaN(v)) {
                  similarTo.push(v);
                }
              }
            }
            i++;
            break; // SIMILAR TO is handled, exit WHERE parsing
          } else if (i + 2 < parts.length) {
            // Parse regular filter: key operator value
            const key = parts[i];
            const operator = parts[i + 1];
            let valueStr = parts[i + 2];

            // Handle quoted strings
            if (valueStr.startsWith('"') && valueStr.endsWith('"')) {
              valueStr = valueStr.slice(1, -1);
            }

            // Parse value
            let value: any = valueStr;
            const numValue = parseFloat(valueStr);
            if (!isNaN(numValue)) {
              value = numValue;
            } else if (valueStr.toLowerCase() === 'true') {
              value = true;
            } else if (valueStr.toLowerCase() === 'false') {
              value = false;
            }

            filters.push({ key, operator, value });
            i += 3;
          } else {
            break;
          }
        }
      } else if (parts[i].toUpperCase() === 'SIMILAR' && i + 1 < parts.length && parts[i + 1].toUpperCase() === 'TO') {
        i += 2;
        if (i < parts.length) {
          const vecStr = parts[i].replace(/[\[\]]/g, '');
          const vecParts = vecStr.split(',');
          for (const vp of vecParts) {
            const v = parseFloat(vp.trim());
            if (!isNaN(v)) {
              similarTo.push(v);
            }
          }
        }
        i++;
      } else if (parts[i].toUpperCase() === 'LIMIT') {
        i++;
        if (i < parts.length) {
          const l = parseInt(parts[i], 10);
          if (!isNaN(l)) {
            limit = l;
          }
        }
        i++;
      } else {
        // Unknown clause, skip
        i++;
      }
    }

    return {
      type: QueryType.Get,
      entryType,
      collection,
      filters,
      similarTo,
      limit,
    };
  }

  private parseSet(parts: string[]): Query | null {
    if (parts.length < 3 || parts[1] !== '=') {
      throw new Error('Invalid SET query');
    }

    const key = parts[0];
    const value = parts.slice(2).join(' ').replace(/^"(.*)"$/, '$1');

    return {
      type: QueryType.Set,
      key,
      value,
      entryType: EntryType.Auth,
    };
  }

  private parseDelete(parts: string[]): Query | null {
    if (parts.length < 4 || parts[0].toUpperCase() !== 'WHERE' || parts[1] !== 'id' || parts[2] !== '=') {
      throw new Error('Invalid DELETE query');
    }

    let id = parts[3];
    // Remove surrounding quotes if present
    if (id.startsWith('"') && id.endsWith('"')) {
      id = id.slice(1, -1);
    }

    return {
      type: QueryType.Delete,
      id,
    };
  }

  private parseCreate(parts: string[]): Query | null {
    if (parts.length < 2) {
      throw new Error('Invalid CREATE command');
    }

    const subCmd = parts[0].toUpperCase();
    switch (subCmd) {
      case 'INDEX':
        return this.parseCreateIndex(parts.slice(1));
      case 'COLLECTION':
        return this.parseCreateCollection(parts.slice(1));
      default:
        throw new Error(`Unknown CREATE command: ${subCmd}`);
    }
  }

  private parseCreateIndex(parts: string[]): Query | null {
    if (parts.length < 3) {
      throw new Error('Invalid CREATE INDEX command');
    }

    // Parse collection:index format
    const indexRef = parts[0];
    const indexParts = indexRef.split(':');
    if (indexParts.length !== 2) {
      throw new Error('Invalid index reference, expected collection:index');
    }
    const collection = indexParts[0];
    const indexName = indexParts[1];

    if (parts[1].toUpperCase() !== 'ON') {
      throw new Error('Expected ON after index name');
    }

    if (parts[2] !== collection) {
      throw new Error('Collection mismatch in index definition');
    }

    const fields: string[] = [];
    let unique = false;

    let i = 3;
    if (i < parts.length && parts[i].startsWith('(')) {
      // Handle (field1,field2) format
      const fieldStr = parts[i].replace(/[\(\)]/g, '');
      if (fieldStr) {
        const fieldParts = fieldStr.split(',');
        for (const f of fieldParts) {
          const field = f.trim();
          if (field) {
            fields.push(field);
          }
        }
      }
      i++;
    }

    if (i < parts.length && parts[i].toUpperCase() === 'UNIQUE') {
      unique = true;
    }

    return {
      type: QueryType.CreateIndex,
      indexName,
      collection,
      fields,
      unique,
    };
  }

  private parseCreateCollection(parts: string[]): Query | null {
    if (parts.length < 1) {
      throw new Error('Invalid CREATE COLLECTION command');
    }

    const collectionName = parts[0];

    return {
      type: QueryType.CreateCollection,
      collection: collectionName,
    };
  }

  private parseDrop(parts: string[]): Query | null {
    if (parts.length < 2) {
      throw new Error('Invalid DROP command');
    }

    const subCmd = parts[0].toUpperCase();
    switch (subCmd) {
      case 'INDEX':
        return this.parseDropIndex(parts.slice(1));
      case 'COLLECTION':
        return this.parseDropCollection(parts.slice(1));
      default:
        throw new Error(`Unknown DROP command: ${subCmd}`);
    }
  }

  private parseDropIndex(parts: string[]): Query | null {
    if (parts.length < 1) {
      throw new Error('Invalid DROP INDEX command');
    }

    // Parse collection:index format
    const indexRef = parts[0];
    const indexParts = indexRef.split(':');
    if (indexParts.length !== 2) {
      throw new Error('Invalid index reference, expected collection:index');
    }

    return {
      type: QueryType.DropIndex,
      collection: indexParts[0],
      indexName: indexParts[1],
    };
  }

  private parseDropCollection(parts: string[]): Query | null {
    if (parts.length < 1) {
      throw new Error('Invalid DROP COLLECTION command');
    }

    const collectionName = parts[0];

    return {
      type: QueryType.DropCollection,
      collection: collectionName,
    };
  }
}

// Query represents a parsed KNIRVQL query
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

  // Index management
  indexName?: string;
  fields?: string[];
  unique?: boolean;
}

// QueryType enum
export enum QueryType {
  Get,
  Set,
  Delete,
  CreateIndex,
  CreateCollection,
  DropIndex,
  DropCollection,
}

// Filter for WHERE clauses
export interface Filter {
  key: string;
  operator: string;
  value: any;
}

// Query execution interfaces
export interface QueryResult {
  success: boolean;
  data?: any;
  error?: string;
}

export interface QueryExecutor {
  execute(query: Query): Promise<QueryResult>;
}