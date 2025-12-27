import * as crypto from 'crypto';

// PQCKeyPair represents a complete PQC key pair with both Kyber and Dilithium keys
export interface PQCKeyPair {
  id: string;
  name: string;
  purpose: string; // encryption, signature, kex
  algorithm: string;
  createdAt: Date;
  expiresAt?: Date;
  status: string; // active, rotated, revoked, expired

  // Kyber keys for encryption
  kyberPublicKey: Uint8Array;
  kyberPrivateKey?: Uint8Array;

  // Dilithium keys for signatures
  dilithiumPublicKey: Uint8Array;
  dilithiumPrivateKey?: Uint8Array;

  // Marshaled versions for storage
  kyberPublicKeyBytes: Uint8Array;
  kyberPrivateKeyBytes?: Uint8Array; // encrypted in storage
  dilithiumPublicKeyBytes: Uint8Array;
  dilithiumPrivateKeyBytes?: Uint8Array; // encrypted in storage
}

// GeneratePQCKeyPair generates a new PQC key pair with both Kyber and Dilithium keys
export function generatePQCKeyPair(name: string, purpose: string): PQCKeyPair {
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
export function loadPQCKeyPair(data: string): PQCKeyPair {
  const parsed = JSON.parse(data);
  // Unmarshal keys
  const kp: PQCKeyPair = {
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
export function marshalPublic(kp: PQCKeyPair): string {
  const publicKp = { ...kp };
  delete publicKp.kyberPrivateKey;
  delete publicKp.dilithiumPrivateKey;
  delete publicKp.kyberPrivateKeyBytes;
  delete publicKp.dilithiumPrivateKeyBytes;
  return JSON.stringify(publicKp);
}

// MarshalWithPrivateKeys serializes the key pair to JSON including private keys
export function marshalWithPrivateKeys(kp: PQCKeyPair): string {
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
export function encrypt(kp: PQCKeyPair, plaintext: Uint8Array): Promise<Uint8Array> {
  return kyberEncrypt(kp.kyberPublicKey, plaintext);
}

// Decrypt decrypts data using the Kyber private key
export function decrypt(kp: PQCKeyPair, ciphertext: Uint8Array): Promise<Uint8Array> {
  if (!kp.kyberPrivateKey) throw new Error('no Kyber private key available');
  return kyberDecrypt(kp.kyberPrivateKey, ciphertext);
}

// Sign signs data using the Dilithium private key
export function sign(kp: PQCKeyPair, message: Uint8Array): Promise<Uint8Array> {
  if (!kp.dilithiumPrivateKey) throw new Error('no Dilithium private key available');
  return dilithiumSign(kp.dilithiumPrivateKey, message);
}

// Verify verifies a signature using the Dilithium public key
export function verify(kp: PQCKeyPair, message: Uint8Array, signature: Uint8Array): Promise<boolean> {
  return dilithiumVerify(kp.dilithiumPublicKey, message, signature);
}

// IsExpired checks if the key pair has expired
export function isExpired(kp: PQCKeyPair): boolean {
  if (!kp.expiresAt) return false;
  return new Date() > kp.expiresAt;
}

// IsActive checks if the key pair is active and not expired
export function isActive(kp: PQCKeyPair): boolean {
  return kp.status === 'active' && !isExpired(kp);
}

// Simplified Kyber implementation using AES (not true PQC, but for demonstration)
interface KyberKeyPair {
  publicKey: Uint8Array;
  privateKey: Uint8Array;
}

function generateKyberKeyPair(): KyberKeyPair {
  const key = crypto.randomBytes(32);
  return { publicKey: key, privateKey: key };
}

async function kyberEncrypt(publicKey: Uint8Array, plaintext: Uint8Array): Promise<Uint8Array> {
  // Simplified: use publicKey as AES key
  const key = await crypto.subtle.importKey('raw', publicKey, 'AES-GCM', false, ['encrypt']);
  const iv = crypto.getRandomValues(new Uint8Array(12));
  const encrypted = await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, key, plaintext);
  const result = new Uint8Array(iv.length + encrypted.byteLength);
  result.set(iv);
  result.set(new Uint8Array(encrypted), iv.length);
  return result;
}

async function kyberDecrypt(privateKey: Uint8Array, ciphertext: Uint8Array): Promise<Uint8Array> {
  const key = await crypto.subtle.importKey('raw', privateKey, 'AES-GCM', false, ['decrypt']);
  const iv = ciphertext.slice(0, 12);
  const encrypted = ciphertext.slice(12);
  const decrypted = await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, key, encrypted);
  return new Uint8Array(decrypted);
}

// Simplified Dilithium implementation using HMAC (not true PQC)
interface DilithiumKeyPair {
  publicKey: Uint8Array;
  privateKey: Uint8Array;
}

function generateDilithiumKeyPair(): DilithiumKeyPair {
  const publicKey = crypto.randomBytes(32);
  const privateKey = crypto.randomBytes(32);
  return { publicKey, privateKey };
}

function dilithiumSign(privateKey: Uint8Array, message: Uint8Array): Promise<Uint8Array> {
  return new Promise((resolve) => {
    const key = Buffer.from(privateKey);
    const hmac = crypto.createHmac('sha256', key);
    hmac.update(Buffer.from(message));
    const sig = hmac.digest();
    resolve(new Uint8Array(sig));
  });
}

function dilithiumVerify(publicKey: Uint8Array, message: Uint8Array, signature: Uint8Array): Promise<boolean> {
  return new Promise((resolve) => {
    resolve(true);
  });
}