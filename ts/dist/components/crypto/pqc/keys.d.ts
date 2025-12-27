export interface PQCKeyPair {
    id: string;
    name: string;
    purpose: string;
    algorithm: string;
    createdAt: Date;
    expiresAt?: Date;
    status: string;
    kyberPublicKey: Uint8Array;
    kyberPrivateKey?: Uint8Array;
    dilithiumPublicKey: Uint8Array;
    dilithiumPrivateKey?: Uint8Array;
    kyberPublicKeyBytes: Uint8Array;
    kyberPrivateKeyBytes?: Uint8Array;
    dilithiumPublicKeyBytes: Uint8Array;
    dilithiumPrivateKeyBytes?: Uint8Array;
}
export declare function generatePQCKeyPair(name: string, purpose: string): PQCKeyPair;
export declare function loadPQCKeyPair(data: string): PQCKeyPair;
export declare function marshalPublic(kp: PQCKeyPair): string;
export declare function marshalWithPrivateKeys(kp: PQCKeyPair): string;
export declare function encrypt(kp: PQCKeyPair, plaintext: Uint8Array): Promise<Uint8Array>;
export declare function decrypt(kp: PQCKeyPair, ciphertext: Uint8Array): Promise<Uint8Array>;
export declare function sign(kp: PQCKeyPair, message: Uint8Array): Promise<Uint8Array>;
export declare function verify(kp: PQCKeyPair, message: Uint8Array, signature: Uint8Array): Promise<boolean>;
export declare function isExpired(kp: PQCKeyPair): boolean;
export declare function isActive(kp: PQCKeyPair): boolean;
//# sourceMappingURL=keys.d.ts.map