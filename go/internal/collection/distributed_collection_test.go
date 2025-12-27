package collection

import (
	"context"
	"testing"

	"github.com/knirvcorp/knirvbase/go/internal/network"
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

type mockNetwork struct{}

func (m *mockNetwork) Initialize() error                                                  { return nil }
func (m *mockNetwork) CreateNetwork(cfg typ.NetworkConfig) (string, error)                { return "net1", nil }
func (m *mockNetwork) JoinNetwork(networkID string, bootstrapPeers []string) error        { return nil }
func (m *mockNetwork) LeaveNetwork(networkID string) error                                { return nil }
func (m *mockNetwork) AddCollectionToNetwork(networkID, collectionName string) error      { return nil }
func (m *mockNetwork) RemoveCollectionFromNetwork(networkID, collectionName string) error { return nil }
func (m *mockNetwork) GetNetworkCollections(networkID string) []string                    { return nil }
func (m *mockNetwork) BroadcastMessage(networkID string, msg typ.ProtocolMessage) error   { return nil }
func (m *mockNetwork) SendToPeer(peerID, networkID string, msg typ.ProtocolMessage) error { return nil }
func (m *mockNetwork) OnMessage(mt typ.MessageType, handler network.MessageHandler)       {}
func (m *mockNetwork) GetNetworkStats(networkID string) *typ.NetworkStats                 { return nil }
func (m *mockNetwork) GetNetworks() []*typ.NetworkConfig                                  { return nil }
func (m *mockNetwork) GetPeerID() string                                                  { return "peer1" }
func (m *mockNetwork) Shutdown() error                                                    { return nil }

func TestNewLocalCollection(t *testing.T) {
	store := &mockStorage{}
	coll := NewLocalCollection("test", store)
	if coll.name != "test" {
		t.Errorf("Expected name 'test', got %s", coll.name)
	}
}

func TestLocalCollectionInsert(t *testing.T) {
	store := &mockStorage{}
	coll := NewLocalCollection("test", store)
	doc := map[string]interface{}{"id": "1", "data": "test"}
	result, err := coll.Insert(context.Background(), doc)
	if err != nil {
		t.Errorf("Insert failed: %v", err)
	}
	if result["id"] != "1" {
		t.Errorf("Expected id '1', got %v", result["id"])
	}
}

func TestLocalCollectionFind(t *testing.T) {
	store := &mockStorage{}
	coll := NewLocalCollection("test", store)
	_, err := coll.Find("1")
	if err != nil {
		t.Errorf("Find failed: %v", err)
	}
}

func TestNewDistributedCollection(t *testing.T) {
	store := &mockStorage{}
	net := &mockNetwork{}
	coll := NewDistributedCollection("test", net, store)
	if coll.Name != "test" {
		t.Errorf("Expected name 'test', got %s", coll.Name)
	}
}

func TestDistributedCollectionInsert(t *testing.T) {
	store := &mockStorage{}
	net := &mockNetwork{}
	coll := NewDistributedCollection("test", net, store)
	doc := map[string]interface{}{"id": "1", "entryType": typ.EntryTypeMemory, "payload": map[string]interface{}{}}
	result, err := coll.Insert(context.Background(), doc)
	if err != nil {
		t.Errorf("Insert failed: %v", err)
	}
	if result["id"] != "1" {
		t.Errorf("Expected id '1', got %v", result["id"])
	}
}

func TestDistributedCollectionFind(t *testing.T) {
	store := &mockStorage{}
	net := &mockNetwork{}
	coll := NewDistributedCollection("test", net, store)
	_, err := coll.Find("1")
	if err != nil {
		t.Errorf("Find failed: %v", err)
	}
}

func TestCloneMap(t *testing.T) {
	original := map[string]interface{}{
		"a": 1,
		"b": map[string]interface{}{"c": 2},
		"d": []interface{}{3, 4},
	}
	cloned := cloneMap(original)
	if cloned["a"] != 1 {
		t.Errorf("Primitive not cloned correctly")
	}
	if cloned["b"].(map[string]interface{})["c"] != 2 {
		t.Errorf("Nested map not cloned correctly")
	}
	if len(cloned["d"].([]interface{})) != 2 {
		t.Errorf("Slice not cloned correctly")
	}
	// Modify original to ensure clone is independent
	original["a"] = 999
	if cloned["a"] == 999 {
		t.Errorf("Clone is not independent")
	}
}

func TestCloneSlice(t *testing.T) {
	original := []interface{}{1, map[string]interface{}{"a": 2}, []interface{}{3}}
	cloned := cloneSlice(original)
	if cloned[0] != 1 {
		t.Errorf("Primitive not cloned correctly")
	}
	if cloned[1].(map[string]interface{})["a"] != 2 {
		t.Errorf("Nested map not cloned correctly")
	}
	if len(cloned[2].([]interface{})) != 1 {
		t.Errorf("Nested slice not cloned correctly")
	}
}
