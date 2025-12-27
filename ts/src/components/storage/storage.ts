import * as fs from 'fs';
import * as path from 'path';
import { IndexManager, Index, IndexType } from './index';
import { EncryptionManager } from '../crypto/pqc';
import { EntryType } from '../types/types';

export interface Storage {
  insert(collection: string, doc: Record<string, any>): Promise<void>;
  update(collection: string, id: string, update: Record<string, any>): Promise<number>;
  delete(collection: string, id: string): Promise<number>;
  find(collection: string, id: string): Promise<Record<string, any> | null>;
  findAll(collection: string): Promise<Record<string, any>[]>;

  // Index management
  createIndex(collection: string, name: string, indexType: IndexType, fields: string[], unique: boolean, partialExpr: string, options: Record<string, any>): Promise<void>;
  dropIndex(collection: string, name: string): Promise<void>;
  getIndex(collection: string, name: string): Index | null;
  getIndexesForCollection(collection: string): Index[];
  queryIndex(collection: string, indexName: string, query: Record<string, any>): Promise<string[]>;
}

export class FileStorage implements Storage {
  private baseDir: string;
  private indexManager: IndexManager;
  private encryptionMgr: EncryptionManager;

  constructor(baseDir: string) {
    this.baseDir = baseDir;
    fs.mkdirSync(baseDir, { recursive: true });
    this.indexManager = new IndexManager(baseDir);
    this.indexManager.loadIndexes();
    this.encryptionMgr = new EncryptionManager();
  }

  private getCollectionDir(collection: string): string {
    return path.join(this.baseDir, collection);
  }

  private getDocPath(collection: string, id: string): string {
    return path.join(this.getCollectionDir(collection), `${id}.json`);
  }

  setMasterKey(keyPair: any): void { // PQCKeyPair
    this.encryptionMgr.setMasterKey(keyPair);
  }

  private isEncryptedCollection(collection: string): boolean {
    const sensitiveCollections = [
      'credentials',
      'pqc_keys',
      'sessions',
      'audit_log',
      'threat_events',
      'access_control',
    ];
    return sensitiveCollections.includes(collection);
  }

  async insert(collection: string, doc: Record<string, any>): Promise<void> {
    fs.mkdirSync(this.getCollectionDir(collection), { recursive: true });
    const docPath = this.getDocPath(collection, doc.id);

    const docCopy = this.deepCopyDoc(doc);

    // Handle MEMORY blob
    if (docCopy.entryType === EntryType.Memory) {
      const payload = docCopy.payload;
      if (payload && payload.blob !== undefined) {
        const blobPath = this.saveBlob(collection, docCopy.id, payload.blob);
        payload.blobRef = blobPath;
        delete payload.blob;
      }
    }

    // Encrypt sensitive collections
    if (this.isEncryptedCollection(collection) && this.encryptionMgr.getMasterKey()) {
      const encryptedDoc = await this.encryptDocument(collection, docCopy);
      Object.assign(docCopy, encryptedDoc);
    }

    const data = JSON.stringify(docCopy);
    fs.writeFileSync(docPath, data, 'utf8');

    // Update indexes
    await this.indexManager.insert(collection, doc);
  }

  async update(collection: string, id: string, update: Record<string, any>): Promise<number> {
    const doc = await this.find(collection, id);
    if (!doc) {
      throw new Error('not found');
    }

    Object.assign(doc, update);
    await this.insert(collection, doc);
    return 1;
  }

  async delete(collection: string, id: string): Promise<number> {
    const docPath = this.getDocPath(collection, id);
    try {
      fs.unlinkSync(docPath);
    } catch (err: any) {
      if (err.code !== 'ENOENT') throw err;
    }

    // Remove blob
    const blobDir = path.join(this.getCollectionDir(collection), 'blobs');
    const blobPath = path.join(blobDir, id);
    try {
      fs.unlinkSync(blobPath);
    } catch {}

    // Remove from indexes
    await this.indexManager.delete(collection, id);
    return 1;
  }

  async find(collection: string, id: string): Promise<Record<string, any> | null> {
    const docPath = this.getDocPath(collection, id);
    try {
      const data = fs.readFileSync(docPath, 'utf8');
      const doc: Record<string, any> = JSON.parse(data);

      // Decrypt if needed
      if (doc.encrypted && this.encryptionMgr.getMasterKey()) {
        const decrypted = await this.decryptDocument(doc);
        Object.assign(doc, decrypted);
      }

      // Load blob
      if (doc.entryType === EntryType.Memory) {
        const payload = doc.payload;
        if (payload && payload.blobRef) {
          const blob = this.loadBlob(payload.blobRef);
          if (blob !== null) {
            payload.blob = blob;
            delete payload.blobRef;
          }
        }
      }

      return doc;
    } catch (err: any) {
      if (err.code === 'ENOENT') return null;
      throw err;
    }
  }

