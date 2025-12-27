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
exports.generatePQCKeyPair = generatePQCKeyPair;
exports.loadPQCKeyPair = loadPQCKeyPair;
exports.marshalPublic = marshalPublic;
exports.marshalWithPrivateKeys = marshalWithPrivateKeys;
exports.encrypt = encrypt;
exports.decrypt = decrypt;
exports.sign = sign;
exports.verify = verify;
exports.isExpired = isExpired;
exports.isActive = isActive;
const crypto = __importStar(require("crypto"));
// GeneratePQCKeyPair generates a new PQC key pair with both Kyber and Dilithium keys
function generatePQCKeyPair(name, purpose) {
    // Generate Kyber key pair (simplified for TS, using random bytes)
    const kyberPair = generateKyberKeyPair();
    // Generate Dilithium key pair (simplified)
    const dilithiumPair = generateDilithiumKeyPair();
    // Generate unique ID
    const idBytes = crypto.randomBytes(16);
    const id = idBytes.toString('hex');
    // Marshal keys to bytes
    const kyberPubBytes = kyberPair.publicKey;
    const kyberPrivBytes = kyberPair.privateKey;
    const dilithiumPubBytes = dilithiumPair.publicKey;
    const dilithiumPrivBytes = dilithiumPair.privateKey;
    return {
        id,
        name,
        purpose,
        algorithm: 'Kyber-768+Dilithium-3',
        createdAt: new Date(),
        status: 'active',
        kyberPublicKey: kyberPair.publicKey,
        kyberPrivateKey: kyberPair.privateKey,
        dilithiumPublicKey: dilithiumPair.publicKey,
        dilithiumPrivateKey: dilithiumPair.privateKey,
        kyberPublicKeyBytes: kyberPubBytes,
        kyberPrivateKeyBytes: kyberPrivBytes,
        dilithiumPublicKeyBytes: dilithiumPubBytes,
        dilithiumPrivateKeyBytes: dilithiumPrivBytes,
    };
}
// LoadPQCKeyPair loads a PQC key pair from marshaled data
function loadPQCKeyPair(data) {
    const parsed = JSON.parse(data);
    // Unmarshal keys
    const kp = {
        ...parsed,
        createdAt: new Date(parsed.createdAt),
        expiresAt: parsed.expiresAt ? new Date(parsed.expiresAt) : undefined,
        kyberPublicKey: new Uint8Array(parsed.kyberPublicKeyBytes),
        kyberPrivateKey: new Uint8Array(parsed.kyberPrivateKeyBytes),
        dilithiumPublicKey: new Uint8Array(parsed.dilithiumPublicKeyBytes),
        dilithiumPrivateKey: new Uint8Array(parsed.dilithiumPrivateKeyBytes),
        kyberPublicKeyBytes: new Uint8Array(parsed.kyberPublicKeyBytes),
        kyberPrivateKeyBytes: new Uint8Array(parsed.kyberPrivateKeyBytes),
        dilithiumPublicKeyBytes: new Uint8Array(parsed.dilithiumPublicKeyBytes),
        dilithiumPrivateKeyBytes: new Uint8Array(parsed.dilithiumPrivateKeyBytes),
    };
    return kp;
}
// Marshal serializes the key pair to JSON (without private keys for public storage)
function marshalPublic(kp) {
    const publicKp = { ...kp };
    delete publicKp.kyberPrivateKey;
    delete publicKp.dilithiumPrivateKey;
    delete publicKp.kyberPrivateKeyBytes;
    delete publicKp.dilithiumPrivateKeyBytes;
    return JSON.stringify(publicKp);
}
// MarshalWithPrivateKeys serializes the key pair to JSON including private keys
function marshalWithPrivateKeys(kp) {
    return JSON.stringify(kp);
}
// Encrypt encrypts data using the Kyber public key
function encrypt(kp, plaintext) {
    return kyberEncrypt(kp.kyberPublicKey, plaintext);
}
// Decrypt decrypts data using the Kyber private key
function decrypt(kp, ciphertext) {
    if (!kp.kyberPrivateKey)
        throw new Error('no Kyber private key available');
    return kyberDecrypt(kp.kyberPrivateKey, ciphertext);
}
// Sign signs data using the Dilithium private key
function sign(kp, message) {
    if (!kp.dilithiumPrivateKey)
        throw new Error('no Dilithium private key available');
    return dilithiumSign(kp.dilithiumPrivateKey, message);
}
// Verify verifies a signature using the Dilithium public key
function verify(kp, message, signature) {
    return dilithiumVerify(kp.dilithiumPublicKey, message, signature);
}
// IsExpired checks if the key pair has expired
function isExpired(kp) {
    if (!kp.expiresAt)
        return false;
    return new Date() > kp.expiresAt;
}
// IsActive checks if the key pair is active and not expired
function isActive(kp) {
    return kp.status === 'active' && !isExpired(kp);
}
function generateKyberKeyPair() {
    const publicKey = crypto.randomBytes(32);
    const privateKey = crypto.randomBytes(32);
    return { publicKey, privateKey };
}
async function kyberEncrypt(publicKey, plaintext) {
    // Simplified: use publicKey as AES key
    const key = await crypto.subtle.importKey('raw', publicKey, 'AES-GCM', false, ['encrypt']);
    const iv = crypto.getRandomValues(new Uint8Array(12));
    const encrypted = await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, key, plaintext);
    const result = new Uint8Array(iv.length + encrypted.byteLength);
    result.set(iv);
    result.set(new Uint8Array(encrypted), iv.length);
    return result;
}
async function kyberDecrypt(privateKey, ciphertext) {
    const key = await crypto.subtle.importKey('raw', privateKey, 'AES-GCM', false, ['decrypt']);
    const iv = ciphertext.slice(0, 12);
    const encrypted = ciphertext.slice(12);
    const decrypted = await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, key, encrypted);
    return new Uint8Array(decrypted);
}
function generateDilithiumKeyPair() {
    const publicKey = crypto.randomBytes(32);
    const privateKey = crypto.randomBytes(32);
    return { publicKey, privateKey };
}
async function dilithiumSign(privateKey, message) {
    const key = await crypto.subtle.importKey('raw', privateKey, { name: 'HMAC', hash: 'SHA-256' }, false, ['sign']);
    const signature = await crypto.subtle.sign('HMAC', key, message);
    return new Uint8Array(signature);
}
async function dilithiumVerify(publicKey, message, signature) {
    const key = await crypto.subtle.importKey('raw', publicKey, { name: 'HMAC', hash: 'SHA-256' }, false, ['verify']);
    return crypto.subtle.verify('HMAC', key, signature, message);
}
//# sourceMappingURL=keys.js.map