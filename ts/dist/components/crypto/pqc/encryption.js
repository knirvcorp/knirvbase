"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.EncryptionManager = void 0;
const keys_1 = require("./keys");
// EncryptionManager manages PQC encryption for sensitive data
class EncryptionManager {
    constructor() {
        this.keyCache = new Map();
    }
    // SetMasterKey sets the master PQC key pair for encryption
    setMasterKey(keyPair) {
        this.masterKey = keyPair;
    }
    // GetMasterKey returns the master key pair
    getMasterKey() {
        return this.masterKey;
    }
    // EncryptData encrypts sensitive data using PQC encryption
    async encryptData(plaintext, keyID) {
        let keyPair = this.keyCache.get(keyID);
        if (!keyPair) {
            // If key not in cache, try to use master key
            if (this.masterKey && this.masterKey.id === keyID) {
                keyPair = this.masterKey;
            }
            else {
                throw new Error(`key ${keyID} not found in cache`);
            }
        }
        if (!(0, keys_1.isActive)(keyPair)) {
            throw new Error(`key ${keyID} is not active`);
        }
        // Encrypt the data
        const ciphertext = await (0, keys_1.encrypt)(keyPair, plaintext);
        // Create encrypted payload with metadata
        const payload = {
            key_id: keyID,
            algorithm: 'Kyber-768+AES-256-GCM',
            ciphertext: Buffer.from(ciphertext).toString('base64'),
        };
        // Sign the payload for integrity
        const payloadBytes = Buffer.from(JSON.stringify(payload));
        const signature = await (0, keys_1.sign)(keyPair, payloadBytes);
        // Create final encrypted structure
        const encrypted = {
            payload,
            signature: Buffer.from(signature).toString('base64'),
        };
        const finalBytes = Buffer.from(JSON.stringify(encrypted));
        return finalBytes.toString('base64');
    }
    // DecryptData decrypts data encrypted with EncryptData
    async decryptData(encryptedData) {
        // Decode the base64 encrypted data
        const encryptedBytes = Buffer.from(encryptedData, 'base64');
        // Unmarshal the encrypted structure
        const encrypted = JSON.parse(encryptedBytes.toString());
        const payload = encrypted.payload;
        const signatureB64 = encrypted.signature;
        const signature = Buffer.from(signatureB64, 'base64');
        // Extract payload
        const payloadBytes = Buffer.from(JSON.stringify(payload));
        const keyID = payload.key_id;
        const ciphertextB64 = payload.ciphertext;
        const ciphertext = Buffer.from(ciphertextB64, 'base64');
        // Get the key pair
        let keyPair = this.keyCache.get(keyID);
        if (!keyPair) {
            // If key not in cache, try to use master key
            if (this.masterKey && this.masterKey.id === keyID) {
                keyPair = this.masterKey;
            }
            else {
                throw new Error(`key ${keyID} not found in cache`);
            }
        }
        if (!(0, keys_1.isActive)(keyPair)) {
            throw new Error(`key ${keyID} is not active`);
        }
        // Verify signature
        const isValid = await (0, keys_1.verify)(keyPair, payloadBytes, signature);
        if (!isValid) {
            throw new Error('signature verification failed');
        }
        // Decrypt the data
        return (0, keys_1.decrypt)(keyPair, ciphertext);
    }
    // CacheKey adds a key pair to the cache
    cacheKey(keyID, keyPair) {
        this.keyCache.set(keyID, keyPair);
    }
    // RemoveKey removes a key pair from the cache
    removeKey(keyID) {
        this.keyCache.delete(keyID);
    }
    // GenerateDataEncryptionKey generates a new key pair for data encryption
    generateDataEncryptionKey(name) {
        const keyPair = (0, keys_1.generatePQCKeyPair)(name, 'encryption');
        this.cacheKey(keyPair.id, keyPair);
        return keyPair;
    }
}
exports.EncryptionManager = EncryptionManager;
//# sourceMappingURL=encryption.js.map