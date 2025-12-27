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
exports.NetworkManager = void 0;
const net = __importStar(require("net"));
const crypto = __importStar(require("crypto"));
const types_1 = require("../types/types");
class NetworkManager {
    constructor() {
        this.networks = new Map();
        this.peers = new Map();
        this.connections = new Map();
        this.stats = new Map();
        this.handlers = new Map();
        this.initialized = false;
        this.peerID = crypto.randomBytes(16).toString('hex');
    }
    async initialize() {
        if (this.initialized)
            return;
        return new Promise((resolve, reject) => {
            this.server = net.createServer((socket) => {
                this.handleConnection(socket);
            });
            this.server.listen(0, () => {
                this.initialized = true;
                console.log(`Network manager initialized: ${this.peerID}`);
                resolve();
            });
            this.server.on('error', reject);
        });
    }
    async createNetwork(cfg) {
        await this.initialize();
        if (this.networks.has(cfg.networkId)) {
            return cfg.networkId;
        }
        cfg.collections = {};
        this.networks.set(cfg.networkId, cfg);
        this.stats.set(cfg.networkId, {
            networkId: cfg.networkId,
            connectedPeers: 0,
            totalPeers: 0,
            collectionsShared: 0,
            operationsSent: 0,
            operationsReceived: 0,
            bytesTransferred: 0,
            averageLatency: 0
        });
        console.log(`Created network ${cfg.networkId}`);
        return cfg.networkId;
    }
    async joinNetwork(networkID, bootstrapPeers) {
        await this.initialize();
        if (!this.networks.has(networkID)) {
            const cfg = {
                networkId: networkID,
                name: `Network ${networkID}`,
                collections: {},
                bootstrapPeers: [],
                defaultPostingNetwork: '',
                autoPostClassifications: [],
                privateByDefault: true,
                encryption: { enabled: false, sharedSecret: '' },
                replication: { factor: 1, strategy: 'full' },
                discovery: { mdns: false, bootstrap: false }
            };
            this.networks.set(networkID, cfg);
            this.stats.set(networkID, {
                networkId: networkID,
                connectedPeers: 0,
                totalPeers: 0,
                collectionsShared: 0,
                operationsSent: 0,
                operationsReceived: 0,
                bytesTransferred: 0,
                averageLatency: 0
            });
        }
        // Connect to bootstrap peers
        for (const addr of bootstrapPeers) {
            this.connectToPeer(addr);
        }
    }
    async leaveNetwork(networkID) {
        this.networks.delete(networkID);
        this.stats.delete(networkID);
        console.log(`Left network ${networkID}`);
    }
    async addCollectionToNetwork(networkID, collectionName) {
        const netCfg = this.networks.get(networkID);
        if (!netCfg)
            throw new Error('network not found');
        netCfg.collections[collectionName] = true;
        const st = this.stats.get(networkID);
        if (st) {
            st.collectionsShared = Object.keys(netCfg.collections).length;
        }
        // Announce collection
        await this.broadcastMessage(networkID, {
            type: types_1.MessageType.CollectionAnnounce,
            networkId: networkID,
            senderId: this.getPeerID(),
            timestamp: Date.now(),
            payload: { collection: collectionName }
        });
    }
    async removeCollectionFromNetwork(networkID, collectionName) {
        const netCfg = this.networks.get(networkID);
        if (!netCfg)
            return;
        delete netCfg.collections[collectionName];
        const st = this.stats.get(networkID);
        if (st) {
            st.collectionsShared = Object.keys(netCfg.collections).length;
        }
    }
    getNetworkCollections(networkID) {
        const netCfg = this.networks.get(networkID);
        return netCfg ? Object.keys(netCfg.collections) : [];
    }
    async broadcastMessage(networkID, msg) {
        if (!this.initialized)
            throw new Error('not initialized');
        const data = JSON.stringify(msg);
        const conns = Array.from(this.connections.values());
        const st = this.stats.get(networkID);
        for (const conn of conns) {
            conn.write(data + '\n');
            if (st) {
                st.operationsSent++;
                st.bytesTransferred += data.length;
            }
        }
    }
    async sendToPeer(peerID, networkID, msg) {
        if (!this.initialized)
            throw new Error('not initialized');
        const conn = this.connections.get(peerID);
        if (!conn)
            throw new Error('peer not connected');
        const data = JSON.stringify(msg);
        conn.write(data + '\n');
        const st = this.stats.get(networkID);
        if (st) {
            st.operationsSent++;
            st.bytesTransferred += data.length;
        }
    }
    onMessage(mt, handler) {
        const handlers = this.handlers.get(mt) || [];
        handlers.push(handler);
        this.handlers.set(mt, handlers);
    }
    getNetworkStats(networkID) {
        const st = this.stats.get(networkID);
        if (st) {
            st.connectedPeers = this.connections.size;
        }
        return st || null;
    }
    getNetworks() {
        return Array.from(this.networks.values());
    }
    getPeerID() {
        return this.peerID;
    }
    async shutdown() {
        if (this.server) {
            this.server.close();
        }
        for (const conn of this.connections.values()) {
            conn.destroy();
        }
        this.connections.clear();
        this.initialized = false;
    }
    handleConnection(socket) {
        let peerID = '';
        const scanner = new net.Socket();
        // Simple handshake
        socket.on('data', (data) => {
            const lines = data.toString().split('\n');
            for (const line of lines) {
                if (line.trim() === '')
                    continue;
                if (line.startsWith('KNIRV:')) {
                    peerID = line.split(':')[1];
                    socket.write(`KNIRV:${this.peerID}\n`);
                    this.connections.set(peerID, socket);
                    this.peers.set(peerID, {
                        peerId: peerID,
                        addrs: [socket.remoteAddress],
                        protocols: [],
                        latency: 0,
                        lastSeen: new Date(),
                        collections: []
                    });
                }
                else {
                    try {
                        const msg = JSON.parse(line);
                        this.handleMessage(msg);
                    }
                    catch { }
                }
            }
        });
        socket.on('close', () => {
            if (peerID) {
                this.connections.delete(peerID);
            }
        });
    }
    connectToPeer(address) {
        const [host, port] = address.split(':');
        const socket = net.createConnection({ host, port: parseInt(port) }, () => {
            socket.write(`KNIRV:${this.peerID}\n`);
        });
        socket.on('data', (data) => {
            const lines = data.toString().split('\n');
            for (const line of lines) {
                if (line.startsWith('KNIRV:')) {
                    const peerID = line.split(':')[1];
                    this.connections.set(peerID, socket);
                    this.peers.set(peerID, {
                        peerId: peerID,
                        addrs: [address],
                        protocols: [],
                        latency: 0,
                        lastSeen: new Date(),
                        collections: []
                    });
                }
                else {
                    try {
                        const msg = JSON.parse(line);
                        this.handleMessage(msg);
                    }
                    catch { }
                }
            }
        });
    }
    handleMessage(msg) {
        const handlers = this.handlers.get(msg.type) || [];
        for (const h of handlers) {
            h(msg);
        }
    }
}
exports.NetworkManager = NetworkManager;
//# sourceMappingURL=network_manager.js.map