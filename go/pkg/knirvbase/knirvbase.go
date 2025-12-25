package knirvbase

import (
	"context"
	"fmt"

	coll "github.com/knirv/knirvbase/internal/collection"
	db "github.com/knirv/knirvbase/internal/database"
	stor "github.com/knirv/knirvbase/internal/storage"
	typ "github.com/knirv/knirvbase/internal/types"
)

// Options contains configuration for the library
type Options struct {
	DataDir                   string
	DistributedEnabled        bool
	DistributedNetworkID      string
	DistributedBootstrapPeers []string
}

// DB is the public wrapper around the internal DistributedDatabase
type DB struct {
	db    *db.DistributedDatabase
	store stor.Storage
}

// New constructs a DB instance with the provided options and storage
func New(ctx context.Context, opts Options) (*DB, error) {
	if opts.DataDir == "" {
		return nil, fmt.Errorf("DataDir cannot be empty")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	store := stor.NewFileStorage(opts.DataDir)
	dopts := db.DistributedDbOptions{}
	dopts.Distributed.Enabled = opts.DistributedEnabled
	dopts.Distributed.NetworkID = opts.DistributedNetworkID
	dopts.Distributed.BootstrapPeers = opts.DistributedBootstrapPeers

	inner, err := db.NewDistributedDatabase(ctx, dopts, store)
	if err != nil {
		return nil, fmt.Errorf("failed to create distributed database: %w", err)
	}
	return &DB{db: inner, store: store}, nil
}

// CreateNetwork creates a network using the underlying manager
func (d *DB) CreateNetwork(cfg typ.NetworkConfig) (string, error) {
	if d.db == nil {
		return "", fmt.Errorf("database not initialized")
	}
	return d.db.CreateNetwork(cfg)
}

// JoinNetwork joins an existing network
func (d *DB) JoinNetwork(networkID string, bootstrapPeers []string) error {
	return d.db.JoinNetwork(networkID, bootstrapPeers)
}

// LeaveNetwork leaves a network
func (d *DB) LeaveNetwork(networkID string) error {
	return d.db.LeaveNetwork(networkID)
}

// Collection returns a collection interface for use by callers
func (d *DB) Collection(name string) Collection {
	if d.db == nil {
		panic("database not initialized")
	}
	if name == "" {
		panic("collection name cannot be empty")
	}
	c := d.db.Collection(name, d.store)
	return &collectionAdapter{c: c}
}

// Raw returns the underlying internal DistributedDatabase for advanced usage
func (d *DB) Raw() *db.DistributedDatabase { return d.db }

// RawCollection returns the underlying internal DistributedCollection for advanced usage
func (d *DB) RawCollection(name string) *coll.DistributedCollection {
	return d.db.Collection(name, d.store)
}

// Shutdown stops the underlying network manager
func (d *DB) Shutdown() error {
	return d.db.Shutdown()
}

// Collection is a thin interface representing collection operations consumers need
type Collection interface {
	Insert(ctx context.Context, doc map[string]interface{}) (map[string]interface{}, error)
	Update(id string, update map[string]interface{}) (int, error)
	Delete(id string) (int, error)
	Find(id string) (map[string]interface{}, error)
	FindAll() ([]map[string]interface{}, error)
	AttachToNetwork(networkID string) error
	DetachFromNetwork() error
	ForceSync() error
}

// collectionAdapter adapts internal DistributedCollection to the Collection interface
type collectionAdapter struct{ c *coll.DistributedCollection }

func (a *collectionAdapter) Insert(ctx context.Context, doc map[string]interface{}) (map[string]interface{}, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	if doc == nil {
		return nil, fmt.Errorf("document cannot be nil")
	}
	if id, ok := doc["id"].(string); !ok || id == "" {
		return nil, fmt.Errorf("document must contain a non-empty 'id' field")
	}
	return a.c.Insert(ctx, doc)
}
func (a *collectionAdapter) Update(id string, update map[string]interface{}) (int, error) {
	return a.c.Update(id, update)
}
func (a *collectionAdapter) Delete(id string) (int, error)                  { return a.c.Delete(id) }
func (a *collectionAdapter) Find(id string) (map[string]interface{}, error) { return a.c.Find(id) }
func (a *collectionAdapter) FindAll() ([]map[string]interface{}, error)     { return a.c.FindAll() }
func (a *collectionAdapter) AttachToNetwork(networkID string) error {
	return a.c.AttachToNetwork(networkID)
}
func (a *collectionAdapter) DetachFromNetwork() error { return a.c.DetachFromNetwork() }
func (a *collectionAdapter) ForceSync() error         { return a.c.ForceSync() }
