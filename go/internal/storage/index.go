package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// IndexType represents the type of index
type IndexType string

const (
	IndexTypeBTree IndexType = "btree"
	IndexTypeGIN   IndexType = "gin"
	IndexTypeHNSW  IndexType = "hnsw"
)

// Index represents a secondary index
type Index struct {
	Name        string                 `json:"name"`
	Collection  string                 `json:"collection"`
	Type        IndexType              `json:"type"`
	Fields      []string               `json:"fields"`
	Unique      bool                   `json:"unique"`
	PartialExpr string                 `json:"partial_expr,omitempty"`
	Options     map[string]interface{} `json:"options,omitempty"`

	// In-memory data structures
	btreeIndex *BTreeIndex
	ginIndex   *GINIndex
	hnswIndex  *HNSWIndex

	mu sync.RWMutex
}

// BTreeIndex for B-Tree indexes
type BTreeIndex struct {
	data map[string][]string // value -> []documentID
}

// GINIndex for Generalized Inverted Index (JSONB)
type GINIndex struct {
	data map[string][]string // token -> []documentID
}

// HNSWIndex for Hierarchical Navigable Small World (vectors)
type HNSWIndex struct {
	dimensions int
	vectors    map[string][]float64 // documentID -> vector
	neighbors  map[string][]string  // documentID -> []neighborIDs
}

// IndexManager manages all indexes for a storage backend
type IndexManager struct {
	baseDir string
	indexes map[string]*Index // collection:indexName -> index
	mu      sync.RWMutex
}

// NewIndexManager creates a new index manager
func NewIndexManager(baseDir string) *IndexManager {
	return &IndexManager{
		baseDir: baseDir,
		indexes: make(map[string]*Index),
	}
}

// CreateIndex creates a new index
func (im *IndexManager) CreateIndex(collection, name string, indexType IndexType, fields []string, unique bool, partialExpr string, options map[string]interface{}) error {
	key := fmt.Sprintf("%s:%s", collection, name)

	im.mu.Lock()
	defer im.mu.Unlock()

	if _, exists := im.indexes[key]; exists {
		return fmt.Errorf("index %s already exists", key)
	}

	index := &Index{
		Name:        name,
		Collection:  collection,
		Type:        indexType,
		Fields:      fields,
		Unique:      unique,
		PartialExpr: partialExpr,
		Options:     options,
	}

	// Initialize index data structure
	switch indexType {
	case IndexTypeBTree:
		index.btreeIndex = &BTreeIndex{data: make(map[string][]string)}
	case IndexTypeGIN:
		index.ginIndex = &GINIndex{data: make(map[string][]string)}
	case IndexTypeHNSW:
		dim := 768 // default
		if d, ok := options["dimensions"].(float64); ok {
			dim = int(d)
		}
		index.hnswIndex = &HNSWIndex{
			dimensions: dim,
			vectors:    make(map[string][]float64),
			neighbors:  make(map[string][]string),
		}
	}

	im.indexes[key] = index

	// Save index metadata
	return im.saveIndexMetadata(index)
}

// DropIndex removes an index
func (im *IndexManager) DropIndex(collection, name string) error {
	key := fmt.Sprintf("%s:%s", collection, name)

	im.mu.Lock()
	defer im.mu.Unlock()

	if _, exists := im.indexes[key]; !exists {
		return fmt.Errorf("index %s does not exist", key)
	}

	delete(im.indexes, key)

	// Remove index files
	indexDir := filepath.Join(im.baseDir, collection, "indexes", name)
	os.RemoveAll(indexDir)

	return nil
}

// GetIndex returns an index by collection and name
func (im *IndexManager) GetIndex(collection, name string) *Index {
	key := fmt.Sprintf("%s:%s", collection, name)

	im.mu.RLock()
	defer im.mu.RUnlock()

	return im.indexes[key]
}

// GetIndexesForCollection returns all indexes for a collection
func (im *IndexManager) GetIndexesForCollection(collection string) []*Index {
	im.mu.RLock()
	defer im.mu.RUnlock()

	var indexes []*Index
	for _, idx := range im.indexes {
		if idx.Collection == collection {
			indexes = append(indexes, idx)
		}
	}
	return indexes
}

// Insert updates all relevant indexes for a document
func (im *IndexManager) Insert(collection string, doc map[string]interface{}) error {
	indexes := im.GetIndexesForCollection(collection)

	for _, idx := range indexes {
		if !im.matchesPartial(idx, doc) {
			continue
		}

		docID := doc["id"].(string)

		switch idx.Type {
		case IndexTypeBTree:
			im.insertBTree(idx, docID, doc)
		case IndexTypeGIN:
			im.insertGIN(idx, docID, doc)
		case IndexTypeHNSW:
			im.insertHNSW(idx, docID, doc)
		}
	}

	return nil
}

