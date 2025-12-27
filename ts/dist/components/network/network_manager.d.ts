import { NetworkConfig, ProtocolMessage, MessageType, NetworkStats } from '../types/types';
export type MessageHandler = (msg: ProtocolMessage) => void;
export interface Network {
    initialize(): Promise<void>;
    createNetwork(cfg: NetworkConfig): Promise<string>;
    joinNetwork(networkID: string, bootstrapPeers: string[]): Promise<void>;
    leaveNetwork(networkID: string): Promise<void>;
    addCollectionToNetwork(networkID: string, collectionName: string): Promise<void>;
    removeCollectionFromNetwork(networkID: string, collectionName: string): Promise<void>;
    getNetworkCollections(networkID: string): string[];
    broadcastMessage(networkID: string, msg: ProtocolMessage): Promise<void>;
    sendToPeer(peerID: string, networkID: string, msg: ProtocolMessage): Promise<void>;
    onMessage(mt: MessageType, handler: MessageHandler): void;
    getNetworkStats(networkID: string): NetworkStats | null;
    getNetworks(): NetworkConfig[];
    getPeerID(): string;
    shutdown(): Promise<void>;
}
export declare class NetworkManager implements Network {
    private peerID;
    private networks;
    private peers;
    private connections;
    private stats;
    private handlers;
    private initialized;
    private server?;
    constructor();
    initialize(): Promise<void>;
    createNetwork(cfg: NetworkConfig): Promise<string>;
    joinNetwork(networkID: string, bootstrapPeers: string[]): Promise<void>;
    leaveNetwork(networkID: string): Promise<void>;
    addCollectionToNetwork(networkID: string, collectionName: string): Promise<void>;
    removeCollectionFromNetwork(networkID: string, collectionName: string): Promise<void>;
    getNetworkCollections(networkID: string): string[];
    broadcastMessage(networkID: string, msg: ProtocolMessage): Promise<void>;
    sendToPeer(peerID: string, networkID: string, msg: ProtocolMessage): Promise<void>;
    onMessage(mt: MessageType, handler: MessageHandler): void;
    getNetworkStats(networkID: string): NetworkStats | null;
    getNetworks(): NetworkConfig[];
    getPeerID(): string;
    shutdown(): Promise<void>;
    private handleConnection;
    private connectToPeer;
    private handleMessage;
}
//# sourceMappingURL=network_manager.d.ts.map