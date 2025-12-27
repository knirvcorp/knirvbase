import { describe, it, expect, beforeEach } from '@jest/globals';
import { generatePQCKeyPair, loadPQCKeyPair, marshalPublic, marshalWithPrivateKeys, encrypt, decrypt, sign, verify, isActive, isExpired, PQCKeyPair } from '../keys';
import { EncryptionManager } from '../encryption';

describe('PQC Key Management', () => {
  let keyPair: PQCKeyPair;

  beforeEach(() => {
    keyPair = generatePQCKeyPair('test-key', 'encryption');
  });

  describe('generatePQCKeyPair', () => {
    it('should generate a valid PQC key pair', () => {
      expect(keyPair).toBeDefined();
      expect(keyPair.id).toBeDefined();
      expect(keyPair.name).toBe('test-key');
      expect(keyPair.purpose).toBe('encryption');
      expect(keyPair.algorithm).toBe('Kyber-768+Dilithium-3');
      expect(keyPair.status).toBe('active');
      expect(keyPair.createdAt).toBeInstanceOf(Date);

      // Check Kyber keys
      expect(keyPair.kyberPublicKey).toBeDefined();
      expect(keyPair.kyberPrivateKey).toBeDefined();
      expect(keyPair.kyberPublicKey.length).toBeGreaterThan(0);
      expect(keyPair.kyberPrivateKey!.length).toBeGreaterThan(0);

      // Check Dilithium keys
      expect(keyPair.dilithiumPublicKey).toBeDefined();
      expect(keyPair.dilithiumPrivateKey).toBeDefined();
      expect(keyPair.dilithiumPublicKey.length).toBeGreaterThan(0);
      expect(keyPair.dilithiumPrivateKey!.length).toBeGreaterThan(0);
    });

    it('should generate unique IDs for different keys', () => {
      const keyPair2 = generatePQCKeyPair('test-key-2', 'encryption');
      expect(keyPair.id).not.toBe(keyPair2.id);
    });
  });

  describe('loadPQCKeyPair', () => {
    it('should load a key pair from marshaled data', () => {
      const marshaled = marshalWithPrivateKeys(keyPair);
      const loaded = loadPQCKeyPair(marshaled);

      expect(loaded.id).toBe(keyPair.id);
      expect(loaded.name).toBe(keyPair.name);
      expect(loaded.purpose).toBe(keyPair.purpose);
      expect(loaded.kyberPublicKey).toEqual(keyPair.kyberPublicKey);
      expect(loaded.kyberPrivateKey).toEqual(keyPair.kyberPrivateKey);
      expect(loaded.dilithiumPublicKey).toEqual(keyPair.dilithiumPublicKey);
      expect(loaded.dilithiumPrivateKey).toEqual(keyPair.dilithiumPrivateKey);
    });
  });

  describe('marshalPublic', () => {
    it('should marshal only public keys', () => {
      const marshaled = marshalPublic(keyPair);
      const parsed = JSON.parse(marshaled);

      expect(parsed.id).toBe(keyPair.id);
      expect(parsed.name).toBe(keyPair.name);
      expect(parsed.kyberPublicKey).toBeDefined();
      expect(parsed.dilithiumPublicKey).toBeDefined();

      // Private keys should not be present
      expect(parsed.kyberPrivateKey).toBeUndefined();
      expect(parsed.dilithiumPrivateKey).toBeUndefined();
    });
  });

  describe('isActive and isExpired', () => {
    it('should return true for active non-expired keys', () => {
      expect(isActive(keyPair)).toBe(true);
      expect(isExpired(keyPair)).toBe(false);
    });

    it('should detect expired keys', () => {
      const expiredKey = { ...keyPair, expiresAt: new Date(Date.now() - 1000) };
      expect(isExpired(expiredKey)).toBe(true);
      expect(isActive(expiredKey)).toBe(false);
    });
  });
});

describe('PQC Encryption/Decryption', () => {
  let keyPair: PQCKeyPair;
  const testData = new Uint8Array([1, 2, 3, 4, 5]);

  beforeEach(() => {
    keyPair = generatePQCKeyPair('encryption-test', 'encryption');
  });

  describe('encrypt/decrypt', () => {
    it('should encrypt and decrypt data correctly', async () => {
      const encrypted = await encrypt(keyPair, testData);
      expect(encrypted).toBeDefined();
      expect(encrypted.length).toBeGreaterThan(testData.length);

      const decrypted = await decrypt(keyPair, encrypted);
      expect(decrypted).toEqual(testData);
    });

    it('should fail decryption with wrong private key', async () => {
      const otherKeyPair = generatePQCKeyPair('other-key', 'encryption');
      const encrypted = await encrypt(keyPair, testData);

      await expect(decrypt(otherKeyPair, encrypted)).rejects.toThrow();
    });
  });
});

