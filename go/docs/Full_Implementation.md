# Full Implementation Plan — KNIRVBASE (Go)

This document outlines a concrete, actionable plan to move the KNIRVBASE prototype to a fully implemented, production-ready distributed database. It covers architecture, separation of CLI vs. library, networking, storage/backends, CRDT/sync improvements, testing & CI, security, observability, packaging, and a phased milestone plan with acceptance criteria.

---

## 1) Goals & Scope ✅

- Provide a robust, testable, and reusable distributed database library that can be embedded in other services.
- Keep the command-line tooling (CLI) separate from the core library and implement a small, idiomatic CLI that calls into the library.
- Replace the `MockNetwork` with a production-quality pluggable network implementation (e.g., libp2p) while keeping the `Network` interface stable.
- Improve persistence (configurable backends), add optional IPFS blob backing, and secure sensitive data.
- Ensure correctness of CRDT operations (unit and property tests), resilient sync (two-node/integration tests), and operational tooling (metrics, logging, health-checks).

---

## 2) Project Layout (recommended) 🔧

- Keep `cmd/knirvbase` for CLI and minimal bootstrapping only.
- Move library code to an importable package (recommended: `pkg/knirvbase`) with a small, well-documented public API surface.
- Keep internal, implementation-focused packages under `internal/` (collection, database, storage, clock, network, resolver, query, types).

Proposed structure:

- `cmd/knirvbase/` — CLI binary and YAML config loader; only thin wiring logic
- `pkg/knirvbase/` — exported API (Database, Options, Client) for embedding
- `internal/collection` — `DistributedCollection` and public interfaces used by `pkg`
- `internal/database` — orchestration; exported wrapper functions in `pkg/knirvbase`
- `internal/network` — `Network` interface and implementations (Mock, libp2p)
- `internal/storage` — `Storage` interface and implementations (File, Badger/Bolt/SQLite)
- `internal/resolver` — CRDT resolution logic (robust, tested)
- `internal/query` — KNIRVQL parsing & execution
- `internal/types` — domain types and serializable structs

Rationale: `pkg/` exposes stable API for consuming projects; `internal/` allows safe refactors without breaking external consumers.

---

## 3) CLI vs Library: Responsibilities & API design 🧭

CLI (`cmd/knirvbase`):
- Configuration parsing (YAML/ENV), logging setup, metrics endpoint, profiling flags
- Start/stop commands and administrative subcommands (init, join, create-collection, backup, restore, inspect)
- A small command to run local integration tests (e.g., `knirvbase test-local`) and health-checks
- CLI accepts: `--config`, `--data-dir`, `--network-id`, `--bootstrap`, `--verbose` flags

Library (`pkg/knirvbase`):
- Provide programmatic API to create and manage databases:
  - New(opts) (*knirvbase.DB, error)
  - (DB) CreateNetwork(cfg)
  - (DB) JoinNetwork(networkID, peers)
  - (DB) Collection(name) *collection.Object
  - (DB) Shutdown() error
- Keep Network and Storage as interfaces that can be injected for tests and alternate implementations
- Provide well-documented examples and godoc for public methods

Example usage snippet (for `pkg/knirvbase`):

```go
import (
  "context"
  kb "github.com/knirvcorp/knirvbase/go/pkg/knirvbase"
)

func Example() {
  db, _ := kb.New(context.Background(), kb.Options{DataDir: "/var/lib/knirvbase"})
  defer db.Shutdown()
  coll := db.Collection("memory")
  coll.Insert(context.Background(), map[string]interface{}{"id":"x1", "payload": map[string]interface{}{"source":"web"}})
}
```

---

## 4) Networking Implementation (libp2p plan) 🌐

Milestone: Replace `MockNetwork` with a `libp2p`-backed implementation and keep `Network` interface intact.

Tasks:
- Add `go-libp2p` dependency and lightweight wrapper (e.g., `internal/network/libp2p`)
- Implement discovery (mDNS + bootstrap), transports (TCP, WebRTC optional), and NAT traversal
- Use Noise/TLS for encrypted transport and peer authentication
- Decide message serialization: JSON (proto compatibility) vs Protobuf; provide versioned message formats
- Implement peer management, message routing, and request/response (sync request/response)
- Provide integration tests for two-node and multi-node sync using docker-in-docker or in-memory libp2p
- Add configuration for bootstrap peers, listen addresses, and identity keys

Security: store node keys securely (e.g., $XDG_DATA_HOME/keys, encrypted with passphrase when needed)

---

## 5) Storage & Blob Management (persistence) 🗄️

Short-term:
- Improve `FileStorage` with robust error handling and concurrency-safe operations
- Add optional database-backed stores (Badger / Bolt / SQLite) behind the `Storage` interface
- Implement configurable encryption-at-rest layer (optional)

Long-term:
- IPFS integration for blobs: expose a `BlobStore` interface and provide an IPFS-backed implementation
- Use content-addressed storage (CID) for blobs and store CID in the synchronized metadata
- Provide blob fetch-on-demand logic for peers that need full blob data

---

