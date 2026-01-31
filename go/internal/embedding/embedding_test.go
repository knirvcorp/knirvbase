package embedding

import (
	"context"
	"math"
	"path/filepath"
	"testing"

	"github.com/knirvcorp/knirvbase/go/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock storage for testing without KNIRVBASE dependency
type mockStorage struct {
	data map[string][]byte
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		data: make(map[string][]byte),
	}
}

func (m *mockStorage) Put(key, value []byte) error {
	m.data[string(key)] = value
	return nil
}

func (m *mockStorage) Get(key []byte) ([]byte, error) {
	val, ok := m.data[string(key)]
	if !ok {
		return nil, assert.AnError
	}
	return val, nil
}

func (m *mockStorage) Has(key []byte) (bool, error) {
	_, ok := m.data[string(key)]
	return ok, nil
}

// TestTokenize tests the tokenization function
func TestTokenize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple text",
			input:    "hello world",
			expected: []string{"hello", "world"},
		},
		{
			name:     "mixed case",
			input:    "Hello WORLD",
			expected: []string{"hello", "world"},
		},
		{
			name:     "with punctuation",
			input:    "Hello, world! How are you?",
			expected: []string{"hello", "world", "how", "are", "you"},
		},
		{
			name:     "single char filtered",
			input:    "a b cd ef",
			expected: []string{"cd", "ef"},
		},
		{
			name:     "numbers included",
			input:    "test123 code456",
			expected: []string{"test123", "code456"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tokenize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestTFIDFVectorizer tests the TF-IDF vectorizer
func TestTFIDFVectorizer(t *testing.T) {
	vectorizer := NewTFIDFVectorizer()

	documents := []string{
		"the quick brown fox",
		"the lazy dog",
		"the fox and the dog",
	}

	err := vectorizer.Fit(documents)
	require.NoError(t, err)

	// Check vocabulary size
	vocabSize := vectorizer.VocabularySize()
	assert.Greater(t, vocabSize, 0)
	assert.LessOrEqual(t, vocabSize, 10) // Should have distinct words

	// Transform a document
	vector, err := vectorizer.Transform("the quick fox")
	require.NoError(t, err)
	assert.Equal(t, vocabSize, len(vector))

	// Non-zero for words in vocabulary
	hasNonZero := false
	for _, val := range vector {
		if val > 0 {
			hasNonZero = true
			break
		}
	}
	assert.True(t, hasNonZero, "Expected non-zero TF-IDF values")
}

// TestTFIDFVectorizer_FitTransform tests the combined fit and transform
func TestTFIDFVectorizer_FitTransform(t *testing.T) {
	vectorizer := NewTFIDFVectorizer()

	documents := []string{
		"machine learning is great",
		"deep learning is powerful",
		"learning is important",
	}

	vectors, err := vectorizer.FitTransform(documents)
	require.NoError(t, err)
	assert.Equal(t, len(documents), len(vectors))

	// All vectors should have same length
	for i := 1; i < len(vectors); i++ {
		assert.Equal(t, len(vectors[0]), len(vectors[i]))
	}
}

// TestTFIDFVectorizer_Incremental tests incremental vocabulary updates
func TestTFIDFVectorizer_Incremental(t *testing.T) {
	vectorizer := NewTFIDFVectorizer()

	// Initial fit
	documents := []string{
		"initial document one",
		"initial document two",
	}
	err := vectorizer.Fit(documents)
	require.NoError(t, err)

	initialSize := vectorizer.VocabularySize()

	// Incremental update with new words
	vectorizer.FitIncremental("new words here")

	newSize := vectorizer.VocabularySize()
	assert.Greater(t, newSize, initialSize, "Vocabulary should grow with new words")
}

// TestLSAReducer tests the LSA dimensionality reduction
func TestLSAReducer(t *testing.T) {
	reducer := NewLSAReducer(5) // Reduce to 5 dimensions

	// Create sample vectors (10 vectors of 20 dimensions)
	vectors := make([][]float64, 10)
	for i := range vectors {
		vectors[i] = make([]float64, 20)
		for j := range vectors[i] {
			vectors[i][j] = float64(i + j)
		}
	}

	// Fit the reducer
	err := reducer.Fit(vectors)
	require.NoError(t, err)

	// Transform a vector
	testVector := make([]float64, 20)
	for i := range testVector {
		testVector[i] = float64(i)
	}

	reduced, err := reducer.Transform(testVector)
	require.NoError(t, err)
	assert.Equal(t, 5, len(reduced), "Should reduce to target dimension")
}

// TestLSAReducer_FitTransform tests the combined fit and transform
func TestLSAReducer_FitTransform(t *testing.T) {
	reducer := NewLSAReducer(3)

	vectors := [][]float64{
		{1.0, 2.0, 3.0, 4.0, 5.0},
		{2.0, 3.0, 4.0, 5.0, 6.0},
		{3.0, 4.0, 5.0, 6.0, 7.0},
	}

	reduced, err := reducer.FitTransform(vectors)
	require.NoError(t, err)
	assert.Equal(t, len(vectors), len(reduced))

	// All reduced vectors should have target dimension
	for _, vec := range reduced {
		assert.Equal(t, 3, len(vec))
	}
}

// TestDotProduct tests the dot product utility function
func TestDotProduct(t *testing.T) {
	a := []float64{1.0, 2.0, 3.0}
	b := []float64{4.0, 5.0, 6.0}

	result := dotProduct(a, b)
	expected := 1.0*4.0 + 2.0*5.0 + 3.0*6.0
	assert.InDelta(t, expected, result, 0.001)

	// Different lengths
	c := []float64{1.0, 2.0}
	result = dotProduct(a, c)
	assert.Equal(t, 0.0, result)
}

// TestVectorNorm tests the vector norm utility function
func TestVectorNorm(t *testing.T) {
	v := []float64{3.0, 4.0}
	norm := vectorNorm(v)
	assert.InDelta(t, 5.0, norm, 0.001) // 3-4-5 triangle

	// Zero vector
	zero := []float64{0.0, 0.0}
	norm = vectorNorm(zero)
	assert.Equal(t, 0.0, norm)
}

// TestNormalizeVector tests vector normalization
func TestNormalizeVector(t *testing.T) {
	v := []float64{3.0, 4.0}
	normalized := normalizeVector(v)

	// Should have unit length
	norm := vectorNorm(normalized)
	assert.InDelta(t, 1.0, norm, 0.001)

	// Direction preserved
	assert.InDelta(t, 0.6, normalized[0], 0.001)
	assert.InDelta(t, 0.8, normalized[1], 0.001)
}

// TestTFIDFEmbedder_Creation tests embedder creation
func TestTFIDFEmbedder_Creation(t *testing.T) {
	stor := newMockStorage()

	embedder, err := NewTFIDFEmbedder(stor, 768)
	require.NoError(t, err)
	assert.NotNil(t, embedder)
	assert.Equal(t, 768, embedder.Dimension())

	// Invalid dimension
	_, err = NewTFIDFEmbedder(stor, 0)
	assert.Error(t, err)

	_, err = NewTFIDFEmbedder(stor, -10)
	assert.Error(t, err)
}

// TestTFIDFEmbedder_Fit tests training the embedder
func TestTFIDFEmbedder_Fit(t *testing.T) {
	stor := newMockStorage()
	embedder, err := NewTFIDFEmbedder(stor, 10)
	require.NoError(t, err)

	ctx := context.Background()
	documents := []string{
		"artificial intelligence is fascinating",
		"machine learning transforms data",
		"deep learning uses neural networks",
	}

	err = embedder.Fit(ctx, documents)
	require.NoError(t, err)
	assert.True(t, embedder.fitted)

	// Verify vocabulary was saved to storage
	has, err := stor.Has([]byte("embedding:vocabulary"))
	require.NoError(t, err)
	assert.True(t, has)
}

// TestTFIDFEmbedder_Generate tests embedding generation
func TestTFIDFEmbedder_Generate(t *testing.T) {
	stor := newMockStorage()
	embedder, err := NewTFIDFEmbedder(stor, 768)
	require.NoError(t, err)

	ctx := context.Background()

	// Before fitting, should return zero vector
	vector, err := embedder.Generate(ctx, "test text")
	require.NoError(t, err)
	assert.Equal(t, 768, len(vector))

	// After fitting
	documents := []string{
		"blockchain technology enables trust",
		"distributed systems are complex",
		"consensus algorithms ensure agreement",
	}

	err = embedder.Fit(ctx, documents)
	require.NoError(t, err)

	// Generate embedding
	vector, err = embedder.Generate(ctx, "blockchain consensus")
	require.NoError(t, err)
	assert.Equal(t, 768, len(vector))

	// Should have some non-zero values for words in vocabulary
	hasNonZero := false
	for _, val := range vector {
		if val != 0 {
			hasNonZero = true
			break
		}
	}
	assert.True(t, hasNonZero, "Expected non-zero embedding values")
}

// TestTFIDFEmbedder_FitIncremental tests incremental updates
func TestTFIDFEmbedder_FitIncremental(t *testing.T) {
	stor := newMockStorage()
	embedder, err := NewTFIDFEmbedder(stor, 100)
	require.NoError(t, err)

	ctx := context.Background()

	// Initial fit
	documents := []string{
		"initial training data",
		"more training examples",
	}
	err = embedder.Fit(ctx, documents)
	require.NoError(t, err)

	// Incremental update
	err = embedder.FitIncremental(ctx, "new additional data")
	require.NoError(t, err)
	assert.True(t, embedder.fitted)

	// Empty document should error
	err = embedder.FitIncremental(ctx, "")
	assert.Error(t, err)
}

// TestTFIDFEmbedder_VocabularyPersistence tests vocabulary save/load
func TestTFIDFEmbedder_VocabularyPersistence(t *testing.T) {
	stor := newMockStorage()

	// Create and train first embedder
	embedder1, err := NewTFIDFEmbedder(stor, 50)
	require.NoError(t, err)

	ctx := context.Background()
	documents := []string{
		"persistent vocabulary test",
		"storage and retrieval",
	}

	err = embedder1.Fit(ctx, documents)
	require.NoError(t, err)

	// Generate embedding with first embedder
	vector1, err := embedder1.Generate(ctx, "persistent test")
	require.NoError(t, err)

	// Create second embedder with same storage
	embedder2, err := NewTFIDFEmbedder(stor, 50)
	require.NoError(t, err)

	// Should load vocabulary automatically
	assert.True(t, embedder2.fitted)

	// Should generate similar embedding
	vector2, err := embedder2.Generate(ctx, "persistent test")
	require.NoError(t, err)

	// Vectors should match (loaded vocabulary)
	assert.Equal(t, vector1, vector2)
}

// TestTFIDFEmbedder_WithRealStorage tests with actual KNIRVBASE storage
func TestTFIDFEmbedder_WithRealStorage(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "testdb")

	stor := storage.NewFileStorage(dbPath)
	defer stor.Close()

	embedder, err := NewTFIDFEmbedder(stor, 768)
	require.NoError(t, err)

	ctx := context.Background()
	documents := []string{
		"KNIRVBASE stores AI memories",
		"KNIRVGRAPH manages knowledge",
		"KNIRVROUTER connects nodes",
	}

	err = embedder.Fit(ctx, documents)
	require.NoError(t, err)

	// Generate embeddings
	vector1, err := embedder.Generate(ctx, "AI knowledge storage")
	require.NoError(t, err)
	assert.Equal(t, 768, len(vector1))

	vector2, err := embedder.Generate(ctx, "node connections")
	require.NoError(t, err)
	assert.Equal(t, 768, len(vector2))

	// Different texts should have different embeddings
	assert.NotEqual(t, vector1, vector2)
}

// TestTFIDFEmbedder_SemanticSimilarity tests that similar texts have similar embeddings
func TestTFIDFEmbedder_SemanticSimilarity(t *testing.T) {
	stor := newMockStorage()
	embedder, err := NewTFIDFEmbedder(stor, 100)
	require.NoError(t, err)

	ctx := context.Background()
	documents := []string{
		"the cat sat on the mat",
		"the dog played in the park",
		"cats and dogs are pets",
		"mats and parks are places",
	}

	err = embedder.Fit(ctx, documents)
	require.NoError(t, err)

	// Similar texts
	vector1, err := embedder.Generate(ctx, "cat on mat")
	require.NoError(t, err)

	vector2, err := embedder.Generate(ctx, "the cat sat")
	require.NoError(t, err)

	// Different text
	vector3, err := embedder.Generate(ctx, "dog in park")
	require.NoError(t, err)

	// Calculate cosine similarity
	sim12 := cosineSimilarityFloat32(vector1, vector2)
	sim13 := cosineSimilarityFloat32(vector1, vector3)

	// Similar texts should have higher similarity
	assert.Greater(t, sim12, sim13, "Similar texts should have higher cosine similarity")
}

// Helper function for cosine similarity of float32 vectors
func cosineSimilarityFloat32(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0.0
	}

	var dotProd, normA, normB float32
	for i := range a {
		dotProd += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	normA = float32(math.Sqrt(float64(normA)))
	normB = float32(math.Sqrt(float64(normB)))

	return dotProd / (normA * normB)
}

// Benchmark embedding generation
func BenchmarkTFIDFEmbedder_Generate(b *testing.B) {
	stor := newMockStorage()
	embedder, _ := NewTFIDFEmbedder(stor, 768)

	ctx := context.Background()
	documents := []string{
		"benchmark test document one",
		"benchmark test document two",
		"benchmark test document three",
	}

	embedder.Fit(ctx, documents)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = embedder.Generate(ctx, "benchmark test query")
	}
}