  async findAll(collection: string): Promise<Record<string, any>[]> {
    const dir = this.getCollectionDir(collection);
    try {
      const files = fs.readdirSync(dir);
      const docs: Record<string, any>[] = [];
      for (const file of files) {
        if (path.extname(file) === '.json') {
          const id = path.basename(file, '.json');
          const doc = await this.find(collection, id);
          if (doc) docs.push(doc);
        }
      }
      return docs;
    } catch (err: any) {
      if (err.code === 'ENOENT') return [];
      throw err;
    }
  }

  private saveBlob(collection: string, id: string, blob: any): string {
    const blobDir = path.join(this.getCollectionDir(collection), 'blobs');
    fs.mkdirSync(blobDir, { recursive: true });
    const blobPath = path.join(blobDir, id);
    const data = JSON.stringify(blob);
    fs.writeFileSync(blobPath, data, 'utf8');
    return blobPath;
  }

  private loadBlob(blobRef: string): any {
    try {
      const data = fs.readFileSync(blobRef, 'utf8');
      return JSON.parse(data);
    } catch {
      return null;
    }
  }

  async createIndex(collection: string, name: string, indexType: IndexType, fields: string[], unique: boolean, partialExpr: string, options: Record<string, any>): Promise<void> {
    return this.indexManager.createIndex(collection, name, indexType, fields, unique, partialExpr, options);
  }

  async dropIndex(collection: string, name: string): Promise<void> {
    return this.indexManager.dropIndex(collection, name);
  }

  getIndex(collection: string, name: string): Index | null {
    return this.indexManager.getIndex(collection, name);
  }

  getIndexesForCollection(collection: string): Index[] {
    return this.indexManager.getIndexesForCollection(collection);
  }

  async queryIndex(collection: string, indexName: string, query: Record<string, any>): Promise<string[]> {
    return this.indexManager.queryIndex(collection, indexName, query);
  }

  private deepCopyDoc(doc: Record<string, any>): Record<string, any> {
    return JSON.parse(JSON.stringify(doc));
  }

  private async encryptDocument(collection: string, doc: Record<string, any>): Promise<Record<string, any>> {
    const masterKey = this.encryptionMgr.getMasterKey();
    if (!masterKey) throw new Error('no master key set');

    if (doc.payload) {
      const encryptedPayload = await this.encryptPayload(collection, doc.payload, masterKey.id);
      doc.payload = encryptedPayload;
      doc.encrypted = true;
      doc.encryption_key_id = masterKey.id;
    }
    return doc;
  }

  private async encryptPayload(collection: string, payload: Record<string, any>, keyID: string): Promise<Record<string, any>> {
    const encrypted: Record<string, any> = {};
    for (const [key, value] of Object.entries(payload)) {
      if (this.isSensitiveField(collection, key)) {
        const valueBytes = JSON.stringify(value);
        const encryptedValue = await this.encryptionMgr.encryptData(Buffer.from(valueBytes), keyID);
        encrypted[key] = encryptedValue;
        encrypted[key + '_encrypted'] = true;
      } else {
        encrypted[key] = value;
      }
    }
    return encrypted;
  }

  private isSensitiveField(collection: string, fieldName: string): boolean {
    const sensitiveFields: Record<string, string[]> = {
      credentials: ['hash', 'salt'],
      pqc_keys: ['kyber_private_key', 'dilithium_private_key'],
      sessions: ['token_hash'],
      audit_log: ['details'],
      threat_events: ['indicators'],
      access_control: ['permissions'],
    };
    return (sensitiveFields[collection] || []).includes(fieldName);
  }

  private async decryptDocument(doc: Record<string, any>): Promise<Record<string, any>> {
    const keyID = doc.encryption_key_id;
    if (!keyID) throw new Error('missing encryption_key_id');

    if (doc.payload) {
      const decryptedPayload = await this.decryptPayload(doc.payload, keyID);
      doc.payload = decryptedPayload;
    }
    delete doc.encrypted;
    delete doc.encryption_key_id;
    return doc;
  }

  private async decryptPayload(payload: Record<string, any>, keyID: string): Promise<Record<string, any>> {
    const decrypted: Record<string, any> = {};
    for (const [key, value] of Object.entries(payload)) {
      if (key.endsWith('_encrypted')) continue;
      if (payload[key + '_encrypted']) {
        const encryptedValue = value as string;
        const decryptedBytes = await this.encryptionMgr.decryptData(encryptedValue);
        decrypted[key] = JSON.parse(decryptedBytes.toString());
      } else {
        decrypted[key] = value;
      }
    }
    return decrypted;
  }
}