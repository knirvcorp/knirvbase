package clock

// VectorClock maps peer IDs to counters
type VectorClock map[string]int64

// ComparisonResult is the relationship between two vector clocks
type ComparisonResult int

const (
	Equal ComparisonResult = iota
	Before
	After
	Concurrent
)

// Increment increments a peer counter on the vector clock
func Increment(clock VectorClock, peerID string) VectorClock {
	if clock == nil {
		clock = make(VectorClock)
	}
	clock[peerID] = clock[peerID] + 1
	return clock
}

// Merge two vector clocks (take max per peer)
func Merge(clock1, clock2 VectorClock) VectorClock {
	merged := make(VectorClock)
	for k, v := range clock1 {
		merged[k] = v
	}
	for k, v := range clock2 {
		if existing, ok := merged[k]; !ok || v > existing {
			merged[k] = v
		}
	}
	return merged
}

// Compare returns Equal|Before|After|Concurrent
func Compare(clock1, clock2 VectorClock) ComparisonResult {
	hasGreater, hasLess := false, false

	allKeys := make(map[string]struct{})
	for k := range clock1 {
		allKeys[k] = struct{}{}
	}
	for k := range clock2 {
		allKeys[k] = struct{}{}
	}

	for k := range allKeys {
		v1 := int64(0)
		v2 := int64(0)
		if vv, ok := clock1[k]; ok {
			v1 = vv
		}
		if vv, ok := clock2[k]; ok {
			v2 = vv
		}

		if v1 > v2 {
			hasGreater = true
		}
		if v1 < v2 {
			hasLess = true
		}
	}

	switch {
	case !hasGreater && !hasLess:
		return Equal
	case hasGreater && !hasLess:
		return After
	case hasLess && !hasGreater:
		return Before
	default:
		return Concurrent
	}
}

// HappensBefore returns true if clock1 is before or equal to clock2
func HappensBefore(clock1, clock2 VectorClock) bool {
	return Compare(clock1, clock2) == Before || Compare(clock1, clock2) == Equal
}

// NewVectorClock returns an empty clock
func NewVectorClock() VectorClock { return make(VectorClock) }

// Clone returns a shallow copy
func Clone(clock VectorClock) VectorClock {
	if clock == nil {
		return nil
	}
	copy := make(VectorClock, len(clock))
	for k, v := range clock {
		copy[k] = v
	}
	return copy
}
