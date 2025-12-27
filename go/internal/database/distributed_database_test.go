package distributed

import (
	"context"
	"testing"

	stor "github.com/knirvcorp/knirvbase/go/internal/storage"
	typ "github.com/knirvcorp/knirvbase/go/internal/types"
)

type mockStorage struct{}

func (m *mockStorage) Insert(collection string, doc map[string]interface{}) error        { return nil }
func (m *mockStorage) Update(collection, id string, update map[string]interface{}) error { return nil }
func (m *mockStorage) Delete(collection, id string) error                                { return nil }
func (m *mockStorage) Find(collection, id string) (map[string]interface{}, error)        { return nil, nil }
func (m *mockStorage) FindAll(collection string) ([]map[string]interface{}, error)       { return nil, nil }
func (m *mockStorage) CreateIndex(collection, name string, indexType stor.IndexType, fields []string, unique bool, partialExpr string, options map[string]interface{}) error {
	return nil
}
func (m *mockStorage) DropIndex(collection, name string) error                 { return nil }
func (m *mockStorage) GetIndex(collection, name string) *stor.Index            { return nil }
func (m *mockStorage) GetIndexesForCollection(collection string) []*stor.Index { return nil }
func (m *mockStorage) QueryIndex(collection, indexName string, query map[string]interface{}) ([]string, error) {
	return nil, nil
}

func TestNewDistributedDatabase(t *testing.T) {
	ctx := context.Background()
	opts := DistributedDbOptions{}
	store := &mockStorage{}
	db, err := NewDistributedDatabase(ctx, opts, store)
	if err != nil {
		t.Errorf("NewDistributedDatabase failed: %v", err)
	}
	if db == nil {
		t.Error("Database is nil")
	}
}

func TestDistributedDatabaseCollection(t *testing.T) {
	ctx := context.Background()
	opts := DistributedDbOptions{}
	store := &mockStorage{}
	db, _ := NewDistributedDatabase(ctx, opts, store)
	coll := db.Collection("test", store)
	if coll == nil {
		t.Error("Collection is nil")
	}
	if coll.Name != "test" {
		t.Errorf("Expected name 'test', got %s", coll.Name)
	}
}

func TestDistributedDatabaseCreateNetwork(t *testing.T) {
	ctx := context.Background()
	opts := DistributedDbOptions{}
	store := &mockStorage{}
	db, _ := NewDistributedDatabase(ctx, opts, store)
	cfg := typ.NetworkConfig{NetworkID: "net1", Name: "Test Network"}
	id, err := db.CreateNetwork(cfg)
	if err != nil {
		t.Errorf("CreateNetwork failed: %v", err)
	}
	if id != "net1" {
		t.Errorf("Expected id 'net1', got %s", id)
	}
}

func TestDistributedDatabaseShutdown(t *testing.T) {
	ctx := context.Background()
	opts := DistributedDbOptions{}
	store := &mockStorage{}
	db, _ := NewDistributedDatabase(ctx, opts, store)
	err := db.Shutdown()
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}