// Delete removes document from all relevant indexes
func (im *IndexManager) Delete(collection, docID string) error {
	indexes := im.GetIndexesForCollection(collection)

	for _, idx := range indexes {
		switch idx.Type {
		case IndexTypeBTree:
			im.deleteBTree(idx, docID)
		case IndexTypeGIN:
			im.deleteGIN(idx, docID)
		case IndexTypeHNSW:
			im.deleteHNSW(idx, docID)
		}
	}

	return nil
}

// QueryIndex performs an index-based query
func (im *IndexManager) QueryIndex(collection, indexName string, query map[string]interface{}) ([]string, error) {
	idx := im.GetIndex(collection, indexName)
	if idx == nil {
		return nil, fmt.Errorf("index %s:%s not found", collection, indexName)
	}

	switch idx.Type {
	case IndexTypeBTree:
		return im.queryBTree(idx, query)
	case IndexTypeGIN:
		return im.queryGIN(idx, query)
	case IndexTypeHNSW:
		return im.queryHNSW(idx, query)
	default:
		return nil, fmt.Errorf("unsupported index type: %s", idx.Type)
	}
}

// matchesPartial checks if document matches partial index expression
func (im *IndexManager) matchesPartial(idx *Index, doc map[string]interface{}) bool {
	if idx.PartialExpr == "" {
		return true
	}

	// Simple partial expression evaluation (can be extended)
	// For now, support basic field=value expressions
	parts := strings.Split(idx.PartialExpr, "=")
	if len(parts) != 2 {
		return true
	}

	field := strings.TrimSpace(parts[0])
	expected := strings.TrimSpace(parts[1])

	if val, ok := doc[field]; ok {
		return fmt.Sprintf("%v", val) == expected
	}

	return false
}

// B-Tree index operations
func (im *IndexManager) insertBTree(idx *Index, docID string, doc map[string]interface{}) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Create composite key from fields
	key := im.buildCompositeKey(idx.Fields, doc)

	if idx.Unique {
		// Check uniqueness
		if existing, exists := idx.btreeIndex.data[key]; exists && len(existing) > 0 {
			// For unique indexes, only one document per key
			return
		}
	}

	idx.btreeIndex.data[key] = append(idx.btreeIndex.data[key], docID)
}

func (im *IndexManager) deleteBTree(idx *Index, docID string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	for key, docIDs := range idx.btreeIndex.data {
		for i, id := range docIDs {
			if id == docID {
				idx.btreeIndex.data[key] = append(docIDs[:i], docIDs[i+1:]...)
				if len(idx.btreeIndex.data[key]) == 0 {
					delete(idx.btreeIndex.data, key)
				}
				break
			}
		}
	}
}

func (im *IndexManager) queryBTree(idx *Index, query map[string]interface{}) ([]string, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// For now, support exact match queries
	if value, ok := query["value"]; ok {
		key := fmt.Sprintf("%v", value)
		if docIDs, exists := idx.btreeIndex.data[key]; exists {
			return docIDs, nil
		}
	}

	return []string{}, nil
}

// GIN index operations (simplified)
func (im *IndexManager) insertGIN(idx *Index, docID string, doc map[string]interface{}) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Tokenize JSON fields
	tokens := im.tokenizeJSON(doc)
	for _, token := range tokens {
		idx.ginIndex.data[token] = append(idx.ginIndex.data[token], docID)
	}
}

func (im *IndexManager) deleteGIN(idx *Index, docID string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	for token, docIDs := range idx.ginIndex.data {
		for i, id := range docIDs {
			if id == docID {
				idx.ginIndex.data[token] = append(docIDs[:i], docIDs[i+1:]...)
				if len(idx.ginIndex.data[token]) == 0 {
					delete(idx.ginIndex.data, token)
				}
				break
			}
		}
	}
}

func (im *IndexManager) queryGIN(idx *Index, query map[string]interface{}) ([]string, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if token, ok := query["token"].(string); ok {
		if docIDs, exists := idx.ginIndex.data[token]; exists {
			return docIDs, nil
		}
	}

	return []string{}, nil
}

