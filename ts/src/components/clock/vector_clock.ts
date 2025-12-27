// VectorClock maps peer IDs to counters
export type VectorClock = Record<string, number>;

// ComparisonResult is the relationship between two vector clocks
export enum ComparisonResult {
  Equal,
  Before,
  After,
  Concurrent,
}

// Increment increments a peer counter on the vector clock
export function increment(clock: VectorClock, peerID: string): VectorClock {
  if (!clock) {
    clock = {};
  }
  clock[peerID] = (clock[peerID] || 0) + 1;
  return clock;
}

// Merge two vector clocks (take max per peer)
export function merge(clock1: VectorClock, clock2: VectorClock): VectorClock {
  const merged: VectorClock = {};
  for (const k in clock1) {
    merged[k] = clock1[k];
  }
  for (const k in clock2) {
    if (!(k in merged) || clock2[k] > merged[k]) {
      merged[k] = clock2[k];
    }
  }
  return merged;
}

// Compare returns Equal|Before|After|Concurrent
export function compare(clock1: VectorClock, clock2: VectorClock): ComparisonResult {
  let hasGreater = false;
  let hasLess = false;

  const allKeys = new Set([...Object.keys(clock1), ...Object.keys(clock2)]);

  for (const k of allKeys) {
    const v1 = clock1[k] || 0;
    const v2 = clock2[k] || 0;

    if (v1 > v2) {
      hasGreater = true;
    }
    if (v1 < v2) {
      hasLess = true;
    }
  }

  if (!hasGreater && !hasLess) {
    return ComparisonResult.Equal;
  } else if (hasGreater && !hasLess) {
    return ComparisonResult.After;
  } else if (hasLess && !hasGreater) {
    return ComparisonResult.Before;
  } else {
    return ComparisonResult.Concurrent;
  }
}

// HappensBefore returns true if clock1 is before or equal to clock2
export function happensBefore(clock1: VectorClock, clock2: VectorClock): boolean {
  const cmp = compare(clock1, clock2);
  return cmp === ComparisonResult.Before || cmp === ComparisonResult.Equal;
}

// NewVectorClock returns an empty clock
export function newVectorClock(): VectorClock {
  return {};
}

// Clone returns a shallow copy
export function clone(clock: VectorClock): VectorClock {
  if (!clock) {
    return {};
  }
  return { ...clock };
}