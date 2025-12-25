package types

import (
	"time"

	"github.com/knirv/knirvbase/internal/clock"
)

// EntryType specifies the kind of data stored.
type EntryType string

const (
	EntryTypeMemory EntryType = "MEMORY"
	EntryTypeAuth   EntryType = "AUTH"
)

// DistributedDocument augments a document with CRDT metadata
type DistributedDocument struct {
	ID        string                 `json:"id"`
	EntryType EntryType              `json:"entryType"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	Vector    clock.VectorClock      `json:"_vector"`
	Timestamp int64                  `json:"_timestamp"`
	PeerID    string                 `json:"_peerId"`
	// Stage is an optional marker used to indicate special handling for a document.
	// Supported values: "post-pending" (document is staged and will be posted as a KNIRVGRAPH
	// transaction during the next sync), or empty string for normal documents.
	Stage   string `json:"_stage,omitempty"`
	Deleted bool   `json:"_deleted,omitempty"`
}

// OperationType enumerates CRDT operation kinds
type OperationType int

const (
	OpInsert OperationType = iota
	OpUpdate
	OpDelete
)

// CRDTOperation represents a change to be synchronized
type CRDTOperation struct {
	ID         string               `json:"id"`
	Type       OperationType        `json:"type"`
	Collection string               `json:"collection"`
	DocumentID string               `json:"documentId"`
	Data       *DistributedDocument `json:"data,omitempty"`
	Vector     clock.VectorClock    `json:"vector"`
	Timestamp  int64                `json:"timestamp"`
	PeerID     string               `json:"peerId"`
}

// NetworkConfig holds network-level configuration
// Additional fields control posting/staging behavior:
//   - DefaultPostingNetwork: the network to which staged entries are posted (e.g. "knirvgraph").
//   - AutoPostClassifications: a list of EntryTypes that are automatically staged for posting by classification.
//   - PrivateByDefault: when true (default), entries are private unless staged or explicitly configured.
type NetworkConfig struct {
	NetworkID      string
	Name           string
	Collections    map[string]bool
	BootstrapPeers []string
	// Default posting target for staged entries (e.g., "knirvgraph").
	DefaultPostingNetwork string

	// Entry classifications which are auto-staged for posting. Common defaults include
	// EntryType values like "ERROR", "CONTEXT", and "IDEA".
	AutoPostClassifications []EntryType

	// Entries are private by default unless staged or configured otherwise.
	PrivateByDefault bool

	Encryption struct {
		Enabled      bool
		SharedSecret string
	}
	Replication struct {
		Factor   int
		Strategy string // full | partial | leader
	}
	Discovery struct {
		MDNS      bool
		Bootstrap bool
	}
}

// PeerInfo
type PeerInfo struct {
	PeerID      string
	Addrs       []string
	Protocols   []string
	Latency     time.Duration
	LastSeen    time.Time
	Collections []string
}

// SyncState for a collection/network
type SyncState struct {
	Collection        string
	NetworkID         string
	LocalVector       clock.VectorClock
	LastSync          time.Time
	PendingOperations []CRDTOperation
	// StagedEntries contains IDs of documents marked with `_stage == "post-pending"`.
	// These will be converted to KNIRVGRAPH transactions and posted during the next sync.
	StagedEntries  []string
	SyncInProgress bool
}

// NetworkStats
type NetworkStats struct {
	NetworkID          string
	ConnectedPeers     int
	TotalPeers         int
	CollectionsShared  int
	OperationsSent     int64
	OperationsReceived int64
	BytesTransferred   int64
	AverageLatency     time.Duration
}

// MessageType strings for protocol
type MessageType string

const (
	MsgSyncRequest        MessageType = "sync_request"
	MsgSyncResponse       MessageType = "sync_response"
	MsgOperation          MessageType = "operation"
	MsgHeartbeat          MessageType = "heartbeat"
	MsgCollectionAnnounce MessageType = "collection_announce"
	MsgCollectionRequest  MessageType = "collection_request"
)

// ProtocolMessage generic envelope
type ProtocolMessage struct {
	Type      MessageType `json:"type"`
	NetworkID string      `json:"networkId"`
	SenderID  string      `json:"senderId"`
	Timestamp int64       `json:"timestamp"`
	Payload   interface{} `json:"payload"`
}
