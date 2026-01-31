// EntryType specifies the kind of data stored.
export var EntryType;
(function (EntryType) {
    EntryType["Memory"] = "MEMORY";
    EntryType["Auth"] = "AUTH";
})(EntryType || (EntryType = {}));
// OperationType enumerates CRDT operation kinds
export var OperationType;
(function (OperationType) {
    OperationType[OperationType["Insert"] = 0] = "Insert";
    OperationType[OperationType["Update"] = 1] = "Update";
    OperationType[OperationType["Delete"] = 2] = "Delete";
})(OperationType || (OperationType = {}));
// MessageType strings for protocol
export var MessageType;
(function (MessageType) {
    MessageType["SyncRequest"] = "sync_request";
    MessageType["SyncResponse"] = "sync_response";
    MessageType["Operation"] = "operation";
    MessageType["Heartbeat"] = "heartbeat";
    MessageType["CollectionAnnounce"] = "collection_announce";
    MessageType["CollectionRequest"] = "collection_request";
})(MessageType || (MessageType = {}));
//# sourceMappingURL=types.js.map