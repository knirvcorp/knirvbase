"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.MessageType = exports.OperationType = exports.EntryType = void 0;
// EntryType specifies the kind of data stored.
var EntryType;
(function (EntryType) {
    EntryType["Memory"] = "MEMORY";
    EntryType["Auth"] = "AUTH";
})(EntryType || (exports.EntryType = EntryType = {}));
// OperationType enumerates CRDT operation kinds
var OperationType;
(function (OperationType) {
    OperationType[OperationType["Insert"] = 0] = "Insert";
    OperationType[OperationType["Update"] = 1] = "Update";
    OperationType[OperationType["Delete"] = 2] = "Delete";
})(OperationType || (exports.OperationType = OperationType = {}));
// MessageType strings for protocol
var MessageType;
(function (MessageType) {
    MessageType["SyncRequest"] = "sync_request";
    MessageType["SyncResponse"] = "sync_response";
    MessageType["Operation"] = "operation";
    MessageType["Heartbeat"] = "heartbeat";
    MessageType["CollectionAnnounce"] = "collection_announce";
    MessageType["CollectionRequest"] = "collection_request";
})(MessageType || (exports.MessageType = MessageType = {}));
//# sourceMappingURL=types.js.map