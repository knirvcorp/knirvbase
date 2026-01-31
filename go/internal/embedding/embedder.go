package embedding

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Embedder defines the interface for generating semantic embeddings
type Embedder interface {
	// Generate creates a semantic embedding vector from text
	Generate(ctx context.Context, text string) ([]float32, error)

	// Fit trains the embedder on a corpus of documents
	Fit(ctx context.Context, documents []string) error

	// FitIncremental updates the embedder with a new document
	FitIncremental(ctx context.Context, document string) error

	// Dimension returns the dimensionality of the embeddings
	Dimension() int
}

// Storage interface for persisting vocabulary
type Storage interface {
	Put(key, value []byte) error
	Get(key []byte) ([]byte, error)
	Has(key []byte) (bool, error)
}

// TFIDFEmbedder implements the Embedder interface using TF-IDF + LSA
type TFIDFEmbedder struct {
	vectorizer *TFIDFVectorizer
	reducer    *LSAReducer
	storage    Storage
	dimension  int
	mu         sync.RWMutex
	fitted     bool
}

// NewTFIDFEmbedder creates a new TF-IDF based embedder
func NewTFIDFEmbedder(storage Storage, dimension int) (*TFIDFEmbedder, error) {
	if dimension <= 0 {
		return nil, fmt.Errorf("dimension must be positive, got %d", dimension)
	}

	embedder := &TFIDFEmbedder{
		vectorizer: NewTFIDFVectorizer(),
		reducer:    NewLSAReducer(dimension),
		storage:    storage,
		dimension:  dimension,
		fitted:     false,
	}

	// Try to load existing vocabulary from storage
	if err := embedder.loadVocabulary(); err == nil {
		embedder.fitted = true
	}

	return embedder, nil
}

// Dimension returns the dimensionality of the embeddings
func (e *TFIDFEmbedder) Dimension() int {
	return e.dimension
}

// Fit trains the embedder on a corpus of documents
func (e *TFIDFEmbedder) Fit(ctx context.Context, documents []string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(documents) == 0 {
		return fmt.Errorf("no documents provided for training")
	}

	// Train TF-IDF vectorizer
	if err := e.vectorizer.Fit(documents); err != nil {
		return fmt.Errorf("failed to fit TF-IDF: %w", err)
	}

	// Generate TF-IDF vectors
	vectors, err := e.vectorizer.FitTransform(documents)
	if err != nil {
		return fmt.Errorf("failed to transform documents: %w", err)
	}

	// Train LSA reducer if we have enough data
	if len(vectors) > 0 && len(vectors[0]) >= e.dimension {
		if err := e.reducer.Fit(vectors); err != nil {
			return fmt.Errorf("failed to fit LSA: %w", err)
		}
	}

	// Save vocabulary to storage
	if err := e.saveVocabulary(); err != nil {
		return fmt.Errorf("failed to save vocabulary: %w", err)
	}

	e.fitted = true
	return nil
}

// FitIncremental updates the embedder with a new document
func (e *TFIDFEmbedder) FitIncremental(ctx context.Context, document string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if document == "" {
		return fmt.Errorf("empty document provided")
	}

	// Update TF-IDF vectorizer incrementally
	e.vectorizer.FitIncremental(document)

	// Save updated vocabulary
	if err := e.saveVocabulary(); err != nil {
		return fmt.Errorf("failed to save vocabulary: %w", err)
	}

	e.fitted = true
	return nil
}

// Generate creates a semantic embedding vector from text
func (e *TFIDFEmbedder) Generate(ctx context.Context, text string) ([]float32, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// If not fitted, return a zero vector
	if !e.fitted {
		return make([]float32, e.dimension), nil
	}

	// Generate TF-IDF vector
	tfidfVector, err := e.vectorizer.Transform(text)
	if err != nil {
		return nil, fmt.Errorf("failed to transform text: %w", err)
	}

	// If we don't have enough dimensions or LSA isn't fitted, normalize TF-IDF vector
	if len(tfidfVector) < e.dimension || e.reducer.sourceDim == 0 {
		return e.normalizeToTarget(tfidfVector), nil
	}

	// Apply LSA dimensionality reduction
	reduced, err := e.reducer.Transform(tfidfVector)
	if err != nil {
		return nil, fmt.Errorf("failed to apply LSA: %w", err)
	}

	// Convert float64 to float32
	result := make([]float32, len(reduced))
	for i, val := range reduced {
		result[i] = float32(val)
	}

	return result, nil
}