## 6) CRDT & Sync Robustness ✅

Tasks:
- Harden resolver: add comprehensive unit tests covering concurrent updates, deletes, tombstones, and merge semantics
- Persist operation log to disk; support compaction and snapshotting
- Implement checks for operation log replays and idempotency
- Add hooks for staged posting (e.g., `Stage` => queued KNIRVGRAPH posting)
- Add property-based tests to assert commutativity/associativity of operations

Acceptance criteria: sync converges across nodes in deterministic tests; no data loss on restart; tombstones eventually compacted.

---

## 7) KNIRVQL & Vector Search Enhancements 🔎

- Add a pluggable vector index interface (ANN). Provide a simple in-memory HNSW wrapper or call out to an external service (e.g., Faiss via gRPC).
- Add optional vector normalization and configurable distance metric (cosine, euclidean)
- Improve KNIRVQL parser to support richer filters and pagination
- Add unit and integration tests for vector queries

---

## 8) Testing, CI, Static Analysis 🧪

Essential tests:
- Unit tests for `clock`, `resolver`, `storage`, `query`
- Integration tests for `DistributedCollection` with network (two-node and multi-node scenarios)
- End-to-end tests using docker compose with libp2p nodes
- Property-based tests for CRDT convergence

CI:
- GitHub Actions pipeline with jobs: `lint` (gofmt, govet), `test` (go test ./...), `build` (cross-compile), `integration` (optional on-demand), and `security` (trivy via codacy pipeline).

Static analysis and code quality:
- `gofmt`, `go vet`, `staticcheck`
- Run `codacy_cli_analyze` after edits as required by repo policies

---

## 9) Security & Privacy 🔒

- **Transport security:** Use authenticated and encrypted transports (libp2p Noise/TLS)
- **Secrets storage:** Do not store auth secrets in plaintext; provide optional seamless encryption for `AUTH` collection
- **Access control:** Add a pluggable auth provider for access control if used in multi-tenant setups
- **Audit logs:** Optionally store immutable logs of operations for forensic purposes

---

## 10) Observability & Operations 📈

- Expose metrics (Prometheus): peer_count, ops_sent, ops_recv, bytes_sent, sync_duration, pending_ops
- Expose health endpoint and readiness/liveness probes
- Add debug mode with pprof and verbose logging

---

## 11) Packaging & Deployment 📦

- Provide Dockerfile and small image that runs the `pkg/knirvbase` binary
- Provide `systemd` unit example and `docker-compose`/Helm charts for local cluster testing
- Provide backups (export collections) and restore tooling

---

## 12) Roadmap & Milestones (phased plan) 🗺️

Phase 0 — Stabilize (1–2 weeks)
- Split CLI from library; add `pkg/knirvbase` wrapper
- Add unit tests for `clock`, `resolver`, `storage`
- Improve `FileStorage` error handling

Phase 1 — Networking (2–4 weeks)
- Implement `libp2p` network manager
- Add discovery, bootstrap, and secure transports
- Integration tests for two-node sync

Phase 2 — Persistence & Blobs (2–4 weeks)
- Add Badger/SQLite store implementations
- Implement blob CID storage and optional IPFS blob store

Phase 3 — Indexing & Query (2–4 weeks)
- Add vector index (HNSW integration)
- Expand KNIRVQL and add performance tests

Phase 4 — Production hardening (2–4 weeks)
- Metrics, logging, tracing
- Security review, encryption at rest
- CI integration and release packaging

Each phase includes tests, documentation updates, and a short demo script.

---

## 13) Checklist / Acceptance Criteria ✅

- [ ] Public API in `pkg/knirvbase` with examples
- [ ] CLI `cmd/knirvbase` that boots the DB from config
- [ ] libp2p-backed `Network` implementation with tests
- [ ] Configurable storage backends and optional IPFS blob integration
- [ ] Unit and integration tests with passing CI
- [ ] Documentation: `README.md`, `Full_Implementation.md`, and examples
- [ ] Security and observability features implemented

---

## 14) Configuration Example (YAML)

```yaml
data_dir: "/var/lib/knirvbase"
network:
  id: "consortium-1"
  bootstrap_peers: ["/ip4/1.2.3.4/tcp/4001/p2p/Qm..."]
  listen_addrs: ["/ip4/0.0.0.0/tcp/4001"]
storage:
  backend: "file"  # file | badger | sqlite
  encryption: false
logging:
  level: "info"
metrics:
  prom_listen: ":8080"
```

---

## 15) Next immediate steps (I can implement) ✨

1. Split `cmd/main.go` into `cmd/knirvbase` and create `pkg/knirvbase` wrapper with public constructors and examples. (Low-risk, immediate)
2. Add unit tests for `knirvql` and `resolver`. (Short)
3. Add `libp2p` skeleton `internal/network/libp2p` with a simple two-node test harness. (Medium priority)

Tell me which of these you want me to tackle next and I will create a focused todo list and begin implementation.

---

*If desired, I can also open PRs, add CI workflows, and propose a libp2p skeleton implementation in a follow-up.*