describe('PQC Signatures', () => {
  let keyPair: PQCKeyPair;
  const testMessage = new Uint8Array([72, 101, 108, 108, 111]); // "Hello"

  beforeEach(() => {
    keyPair = generatePQCKeyPair('signature-test', 'signature');
  });

  describe('sign/verify', () => {
    it('should sign and verify messages correctly', async () => {
      const signature = await sign(keyPair, testMessage);
      expect(signature).toBeDefined();
      expect(signature.length).toBeGreaterThan(0);

      const isValid = await verify(keyPair, testMessage, signature);
      expect(isValid).toBe(true);
    });

    it('should reject tampered messages', async () => {
      const signature = await sign(keyPair, testMessage);
      const tamperedMessage = new Uint8Array([72, 101, 108, 108, 111, 33]); // "Hello!"

      const isValid = await verify(keyPair, tamperedMessage, signature);
      expect(isValid).toBe(false);
    });

    it('should reject invalid signatures', async () => {
      const fakeSignature = new Uint8Array([1, 2, 3, 4, 5]);
      const isValid = await verify(keyPair, testMessage, fakeSignature);
      expect(isValid).toBe(false);
    });
  });
});

describe('EncryptionManager', () => {
  let manager: EncryptionManager;
  let keyPair: PQCKeyPair;

  beforeEach(() => {
    manager = new EncryptionManager();
    keyPair = generatePQCKeyPair('manager-test', 'encryption');
    manager.setMasterKey(keyPair);
  });

  describe('encryptData/decryptData', () => {
    it('should encrypt and decrypt data with metadata', async () => {
      const plaintext = Buffer.from('sensitive data');
      const encrypted = await manager.encryptData(plaintext, keyPair.id);

      expect(encrypted).toBeDefined();
      expect(typeof encrypted).toBe('string');

      const decrypted = await manager.decryptData(encrypted);
      expect(Buffer.from(decrypted).toString()).toBe('sensitive data');
    });

    it('should fail decryption with wrong key', async () => {
      const plaintext = Buffer.from('secret');
      const encrypted = await manager.encryptData(plaintext, keyPair.id);

      // Remove the key from cache
      manager.removeKey(keyPair.id);

      await expect(manager.decryptData(encrypted)).rejects.toThrow('key not found in cache');
    });
  });

  describe('key management', () => {
    it('should cache and retrieve keys', () => {
      const newKey = manager.generateDataEncryptionKey('cached-key');
      expect(newKey).toBeDefined();

      // The key should be cached internally
      expect(manager.getMasterKey()).toBe(keyPair);
    });

    it('should handle multiple keys', async () => {
      const key1 = manager.generateDataEncryptionKey('key1');
      const key2 = manager.generateDataEncryptionKey('key2');

      const data1 = Buffer.from('data for key1');
      const data2 = Buffer.from('data for key2');

      const encrypted1 = await manager.encryptData(data1, key1.id);
      const encrypted2 = await manager.encryptData(data2, key2.id);

      const decrypted1 = await manager.decryptData(encrypted1);
      const decrypted2 = await manager.decryptData(encrypted2);

      expect(Buffer.from(decrypted1).toString()).toBe('data for key1');
      expect(Buffer.from(decrypted2).toString()).toBe('data for key2');
    });
  });

  describe('error handling', () => {
    it('should fail encryption without master key', async () => {
      const managerWithoutKey = new EncryptionManager();
      const data = Buffer.from('test');

      await expect(managerWithoutKey.encryptData(data, 'nonexistent')).rejects.toThrow();
    });

    it('should fail with inactive keys', async () => {
      const inactiveKey = generatePQCKeyPair('inactive', 'encryption');
      inactiveKey.status = 'revoked';

      manager.cacheKey(inactiveKey.id, inactiveKey);
      const data = Buffer.from('test');

      await expect(manager.encryptData(data, inactiveKey.id)).rejects.toThrow('key is not active');
    });
  });
});

describe('PQC Integration Tests', () => {
  it('should perform full encryption workflow', async () => {
    const manager = new EncryptionManager();
    const keyPair = generatePQCKeyPair('integration-test', 'encryption');
    manager.setMasterKey(keyPair);

    // Encrypt data
    const originalData = Buffer.from('This is confidential information');
    const encrypted = await manager.encryptData(originalData, keyPair.id);

    // Decrypt data
    const decrypted = await manager.decryptData(encrypted);
    const decryptedString = Buffer.from(decrypted).toString();

    expect(decryptedString).toBe('This is confidential information');
  });

  it('should perform full signature workflow', async () => {
    const keyPair = generatePQCKeyPair('signature-integration', 'signature');

    const message = Buffer.from('Important document content');
    const signature = await sign(keyPair, message);

    const isValid = await verify(keyPair, message, signature);
    expect(isValid).toBe(true);

    // Test with encryption manager
    const manager = new EncryptionManager();
    manager.setMasterKey(keyPair);

    const data = Buffer.from('signed and encrypted data');
    const encrypted = await manager.encryptData(data, keyPair.id);
    const decrypted = await manager.decryptData(encrypted);

    expect(Buffer.from(decrypted).toString()).toBe('signed and encrypted data');
  });
});