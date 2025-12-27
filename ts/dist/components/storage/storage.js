"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
Object.defineProperty(exports, "__esModule", { value: true });
exports.FileStorage = void 0;
const fs = __importStar(require("fs"));
const path = __importStar(require("path"));
const index_1 = require("./index");
const pqc_1 = require("../crypto/pqc");
const types_1 = require("../types/types");
class FileStorage {
    constructor(baseDir) {
        this.baseDir = baseDir;
        fs.mkdirSync(baseDir, { recursive: true });
        this.indexManager = new index_1.IndexManager(baseDir);
        this.indexManager.loadIndexes();
        this.encryptionMgr = new pqc_1.EncryptionManager();
    }
    getCollectionDir(collection) {
        return path.join(this.baseDir, collection);
    }
    getDocPath(collection, id) {
        return path.join(this.getCollectionDir(collection), `${id}.json`);
    }
    setMasterKey(keyPair) {
        this.encryptionMgr.setMasterKey(keyPair);
    }
    isEncryptedCollection(collection) {
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
    async insert(collection, doc) {
        fs.mkdirSync(this.getCollectionDir(collection), { recursive: true });
        const docPath = this.getDocPath(collection, doc.id);
        const docCopy = this.deepCopyDoc(doc);
        // Handle MEMORY blob
        if (docCopy.entryType === types_1.EntryType.Memory) {
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
    async update(collection, id, update) {
        const doc = await this.find(collection, id);
        if (!doc) {
            throw new Error('not found');
        }
        Object.assign(doc, update);
        await this.insert(collection, doc);
        return 1;
    }
    async delete(collection, id) {
        const docPath = this.getDocPath(collection, id);
        try {
            fs.unlinkSync(docPath);
        }
        catch (err) {
            if (err.code !== 'ENOENT')
                throw err;
        }
        // Remove blob
        const blobDir = path.join(this.getCollectionDir(collection), 'blobs');
        const blobPath = path.join(blobDir, id);
        try {
            fs.unlinkSync(blobPath);
        }
        catch { }
        // Remove from indexes
        await this.indexManager.delete(collection, id);
        return 1;
    }
    async find(collection, id) {
        const docPath = this.getDocPath(collection, id);
        try {
            const data = fs.readFileSync(docPath, 'utf8');
            const doc = JSON.parse(data);
            // Decrypt if needed
            if (doc.encrypted && this.encryptionMgr.getMasterKey()) {
                const decrypted = await this.decryptDocument(doc);
                Object.assign(doc, decrypted);
            }
            // Load blob
            if (doc.entryType === types_1.EntryType.Memory) {
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
        }
        catch (err) {
            if (err.code === 'ENOENT')
                return null;
            throw err;
        }
    }
    async findAll(collection) {
        const dir = this.getCollectionDir(collection);
        try {
            const files = fs.readdirSync(dir);
            const docs = [];
            for (const file of files) {
                if (path.extname(file) === '.json') {
                    const id = path.basename(file, '.json');
                    const doc = await this.find(collection, id);
                    if (doc)
                        docs.push(doc);
                }
            }
            return docs;
        }
        catch (err) {
            if (err.code === 'ENOENT')
                return [];
            throw err;
        }
    }
    saveBlob(collection, id, blob) {
        const blobDir = path.join(this.getCollectionDir(collection), 'blobs');
        fs.mkdirSync(blobDir, { recursive: true });
        const blobPath = path.join(blobDir, id);
        const data = JSON.stringify(blob);
        fs.writeFileSync(blobPath, data, 'utf8');
        return blobPath;
    }
    loadBlob(blobRef) {
        try {
            const data = fs.readFileSync(blobRef, 'utf8');
            return JSON.parse(data);
        }
        catch {
            return null;
        }
    }
    async createIndex(collection, name, indexType, fields, unique, partialExpr, options) {
        return this.indexManager.createIndex(collection, name, indexType, fields, unique, partialExpr, options);
    }
    async dropIndex(collection, name) {
        return this.indexManager.dropIndex(collection, name);
    }
    getIndex(collection, name) {
        return this.indexManager.getIndex(collection, name);
    }
    getIndexesForCollection(collection) {
        return this.indexManager.getIndexesForCollection(collection);
    }
    async queryIndex(collection, indexName, query) {
        return this.indexManager.queryIndex(collection, indexName, query);
    }
    deepCopyDoc(doc) {
        return JSON.parse(JSON.stringify(doc));
    }
    async encryptDocument(collection, doc) {
        const masterKey = this.encryptionMgr.getMasterKey();
        if (!masterKey)
            throw new Error('no master key set');
        if (doc.payload) {
            const encryptedPayload = await this.encryptPayload(collection, doc.payload, masterKey.id);
            doc.payload = encryptedPayload;
            doc.encrypted = true;
            doc.encryption_key_id = masterKey.id;
        }
        return doc;
    }
    async encryptPayload(collection, payload, keyID) {
        const encrypted = {};
        for (const [key, value] of Object.entries(payload)) {
            if (this.isSensitiveField(collection, key)) {
                const valueBytes = JSON.stringify(value);
                const encryptedValue = await this.encryptionMgr.encryptData(Buffer.from(valueBytes), keyID);
                encrypted[key] = encryptedValue;
                encrypted[key + '_encrypted'] = true;
            }
            else {
                encrypted[key] = value;
            }
        }
        return encrypted;
    }
    isSensitiveField(collection, fieldName) {
        const sensitiveFields = {
            credentials: ['hash', 'salt'],
            pqc_keys: ['kyber_private_key', 'dilithium_private_key'],
            sessions: ['token_hash'],
            audit_log: ['details'],
            threat_events: ['indicators'],
            access_control: ['permissions'],
        };
        return (sensitiveFields[collection] || []).includes(fieldName);
    }
    async decryptDocument(doc) {
        const keyID = doc.encryption_key_id;
        if (!keyID)
            throw new Error('missing encryption_key_id');
        if (doc.payload) {
            const decryptedPayload = await this.decryptPayload(doc.payload, keyID);
            doc.payload = decryptedPayload;
        }
        delete doc.encrypted;
        delete doc.encryption_key_id;
        return doc;
    }
    async decryptPayload(payload, keyID) {
        const decrypted = {};
        for (const [key, value] of Object.entries(payload)) {
            if (key.endsWith('_encrypted'))
                continue;
            if (payload[key + '_encrypted']) {
                const encryptedValue = value;
                const decryptedBytes = await this.encryptionMgr.decryptData(encryptedValue);
                decrypted[key] = JSON.parse(decryptedBytes.toString());
            }
            else {
                decrypted[key] = value;
            }
        }
        return decrypted;
    }
}
exports.FileStorage = FileStorage;
//# sourceMappingURL=storage.js.map