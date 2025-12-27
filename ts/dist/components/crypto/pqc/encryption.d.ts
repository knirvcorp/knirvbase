import { PQCKeyPair } from './keys';
export declare class EncryptionManager {
    private masterKey?;
    private keyCache;
    setMasterKey(keyPair: PQCKeyPair): void;
    getMasterKey(): PQCKeyPair | undefined;
    encryptData(plaintext: Uint8Array, keyID: string): Promise<string>;
    decryptData(encryptedData: string): Promise<Uint8Array>;
    cacheKey(keyID: string, keyPair: PQCKeyPair): void;
    removeKey(keyID: string): void;
    generateDataEncryptionKey(name: string): PQCKeyPair;
}
//# sourceMappingURL=encryption.d.ts.map