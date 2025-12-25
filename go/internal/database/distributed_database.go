package distributed

import (
	"context"
	"errors"
	"sync"

	coll "github.com/knirvcorp/knirvbase/go/internal/collection"
	netpkg "github.com/knirvcorp/knirvbase/go/internal/network"
	stor "github.com/knirvcorp/knirvbase/go/internal/storage"
	typ "github.com/knirvcorp/knirvbase/go/internal/types"
)

type DistributedDbOptions struct {
	Distributed struct {
		Enabled        bool
		NetworkID      string
		BootstrapPeers []string
	}
}

type DistributedDatabase struct {
	network     netpkg.Network
	storage     stor.Storage
	distributed bool
	collections map[string]*coll.DistributedCollection
	mu          sync.Mutex
}

func NewDistributedDatabase(ctx context.Context, opts DistributedDbOptions, store stor.Storage) (*DistributedDatabase, error) {
	nm := netpkg.NewNetworkManager(ctx)
	db := &DistributedDatabase{network: nm, storage: store, distributed: opts.Distributed.Enabled, collections: make(map[string]*coll.DistributedCollection)}
	if db.distributed {
		if err := nm.Initialize(); err != nil {
			return nil, err
		}
		if opts.Distributed.NetworkID != "" {
			if len(opts.Distributed.BootstrapPeers) > 0 {
				if err := nm.JoinNetwork(opts.Distributed.NetworkID, opts.Distributed.BootstrapPeers); err != nil {
					return nil, err
				}
			} else {
				_, err := nm.CreateNetwork(typ.NetworkConfig{NetworkID: opts.Distributed.NetworkID, Name: "Network " + opts.Distributed.NetworkID})
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return db, nil
}

func (db *DistributedDatabase) Collection(name string, store stor.Storage) *coll.DistributedCollection {
	db.mu.Lock()
	defer db.mu.Unlock()

	if c, ok := db.collections[name]; ok {
		return c
	}
	c := coll.NewDistributedCollection(name, db.network, store)
	db.collections[name] = c
	return c
}

func (db *DistributedDatabase) CreateNetwork(cfg typ.NetworkConfig) (string, error) {
	if db.network == nil {
		return "", errors.New("network manager not initialized")
	}
	return db.network.CreateNetwork(cfg)
}

func (db *DistributedDatabase) JoinNetwork(networkID string, bootstrapPeers []string) error {
	if db.network == nil {
		return errors.New("network manager not initialized")
	}
	return db.network.JoinNetwork(networkID, bootstrapPeers)
}

func (db *DistributedDatabase) LeaveNetwork(networkID string) error {
	if db.network == nil {
		return errors.New("network manager not initialized")
	}
	return db.network.LeaveNetwork(networkID)
}

func (db *DistributedDatabase) AddCollectionToNetwork(networkID string, collectionName string) error {
	c := db.collections[collectionName]
	if c == nil {
		return errors.New("collection not found")
	}
	return c.AttachToNetwork(networkID)
}

func (db *DistributedDatabase) RemoveCollectionFromNetwork(collectionName string) error {
	c := db.collections[collectionName]
	if c == nil {
		return nil
	}
	return c.DetachFromNetwork()
}

func (db *DistributedDatabase) GetNetworkManager() netpkg.Network { return db.network }

// Index management methods
func (db *DistributedDatabase) CreateIndex(collection, name string, indexType stor.IndexType, fields []string, unique bool, partialExpr string, options map[string]interface{}) error {
	return db.storage.CreateIndex(collection, name, indexType, fields, unique, partialExpr, options)
}

func (db *DistributedDatabase) DropIndex(collection, name string) error {
	return db.storage.DropIndex(collection, name)
}

func (db *DistributedDatabase) GetIndex(collection, name string) *stor.Index {
	return db.storage.GetIndex(collection, name)
}

func (db *DistributedDatabase) GetIndexesForCollection(collection string) []*stor.Index {
	return db.storage.GetIndexesForCollection(collection)
}

func (db *DistributedDatabase) QueryIndex(collection, indexName string, query map[string]interface{}) ([]string, error) {
	return db.storage.QueryIndex(collection, indexName, query)
}

func (db *DistributedDatabase) Shutdown() error {
	if db.network == nil {
		return nil
	}
	return db.network.Shutdown()
}
