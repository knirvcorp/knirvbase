package distributed

import (
	"context"
	"os"
	"testing"

	dbpkg "github.com/knirvcorp/knirvbase/go/internal/database"
	stor "github.com/knirvcorp/knirvbase/go/internal/storage"
)

func TestKNIRVQLIndexCommands(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "knirvql_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create storage and database
	store := stor.NewFileStorage(tmpDir)
	database, err := dbpkg.NewDistributedDatabase(context.Background(), dbpkg.DistributedDbOptions{}, store)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Shutdown()

	// Get collection
	collection := database.Collection("users", store)

	// Test CREATE INDEX command
	parser := &KNIRVQLParser{}
	query, err := parser.Parse("CREATE INDEX users:username ON users (username) UNIQUE")
	if err != nil {
		t.Fatalf("Failed to parse CREATE INDEX: %v", err)
	}

	_, err = query.Execute(database, collection)
	if err != nil {
		t.Fatalf("Failed to execute CREATE INDEX: %v", err)
	}

	// Verify index was created
	indexes := database.GetIndexesForCollection("users")
	if len(indexes) != 1 {
		t.Fatalf("Expected 1 index, got %d", len(indexes))
	}

	if indexes[0].Name != "username" || !indexes[0].Unique || indexes[0].Type != stor.IndexTypeBTree {
		t.Fatal("Index properties incorrect")
	}

	// Insert some test data
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

	_, err = collection.Insert(context.Background(), doc1)
	if err != nil {
		t.Fatalf("Failed to insert user1: %v", err)
	}

	_, err = collection.Insert(context.Background(), doc2)
	if err != nil {
		t.Fatalf("Failed to insert user2: %v", err)
	}

	// Test DROP INDEX command
	query, err = parser.Parse("DROP INDEX users:username")
	if err != nil {
		t.Fatalf("Failed to parse DROP INDEX: %v", err)
	}

	_, err = query.Execute(database, collection)
	if err != nil {
		t.Fatalf("Failed to execute DROP INDEX: %v", err)
	}

	// Verify index was dropped
	indexes = database.GetIndexesForCollection("users")
	if len(indexes) != 0 {
		t.Fatalf("Expected 0 indexes after drop, got %d", len(indexes))
	}
}

func TestKNIRVQLInsertWithIndex(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "knirvql_insert_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create storage and database
	store := stor.NewFileStorage(tmpDir)
	database, err := dbpkg.NewDistributedDatabase(context.Background(), dbpkg.DistributedDbOptions{}, store)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Shutdown()

	collection := database.Collection("credentials", store)

	// Create index
	err = database.CreateIndex("credentials", "username", stor.IndexTypeBTree, []string{"username"}, true, "", nil)
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	// Insert document
	doc := map[string]interface{}{
		"id":       "cred1",
		"username": "alice@example.com",
		"hash":     "hashed_password",
		"status":   "active",
	}

	_, err = collection.Insert(context.Background(), doc)
	if err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}

	// Query index
	results, err := database.QueryIndex("credentials", "username", map[string]interface{}{
		"value": "alice@example.com",
	})
	if err != nil {
		t.Fatalf("Failed to query index: %v", err)
	}

	if len(results) != 1 || results[0] != "cred1" {
		t.Fatalf("Expected [cred1], got %v", results)
	}
}

func TestQueryOptimizer(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "optimizer_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create storage and database
	store := stor.NewFileStorage(tmpDir)
	database, err := dbpkg.NewDistributedDatabase(context.Background(), dbpkg.DistributedDbOptions{}, store)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Shutdown()

	// Create index
	err = database.CreateIndex("test", "name", stor.IndexTypeBTree, []string{"name"}, false, "", nil)
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	// Create optimizer
	indexes := database.GetIndexesForCollection("test")
	optimizer := NewQueryOptimizer("test", indexes, nil)

	// Create query
	parser := &KNIRVQLParser{}
	query, err := parser.Parse("GET MEMORY WHERE name = \"alice\"")
	if err != nil {
		t.Fatalf("Failed to parse query: %v", err)
	}

	// Optimize
	plan, err := optimizer.Optimize(query)
	if err != nil {
		t.Fatalf("Failed to optimize: %v", err)
	}

	// Check plan
	if !plan.UseIndex {
		t.Fatal("Expected to use index")
	}
	if plan.IndexName != "name" {
		t.Fatalf("Expected index name 'name', got %s", plan.IndexName)
	}
	if plan.ScanType != IndexOnlyScan {
		t.Fatalf("Expected IndexOnlyScan, got %v", plan.ScanType)
	}
}
