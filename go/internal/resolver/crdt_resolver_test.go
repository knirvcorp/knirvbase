package resolver

import (
	"testing"

	"github.com/knirvcorp/knirvbase/go/internal/clock"
	"github.com/knirvcorp/knirvbase/go/internal/types"
)

func TestResolveConflict(t *testing.T) {
	// Test nil cases
	local := &types.DistributedDocument{ID: "1", Payload: map[string]interface{}{"data": "local"}}
	if ResolveConflict(local, nil) != local {
		t.Error("Expected local when remote is nil")
	}
	if ResolveConflict(nil, local) != local {
		t.Error("Expected remote when local is nil")
	}

	// Test vector clock comparison
	v1 := clock.VectorClock{"a": 1}
	v2 := clock.VectorClock{"a": 2}
	doc1 := &types.DistributedDocument{ID: "1", Vector: v1, Timestamp: 100, PeerID: "a"}
	doc2 := &types.DistributedDocument{ID: "1", Vector: v2, Timestamp: 200, PeerID: "b"}

	result := ResolveConflict(doc1, doc2)
	if result.Vector["a"] != 2 {
		t.Error("Expected higher vector clock to win")
	}
}

func TestApplyOperation(t *testing.T) {
	// Test insert
	op := types.CRDTOperation{
		Type: types.OpInsert,
		Data: &types.DistributedDocument{
			ID:      "1",
			Payload: map[string]interface{}{"data": "test"},
			Vector:  clock.VectorClock{"a": 1},
		},
	}
	result := ApplyOperation(nil, op)
	if result == nil || result.ID != "1" {
		t.Error("Insert operation failed")
	}

	// Test update
	doc := &types.DistributedDocument{ID: "1", Vector: clock.VectorClock{"a": 1}}
	op = types.CRDTOperation{
		Type:   types.OpUpdate,
		Data:   &types.DistributedDocument{Payload: map[string]interface{}{"data": "updated"}},
		Vector: clock.VectorClock{"a": 2},
	}
	result = ApplyOperation(doc, op)
	if result.Payload["data"] != "updated" {
		t.Error("Update operation failed")
	}

	// Test delete
	op = types.CRDTOperation{
		Type:   types.OpDelete,
		Vector: clock.VectorClock{"a": 3},
	}
	result = ApplyOperation(doc, op)
	if !result.Deleted {
		t.Error("Delete operation failed")
	}
}

func TestToDistributed(t *testing.T) {
	payload := map[string]interface{}{"id": "1", "data": "test"}
	doc := ToDistributed(payload, "peer1")
	if doc.ID != "1" || doc.PeerID != "peer1" {
		t.Error("ToDistributed failed")
	}
	if doc.Vector["peer1"] != 1 {
		t.Error("Vector clock not set correctly")
	}
}

func TestToRegular(t *testing.T) {
	doc := &types.DistributedDocument{
		ID:      "1",
		Payload: map[string]interface{}{"data": "test"},
	}
	regular := ToRegular(doc)
	if regular["data"] != "test" {
		t.Error("ToRegular failed")
	}
	if ToRegular(nil) != nil {
		t.Error("ToRegular with nil should return nil")
	}
}

func TestCloneMap(t *testing.T) {
	original := map[string]interface{}{"a": 1, "b": "test"}
	cloned := cloneMap(original)
	if cloned["a"] != 1 || cloned["b"] != "test" {
		t.Error("Clone failed")
	}
	// Modify original to ensure independence
	original["a"] = 2
	if cloned["a"] == 2 {
		t.Error("Clone is not independent")
	}
}
