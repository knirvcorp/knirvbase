"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.ComparisonResult = void 0;
exports.increment = increment;
exports.merge = merge;
exports.compare = compare;
exports.happensBefore = happensBefore;
exports.newVectorClock = newVectorClock;
exports.clone = clone;
// ComparisonResult is the relationship between two vector clocks
var ComparisonResult;
(function (ComparisonResult) {
    ComparisonResult[ComparisonResult["Equal"] = 0] = "Equal";
    ComparisonResult[ComparisonResult["Before"] = 1] = "Before";
    ComparisonResult[ComparisonResult["After"] = 2] = "After";
    ComparisonResult[ComparisonResult["Concurrent"] = 3] = "Concurrent";
})(ComparisonResult || (exports.ComparisonResult = ComparisonResult = {}));
// Increment increments a peer counter on the vector clock
function increment(clock, peerID) {
    if (!clock) {
        clock = {};
    }
    clock[peerID] = (clock[peerID] || 0) + 1;
    return clock;
}
// Merge two vector clocks (take max per peer)
function merge(clock1, clock2) {
    const merged = {};
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
function compare(clock1, clock2) {
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
    }
    else if (hasGreater && !hasLess) {
        return ComparisonResult.After;
    }
    else if (hasLess && !hasGreater) {
        return ComparisonResult.Before;
    }
    else {
        return ComparisonResult.Concurrent;
    }
}
// HappensBefore returns true if clock1 is before or equal to clock2
function happensBefore(clock1, clock2) {
    const cmp = compare(clock1, clock2);
    return cmp === ComparisonResult.Before || cmp === ComparisonResult.Equal;
}
// NewVectorClock returns an empty clock
function newVectorClock() {
    return {};
}
// Clone returns a shallow copy
function clone(clock) {
    if (!clock) {
        return {};
    }
    return { ...clock };
}
//# sourceMappingURL=vector_clock.js.map