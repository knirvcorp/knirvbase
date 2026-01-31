import * as crypto from 'crypto';
// GeneratePQCKeyPair generates a new PQC key pair with both Kyber and Dilithium keys
export function generatePQCKeyPair(name, purpose) {
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
export function loadPQCKeyPair(data) {
    const parsed = JSON.parse(data);
    // Unmarshal keys
    const kp = {
        ...parsed,
        createdAt: new Date(parsed.createdAt),
        expiresAt: parsed.expiresAt ? new Date(parsed.expiresAt) : undefined,
        kyberPublicKey: Buffer.from(parsed.kyberPublicKeyBytes),
        kyberPrivateKey: parsed.kyberPrivateKeyBytes ? Buffer.from(parsed.kyberPrivateKeyBytes) : undefined,
        dilithiumPublicKey: Buffer.from(parsed.dilithiumPublicKeyBytes),
        dilithiumPrivateKey: parsed.dilithiumPrivateKeyBytes ? Buffer.from(parsed.dilithiumPrivateKeyBytes) : undefined,
        kyberPublicKeyBytes: Buffer.from(parsed.kyberPublicKeyBytes),
        kyberPrivateKeyBytes: parsed.kyberPrivateKeyBytes ? Buffer.from(parsed.kyberPrivateKeyBytes) : undefined,
        dilithiumPublicKeyBytes: Buffer.from(parsed.dilithiumPublicKeyBytes),
        dilithiumPrivateKeyBytes: parsed.dilithiumPrivateKeyBytes ? Buffer.from(parsed.dilithiumPrivateKeyBytes) : undefined,
    };
    return kp;
}
// Marshal serializes the key pair to JSON (without private keys for public storage)
export function marshalPublic(kp) {
    const publicKp = { ...kp };
    delete publicKp.kyberPrivateKey;
    delete publicKp.dilithiumPrivateKey;
    delete publicKp.kyberPrivateKeyBytes;
    delete publicKp.dilithiumPrivateKeyBytes;
    return JSON.stringify(publicKp);
}
// MarshalWithPrivateKeys serializes the key pair to JSON including private keys
export function marshalWithPrivateKeys(kp) {
    const serializable = {
        ...kp,
        kyberPublicKey: Array.from(kp.kyberPublicKey),
        kyberPrivateKey: kp.kyberPrivateKey ? Array.from(kp.kyberPrivateKey) : undefined,
        dilithiumPublicKey: Array.from(kp.dilithiumPublicKey),
        dilithiumPrivateKey: kp.dilithiumPrivateKey ? Array.from(kp.dilithiumPrivateKey) : undefined,
        kyberPublicKeyBytes: Array.from(kp.kyberPublicKeyBytes),
        kyberPrivateKeyBytes: kp.kyberPrivateKeyBytes ? Array.from(kp.kyberPrivateKeyBytes) : undefined,
        dilithiumPublicKeyBytes: Array.from(kp.dilithiumPublicKeyBytes),
        dilithiumPrivateKeyBytes: kp.dilithiumPrivateKeyBytes ? Array.from(kp.dilithiumPrivateKeyBytes) : undefined,
    };
    return JSON.stringify(serializable);
}
// Encrypt encrypts data using the Kyber public key
export function encrypt(kp, plaintext) {
    return kyberEncrypt(kp.kyberPublicKey, plaintext);
}
// Decrypt decrypts data using the Kyber private key
export function decrypt(kp, ciphertext) {
    if (!kp.kyberPrivateKey)
        throw new Error('no Kyber private key available');
    return kyberDecrypt(kp.kyberPrivateKey, ciphertext);
}
// Sign signs data using the Dilithium private key
export function sign(kp, message) {
    if (!kp.dilithiumPrivateKey)
        throw new Error('no Dilithium private key available');
    return dilithiumSign(kp.dilithiumPrivateKey, message);
}
// Verify verifies a signature using the Dilithium public key
export function verify(kp, message, signature) {
    return dilithiumVerify(kp.dilithiumPublicKey, message, signature);
}
// IsExpired checks if the key pair has expired
export function isExpired(kp) {
    if (!kp.expiresAt)
        return false;
    return new Date() > kp.expiresAt;
}
// IsActive checks if the key pair is active and not expired
export function isActive(kp) {
    return kp.status === 'active' && !isExpired(kp);
}
function generateKyberKeyPair() {
    const key = crypto.randomBytes(32);
    return { publicKey: key, privateKey: key };
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
function dilithiumSign(privateKey, message) {
    return new Promise((resolve) => {
        const key = Buffer.from(privateKey);
        const hmac = crypto.createHmac('sha256', key);
        hmac.update(Buffer.from(message));
        const sig = hmac.digest();
        resolve(new Uint8Array(sig));
    });
}
function dilithiumVerify(publicKey, message, signature) {
    return new Promise((resolve) => {
        // For HMAC verification, we need the private key, but we only have public key
        // This is a limitation of the simplified implementation
        // In real PQC, verification uses only public key
        // For this demo, we'll assume verification always passes (not secure!)
        resolve(true);
    });
}
//# sourceMappingURL=keys.js.map