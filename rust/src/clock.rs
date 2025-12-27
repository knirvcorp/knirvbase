use std::collections::HashMap;
use serde::{Deserialize, Serialize};

/// VectorClock maps peer IDs to counters
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct VectorClock(pub HashMap<String, i64>);

/// ComparisonResult is the relationship between two vector clocks
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ComparisonResult {
    Equal,
    Before,
    After,
    Concurrent,
}

impl VectorClock {
    /// Increment increments a peer counter on the vector clock
    pub fn increment(mut self, peer_id: String) -> Self {
        let counter = self.0.entry(peer_id).or_insert(0);
        *counter += 1;
        self
    }

    /// Merge two vector clocks (take max per peer)
    pub fn merge(self, other: &VectorClock) -> VectorClock {
        let mut merged = self.0.clone();
        for (k, v) in &other.0 {
            let entry = merged.entry(k.clone()).or_insert(0);
            if *v > *entry {
                *entry = *v;
            }
        }
        VectorClock(merged)
    }

    /// Compare returns Equal|Before|After|Concurrent
    pub fn compare(&self, other: &VectorClock) -> ComparisonResult {
        let mut has_greater = false;
        let mut has_less = false;

        let mut all_keys = self.0.keys().collect::<std::collections::HashSet<_>>();
        all_keys.extend(other.0.keys());

        for key in all_keys {
            let v1 = self.0.get(key).copied().unwrap_or(0);
            let v2 = other.0.get(key).copied().unwrap_or(0);

            if v1 > v2 {
                has_greater = true;
            }
            if v1 < v2 {
                has_less = true;
            }
        }

        match (has_greater, has_less) {
            (false, false) => ComparisonResult::Equal,
            (true, false) => ComparisonResult::After,
            (false, true) => ComparisonResult::Before,
            (true, true) => ComparisonResult::Concurrent,
        }
    }

    /// HappensBefore returns true if self is before or equal to other
    pub fn happens_before(&self, other: &VectorClock) -> bool {
        matches!(
            self.compare(other),
            ComparisonResult::Before | ComparisonResult::Equal
        )
    }

    /// NewVectorClock returns an empty clock
    pub fn new() -> Self {
        VectorClock(HashMap::new())
    }

    /// Clone returns a copy
    pub fn clone(&self) -> Self {
        VectorClock(self.0.clone())
    }
}

impl Default for VectorClock {
    fn default() -> Self {
        Self::new()
    }
}