// normalizeToTarget pads or truncates a vector to match target dimension
func (e *TFIDFEmbedder) normalizeToTarget(vector []float64) []float32 {
	result := make([]float32, e.dimension)

	// Copy available values
	copyLen := len(vector)
	if copyLen > e.dimension {
		copyLen = e.dimension
	}

	for i := 0; i < copyLen; i++ {
		result[i] = float32(vector[i])
	}

	// Remaining values stay as zero
	return result
}

// saveVocabulary persists the vocabulary to storage
func (e *TFIDFEmbedder) saveVocabulary() error {
	if e.storage == nil {
		return nil // No storage configured
	}

	// Serialize vocabulary
	vocabData, err := json.Marshal(e.vectorizer.vocabulary)
	if err != nil {
		return fmt.Errorf("failed to marshal vocabulary: %w", err)
	}

	// Serialize IDF values
	idfData, err := json.Marshal(e.vectorizer.idf)
	if err != nil {
		return fmt.Errorf("failed to marshal IDF: %w", err)
	}

	// Serialize word-document counts
	wordDocCountsData, err := json.Marshal(e.vectorizer.wordDocCounts)
	if err != nil {
		return fmt.Errorf("failed to marshal word doc counts: %w", err)
	}

	// Serialize document count
	docCountData, err := json.Marshal(e.vectorizer.docCount)
	if err != nil {
		return fmt.Errorf("failed to marshal doc count: %w", err)
	}

	// Store in database
	if err := e.storage.Put([]byte("embedding:vocabulary"), vocabData); err != nil {
		return err
	}
	if err := e.storage.Put([]byte("embedding:idf"), idfData); err != nil {
		return err
	}
	if err := e.storage.Put([]byte("embedding:word_doc_counts"), wordDocCountsData); err != nil {
		return err
	}
	if err := e.storage.Put([]byte("embedding:doc_count"), docCountData); err != nil {
		return err
	}

	return nil
}

// loadVocabulary restores the vocabulary from storage
func (e *TFIDFEmbedder) loadVocabulary() error {
	if e.storage == nil {
		return fmt.Errorf("no storage configured")
	}

	// Check if vocabulary exists
	exists, err := e.storage.Has([]byte("embedding:vocabulary"))
	if err != nil || !exists {
		return fmt.Errorf("vocabulary not found in storage")
	}

	// Load vocabulary
	vocabData, err := e.storage.Get([]byte("embedding:vocabulary"))
	if err != nil {
		return fmt.Errorf("failed to load vocabulary: %w", err)
	}
	if err := json.Unmarshal(vocabData, &e.vectorizer.vocabulary); err != nil {
		return fmt.Errorf("failed to unmarshal vocabulary: %w", err)
	}

	// Load IDF values
	idfData, err := e.storage.Get([]byte("embedding:idf"))
	if err != nil {
		return fmt.Errorf("failed to load IDF: %w", err)
	}
	if err := json.Unmarshal(idfData, &e.vectorizer.idf); err != nil {
		return fmt.Errorf("failed to unmarshal IDF: %w", err)
	}

	// Load word-document counts
	wordDocCountsData, err := e.storage.Get([]byte("embedding:word_doc_counts"))
	if err != nil {
		return fmt.Errorf("failed to load word doc counts: %w", err)
	}
	if err := json.Unmarshal(wordDocCountsData, &e.vectorizer.wordDocCounts); err != nil {
		return fmt.Errorf("failed to unmarshal word doc counts: %w", err)
	}

	// Load document count
	docCountData, err := e.storage.Get([]byte("embedding:doc_count"))
	if err != nil {
		return fmt.Errorf("failed to load doc count: %w", err)
	}
	if err := json.Unmarshal(docCountData, &e.vectorizer.docCount); err != nil {
		return fmt.Errorf("failed to unmarshal doc count: %w", err)
	}

	return nil
}
