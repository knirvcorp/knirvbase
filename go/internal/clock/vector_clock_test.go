package clock

import (
	"testing"
)

func TestIncrement(t *testing.T) {
	clock := NewVectorClock()
	clock = Increment(clock, "peer1")
	if clock["peer1"] != 1 {
		t.Errorf("Expected 1, got %d", clock["peer1"])
	}
	clock = Increment(clock, "peer1")
	if clock["peer1"] != 2 {
		t.Errorf("Expected 2, got %d", clock["peer1"])
	}
}

func TestIncrementNil(t *testing.T) {
	var clock VectorClock
	clock = Increment(clock, "peer1")
	if clock["peer1"] != 1 {
		t.Errorf("Expected 1, got %d", clock["peer1"])
	}
}

func TestMerge(t *testing.T) {
	clock1 := VectorClock{"a": 1, "b": 2}
	clock2 := VectorClock{"a": 3, "c": 4}
	merged := Merge(clock1, clock2)
	if merged["a"] != 3 || merged["b"] != 2 || merged["c"] != 4 {
		t.Errorf("Merge failed: %v", merged)
	}
}

func TestCompare(t *testing.T) {
	clock1 := VectorClock{"a": 1, "b": 2}
	clock2 := VectorClock{"a": 1, "b": 2}
	if Compare(clock1, clock2) != Equal {
		t.Error("Expected Equal")
	}

	clock3 := VectorClock{"a": 2, "b": 2}
	if Compare(clock1, clock3) != Before {
		t.Error("Expected Before")
	}

	clock4 := VectorClock{"a": 0, "b": 2}
	if Compare(clock1, clock4) != After {
		t.Error("Expected After")
	}

	clock5 := VectorClock{"a": 2, "b": 1}
	if Compare(clock1, clock5) != Concurrent {
		t.Error("Expected Concurrent")
	}
}

func TestHappensBefore(t *testing.T) {
	clock1 := VectorClock{"a": 1, "b": 2}
	clock2 := VectorClock{"a": 1, "b": 2}
	if !HappensBefore(clock1, clock2) {
		t.Error("Equal should happen before")
	}

	clock3 := VectorClock{"a": 2, "b": 2}
	if !HappensBefore(clock1, clock3) {
		t.Error("Before should happen before")
	}

	clock4 := VectorClock{"a": 0, "b": 2}
	if HappensBefore(clock1, clock4) {
		t.Error("After should not happen before")
	}
}

func TestClone(t *testing.T) {
	clock := VectorClock{"a": 1, "b": 2}
	cloned := Clone(clock)
	if cloned["a"] != 1 || cloned["b"] != 2 {
		t.Errorf("Clone failed: %v", cloned)
	}
	cloned["a"] = 3
	if clock["a"] != 1 {
		t.Error("Clone should be independent")
	}
}

func TestCloneNil(t *testing.T) {
	var clock VectorClock
	cloned := Clone(clock)
	if cloned != nil {
		t.Error("Clone of nil should be nil")
	}
}