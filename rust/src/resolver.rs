use std::collections::HashMap;
use crate::types::*;

/// CRDTResolver handles conflict resolution for CRDT operations
pub struct CRDTResolver;

impl CRDTResolver {
    /// NewCRDTResolver creates a new CRDT resolver
    pub fn new() -> Self {
        CRDTResolver
    }

    /// ResolveConflicts resolves conflicts between CRDT operations
    pub fn resolve_conflicts(&self, operations: Vec<CRDTOperation>) -> Vec<CRDTOperation> {
        // Simple last-writer-wins resolution based on timestamp
        let mut resolved = operations;
        resolved.sort_by(|a, b| b.timestamp.cmp(&a.timestamp));
        resolved.dedup_by(|a, b| a.document_id == b.document_id);
        resolved
    }

    /// MergeDocuments merges two document versions using CRDT rules
    pub fn merge_documents(&self, local: &DistributedDocument, remote: &DistributedDocument) -> DistributedDocument {
        // Compare vector clocks
        match local.vector.compare(&remote.vector) {
            crate::clock::ComparisonResult::Equal | crate::clock::ComparisonResult::Before => {
                // Remote is newer or equal, use remote
                remote.clone()
            }
            crate::clock::ComparisonResult::After => {
                // Local is newer, use local
                local.clone()
            }
            crate::clock::ComparisonResult::Concurrent => {
                // Concurrent changes, merge payloads
                let mut merged = local.clone();
                if let (Some(local_payload), Some(remote_payload)) = (&local.payload, &remote.payload) {
                    let mut merged_payload = local_payload.clone();
                    for (key, value) in remote_payload {
                        merged_payload.insert(key.clone(), value.clone());
                    }
                    merged.payload = Some(merged_payload);
                }
                // Update vector clock
                merged.vector = local.vector.clone().merge(&remote.vector);
                merged.timestamp = std::cmp::max(local.timestamp, remote.timestamp);
                merged
            }
        }
    }
}