// HNSW index operations (simplified)
func (im *IndexManager) insertHNSW(idx *Index, docID string, doc map[string]interface{}) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Extract vector from document
	if payload, ok := doc["payload"].(map[string]interface{}); ok {
		if vector, ok := payload["vector"].([]interface{}); ok {
			vec := make([]float64, len(vector))
			for i, v := range vector {
				if f, ok := v.(float64); ok {
					vec[i] = f
				}
			}
			idx.hnswIndex.vectors[docID] = vec
			// Simplified: add to neighbors (in practice, use proper HNSW algorithm)
			idx.hnswIndex.neighbors[docID] = []string{}
		}
	}
}

func (im *IndexManager) deleteHNSW(idx *Index, docID string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	delete(idx.hnswIndex.vectors, docID)
	delete(idx.hnswIndex.neighbors, docID)
}

func (im *IndexManager) queryHNSW(idx *Index, query map[string]interface{}) ([]string, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if queryVec, ok := query["vector"].([]float64); ok {
		// Simplified cosine similarity search
		type scoredDoc struct {
			id    string
			score float64
		}

		var results []scoredDoc
		for docID, vec := range idx.hnswIndex.vectors {
			score := cosineSimilarity(queryVec, vec)
			results = append(results, scoredDoc{id: docID, score: score})
		}

		// Sort by score descending
		sort.Slice(results, func(i, j int) bool {
			return results[i].score > results[j].score
		})

		// Return top results
		limit := 10
		if l, ok := query["limit"].(int); ok {
			limit = l
		}

		var docIDs []string
		for i, result := range results {
			if i >= limit {
				break
			}
			docIDs = append(docIDs, result.id)
		}

		return docIDs, nil
	}

	return []string{}, nil
}

// Helper functions

func (im *IndexManager) buildCompositeKey(fields []string, doc map[string]interface{}) string {
	var parts []string
	for _, field := range fields {
		if val, ok := doc[field]; ok {
			parts = append(parts, fmt.Sprintf("%v", val))
		}
	}
	return strings.Join(parts, "|")
}

func (im *IndexManager) tokenizeJSON(doc map[string]interface{}) []string {
	var tokens []string
	im.tokenizeValue(doc, &tokens)
	return tokens
}

func (im *IndexManager) tokenizeValue(val interface{}, tokens *[]string) {
	switch v := val.(type) {
	case string:
		// Simple tokenization: split by spaces and lowercase
		words := strings.Fields(strings.ToLower(v))
		*tokens = append(*tokens, words...)
	case map[string]interface{}:
		for _, value := range v {
			im.tokenizeValue(value, tokens)
		}
	case []interface{}:
		for _, item := range v {
			im.tokenizeValue(item, tokens)
		}
	}
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (normA * normB)
}

func (im *IndexManager) saveIndexMetadata(idx *Index) error {
	indexDir := filepath.Join(im.baseDir, idx.Collection, "indexes", idx.Name)
	os.MkdirAll(indexDir, 0755)

	metaPath := filepath.Join(indexDir, "metadata.json")
	data, err := json.Marshal(idx)
	if err != nil {
		return err
	}

	return os.WriteFile(metaPath, data, 0644)
}

// LoadIndexes loads all index metadata from disk
func (im *IndexManager) LoadIndexes() error {
	collectionsDir := im.baseDir

	entries, err := os.ReadDir(collectionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		collection := entry.Name()
		indexesDir := filepath.Join(collectionsDir, collection, "indexes")

		if _, err := os.Stat(indexesDir); os.IsNotExist(err) {
			continue
		}

		indexEntries, err := os.ReadDir(indexesDir)
		if err != nil {
			continue
		}

		for _, idxEntry := range indexEntries {
			if !idxEntry.IsDir() {
				continue
			}

			indexName := idxEntry.Name()
			metaPath := filepath.Join(indexesDir, indexName, "metadata.json")

			data, err := os.ReadFile(metaPath)
			if err != nil {
				continue
			}

			var idx Index
			if err := json.Unmarshal(data, &idx); err != nil {
				continue
			}

			// Reinitialize data structures
			switch idx.Type {
			case IndexTypeBTree:
				idx.btreeIndex = &BTreeIndex{data: make(map[string][]string)}
			case IndexTypeGIN:
				idx.ginIndex = &GINIndex{data: make(map[string][]string)}
			case IndexTypeHNSW:
				dim := 768
				if d, ok := idx.Options["dimensions"].(float64); ok {
					dim = int(d)
				}
				idx.hnswIndex = &HNSWIndex{
					dimensions: dim,
					vectors:    make(map[string][]float64),
					neighbors:  make(map[string][]string),
				}
			}

			key := fmt.Sprintf("%s:%s", collection, indexName)
			im.indexes[key] = &idx
		}
	}

	return nil
}
