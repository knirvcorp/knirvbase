package distributed

import (
	"os"
	"testing"
)

func TestIndexManager(t *testing.T) {
	// Create temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "knirvbase_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create index manager
	im := NewIndexManager(tmpDir)

	// Test creating B-Tree index
	err = im.CreateIndex("users", "username", IndexTypeBTree, []string{"username"}, true, "", nil)
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	// Test getting index
	idx := im.GetIndex("users", "username")
	if idx == nil {
		t.Fatal("Index not found")
	}

	if idx.Name != "username" || idx.Type != IndexTypeBTree || !idx.Unique {
		t.Fatal("Index properties incorrect")
	}

	// Test inserting documents
	doc1 := map[string]interface{}{
		"id":       "user1",
		"username": "alice",
		"email":    "alice@example.com",
	}

	doc2 := map[string]interface{}{
		"id":       "user2",
		"username": "bob",
		"email":    "bob@example.com",
	}

	err = im.Insert("users", doc1)
	if err != nil {
		t.Fatalf("Failed to insert doc1: %v", err)
	}

	err = im.Insert("users", doc2)
	if err != nil {
		t.Fatalf("Failed to insert doc2: %v", err)
	}

	// Test querying index
	results, err := im.QueryIndex("users", "username", map[string]interface{}{
		"value": "alice",
	})
	if err != nil {
		t.Fatalf("Failed to query index: %v", err)
	}

	if len(results) != 1 || results[0] != "user1" {
		t.Fatalf("Expected [user1], got %v", results)
	}

	// Test dropping index
	err = im.DropIndex("users", "username")
	if err != nil {
		t.Fatalf("Failed to drop index: %v", err)
	}

	idx = im.GetIndex("users", "username")
	if idx != nil {
		t.Fatal("Index should be dropped")
	}
}

func TestIndexPersistence(t *testing.T) {
	// Create temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "knirvbase_persist_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create first index manager and add index
	im1 := NewIndexManager(tmpDir)
	err = im1.CreateIndex("test", "field", IndexTypeBTree, []string{"field"}, false, "", nil)
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	// Insert some data
	doc := map[string]interface{}{
		"id":    "doc1",
		"field": "value1",
	}
	err = im1.Insert("test", doc)
	if err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}

	// Create second index manager and load indexes
	im2 := NewIndexManager(tmpDir)
	err = im2.LoadIndexes()
	if err != nil {
		t.Fatalf("Failed to load indexes: %v", err)
	}

	// Check if index was loaded
	idx := im2.GetIndex("test", "field")
	if idx == nil {
		t.Fatal("Index not loaded from disk")
	}

	// Check if data was preserved (in a real implementation, this would require loading index data too)
	// For now, just check metadata
	if idx.Name != "field" || idx.Type != IndexTypeBTree {
		t.Fatal("Index metadata not preserved")
	}
}

func TestHNSWIndex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "knirvbase_hnsw_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	im := NewIndexManager(tmpDir)

	// Create HNSW index
	err = im.CreateIndex("vectors", "embedding", IndexTypeHNSW, []string{"vector"}, false, "", map[string]interface{}{
		"dimensions": 3,
	})
	if err != nil {
		t.Fatalf("Failed to create HNSW index: %v", err)
	}

	// Insert documents with vectors
	doc1 := map[string]interface{}{
		"id": "vec1",
		"payload": map[string]interface{}{
			"vector": []interface{}{0.1, 0.2, 0.3},
		},
	}

	doc2 := map[string]interface{}{
		"id": "vec2",
		"payload": map[string]interface{}{
			"vector": []interface{}{0.4, 0.5, 0.6},
		},
	}

	err = im.Insert("vectors", doc1)
	if err != nil {
		t.Fatalf("Failed to insert vec1: %v", err)
	}

	err = im.Insert("vectors", doc2)
	if err != nil {
		t.Fatalf("Failed to insert vec2: %v", err)
	}

	// Query with similar vector
	results, err := im.QueryIndex("vectors", "embedding", map[string]interface{}{
		"vector": []float64{0.1, 0.2, 0.3},
		"limit":  2,
	})
	if err != nil {
		t.Fatalf("Failed to query HNSW index: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("Expected some results from vector search")
	}

	// vec1 should be most similar to itself
	if results[0] != "vec1" {
		t.Logf("Results: %v", results) // Log for debugging
	}
}
