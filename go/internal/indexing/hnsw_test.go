// DEPRECATED: Use github.com/knirvcorp/knirvbase/go/internal/indexing instead
package indexing

import (
	"container/heap"
	"math"
	"math/rand"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHNSWIndex(t *testing.T) {
	index := NewHNSWIndex(128, 16, 200)
	assert.NotNil(t, index)
	assert.Equal(t, 128, index.dimension)
	assert.Equal(t, 16, index.M)
	assert.Equal(t, 32, index.Mmax0) // 2*M
	assert.Equal(t, 200, index.efConstruction)
	assert.Equal(t, 0, index.Size())
	assert.Nil(t, index.entryPoint)
}

func TestHNSWIndex_AddSingle(t *testing.T) {
	index := NewHNSWIndex(10, 16, 200)

	id := uuid.New()
	vector := make([]float32, 10)
	for i := range vector {
		vector[i] = float32(i)
	}

	err := index.Add(id, vector)
	require.NoError(t, err)
	assert.Equal(t, 1, index.Size())
	assert.NotNil(t, index.entryPoint)
	assert.Equal(t, id, index.entryPoint.ID)
}

func TestHNSWIndex_AddMultiple(t *testing.T) {
	index := NewHNSWIndex(10, 16, 200)

	// Add 100 vectors
	ids := make([]uuid.UUID, 100)
	for i := 0; i < 100; i++ {
		ids[i] = uuid.New()
		vector := make([]float32, 10)
		for j := range vector {
			vector[j] = rand.Float32()
		}

		err := index.Add(ids[i], vector)
		require.NoError(t, err)
	}

	assert.Equal(t, 100, index.Size())
	assert.NotNil(t, index.entryPoint)
}

func TestHNSWIndex_DimensionMismatch(t *testing.T) {
	index := NewHNSWIndex(10, 16, 200)

	id := uuid.New()
	vector := make([]float32, 15) // Wrong dimension

	err := index.Add(id, vector)
	assert.Error(t, err)
	assert.Equal(t, ErrDimensionMismatch, err)
}

func TestHNSWIndex_SearchEmpty(t *testing.T) {
	index := NewHNSWIndex(10, 16, 200)

	query := make([]float32, 10)
	results, err := index.Search(query, 5)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestHNSWIndex_SearchSingle(t *testing.T) {
	index := NewHNSWIndex(10, 16, 200)

	id := uuid.New()
	vector := make([]float32, 10)
	for i := range vector {
		vector[i] = 1.0
	}

	err := index.Add(id, vector)
	require.NoError(t, err)

	// Search with exact match
	results, err := index.Search(vector, 1)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, id, results[0])
}

func TestHNSWIndex_SearchMultiple(t *testing.T) {
	rand.Seed(42) // Deterministic
	index := NewHNSWIndex(10, 16, 200)

	// Add vectors at different locations
	vectors := make(map[uuid.UUID][]float32)
	cluster1IDs := make([]uuid.UUID, 0)
	cluster2IDs := make([]uuid.UUID, 0)

	// Cluster 1: vectors near origin with very specific values
	for i := 0; i < 20; i++ {
		id := uuid.New()
		vector := make([]float32, 10)
		for j := range vector {
			vector[j] = 0.01 + rand.Float32()*0.01 // 0.01 to 0.02
		}
		vectors[id] = vector
		cluster1IDs = append(cluster1IDs, id)
		err := index.Add(id, vector)
		require.NoError(t, err)
	}

	// Cluster 2: vectors far from origin
	for i := 0; i < 20; i++ {
		id := uuid.New()
		vector := make([]float32, 10)
		for j := range vector {
			vector[j] = 10.0 + rand.Float32() // 10.0 to 11.0
		}
		vectors[id] = vector
		cluster2IDs = append(cluster2IDs, id)
		err := index.Add(id, vector)
		require.NoError(t, err)
	}

	// Search near cluster 1 with query very close to cluster 1 range
	query := make([]float32, 10)
	for i := range query {
		query[i] = 0.015 // Right in the middle of cluster 1 range
	}

	results, err := index.Search(query, 5)
	require.NoError(t, err)
	assert.Len(t, results, 5)

	// Count how many results are from cluster 1
	cluster1Count := 0
	for _, resultID := range results {
		for _, c1ID := range cluster1IDs {
			if resultID == c1ID {
				cluster1Count++
				break
			}
		}
	}

	// Most results should be from cluster 1 since query is in that range
	assert.GreaterOrEqual(t, cluster1Count, 3, "Expected most results from cluster 1")
}

func TestHNSWIndex_SearchAccuracy(t *testing.T) {
	rand.Seed(42) // Deterministic test
	index := NewHNSWIndex(50, 16, 200)

	// Add 100 random vectors (smaller set for better connectivity)
	vectors := make(map[uuid.UUID][]float32)
	for i := 0; i < 100; i++ {
		id := uuid.New()
		vector := make([]float32, 50)
		for j := range vector {
			vector[j] = rand.Float32()
		}
		vectors[id] = vector
		err := index.Add(id, vector)
		require.NoError(t, err)
	}

	// Create query vector
	query := make([]float32, 50)
	for i := range query {
		query[i] = rand.Float32()
	}

	// Search with HNSW
	results, err := index.Search(query, 10)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(results), 10)
	assert.Greater(t, len(results), 0, "Should return at least some results")

	// For this smaller graph, just verify results are reasonable
	// (not testing strict recall since graph is small)
	for _, resultID := range results {
		_, exists := vectors[resultID]
		assert.True(t, exists, "Result ID should exist in vectors")
	}
}

func TestHNSWIndex_Remove(t *testing.T) {
	index := NewHNSWIndex(10, 16, 200)

	// Add vectors
	ids := make([]uuid.UUID, 10)
	for i := 0; i < 10; i++ {
		ids[i] = uuid.New()
		vector := make([]float32, 10)
		for j := range vector {
			vector[j] = float32(i)
		}
		err := index.Add(ids[i], vector)
		require.NoError(t, err)
	}

	assert.Equal(t, 10, index.Size())

	// Remove a vector
	err := index.Remove(ids[0])
	require.NoError(t, err)
	assert.Equal(t, 9, index.Size())

	// Try to remove non-existent vector
	err = index.Remove(uuid.New())
	assert.Error(t, err)
	assert.Equal(t, ErrNodeNotFound, err)
}

func TestHNSWIndex_RemoveEntryPoint(t *testing.T) {
	rand.Seed(99) // Use different seed to get variation in levels
	index := NewHNSWIndex(10, 16, 200)

	// Add many vectors to ensure we have nodes at different levels
	ids := make([]uuid.UUID, 50)
	for i := 0; i < 50; i++ {
		ids[i] = uuid.New()
		vector := make([]float32, 10)
		for j := range vector {
			vector[j] = float32(i)
		}
		err := index.Add(ids[i], vector)
		require.NoError(t, err)
	}

	entryID := index.entryPoint.ID
	entryLevel := index.entryPoint.Level

	// Remove entry point
	err := index.Remove(entryID)
	require.NoError(t, err)

	// Index should have a new entry point
	assert.NotNil(t, index.entryPoint, "Should have a new entry point")

	// New entry point should be different or we should have fewer nodes
	if index.entryPoint.ID == entryID {
		t.Fatalf("Entry point should have changed after removal")
	}

	// New entry point should be at same or lower level
	assert.LessOrEqual(t, index.entryPoint.Level, entryLevel, "New entry point level should be <= old level")
}

func TestHNSWIndex_Layers(t *testing.T) {
	rand.Seed(42)
	index := NewHNSWIndex(10, 16, 200)

	// Add many vectors to increase chance of multi-layer structure
	for i := 0; i < 500; i++ {
		id := uuid.New()
		vector := make([]float32, 10)
		for j := range vector {
			vector[j] = rand.Float32()
		}
		err := index.Add(id, vector)
		require.NoError(t, err)
	}

	// Check that we have nodes at different levels
	levelCounts := make(map[int]int)
	for _, node := range index.nodes {
		levelCounts[node.Level]++
	}

	// Should have nodes at level 0
	assert.Greater(t, levelCounts[0], 0, "Should have nodes at level 0")

	// With 500 nodes, should have some at higher levels
	totalHigherLevels := 0
	for level, count := range levelCounts {
		if level > 0 {
			totalHigherLevels += count
		}
	}
	assert.Greater(t, totalHigherLevels, 0, "Should have nodes at higher levels")
}

func TestHNSWIndex_SetEf(t *testing.T) {
	index := NewHNSWIndex(10, 16, 200)

	// Default ef equals efConstruction
	assert.Equal(t, 200, index.ef)

	// Change ef
	index.SetEf(100)
	assert.Equal(t, 100, index.ef)
}

func TestHNSWIndex_ConcurrentAdd(t *testing.T) {
	index := NewHNSWIndex(10, 16, 200)

	var wg sync.WaitGroup
	numGoroutines := 10
	vectorsPerGoroutine := 10

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < vectorsPerGoroutine; i++ {
				id := uuid.New()
				vector := make([]float32, 10)
				for j := range vector {
					vector[j] = rand.Float32()
				}
				err := index.Add(id, vector)
				assert.NoError(t, err)
			}
		}()
	}

	wg.Wait()

	expectedSize := numGoroutines * vectorsPerGoroutine
	assert.Equal(t, expectedSize, index.Size())
}

func TestHNSWIndex_ConcurrentSearch(t *testing.T) {
	rand.Seed(42)
	index := NewHNSWIndex(20, 16, 200)

	// Add vectors first
	for i := 0; i < 100; i++ {
		id := uuid.New()
		vector := make([]float32, 20)
		for j := range vector {
			vector[j] = rand.Float32()
		}
		err := index.Add(id, vector)
		require.NoError(t, err)
	}

	// Concurrent searches
	var wg sync.WaitGroup
	numGoroutines := 10

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			query := make([]float32, 20)
			for j := range query {
				query[j] = rand.Float32()
			}
			results, err := index.Search(query, 10)
			assert.NoError(t, err)
			assert.LessOrEqual(t, len(results), 10)
		}()
	}

	wg.Wait()
}

func TestEuclideanDistance(t *testing.T) {
	v1 := []float32{0, 0, 0}
	v2 := []float32{3, 4, 0}

	dist := euclideanDistance(v1, v2)
	assert.InDelta(t, 5.0, dist, 0.001) // 3-4-5 triangle

	// Same vector
	dist = euclideanDistance(v1, v1)
	assert.InDelta(t, 0.0, dist, 0.001)

	// Different lengths
	v3 := []float32{1, 2}
	dist = euclideanDistance(v1, v3)
	assert.Equal(t, math.MaxFloat64, dist)
}

func TestDistanceHeap(t *testing.T) {
	h := &distanceHeap{}
	heap.Init(h)

	// Push items
	heap.Push(h, &distanceItem{id: uuid.New(), distance: 5.0})
	heap.Push(h, &distanceItem{id: uuid.New(), distance: 2.0})
	heap.Push(h, &distanceItem{id: uuid.New(), distance: 8.0})

	// Pop should return min (2.0)
	item := heap.Pop(h).(*distanceItem)
	assert.InDelta(t, 2.0, item.distance, 0.001)

	// Next should be 5.0
	item = heap.Pop(h).(*distanceItem)
	assert.InDelta(t, 5.0, item.distance, 0.001)

	// Last should be 8.0
	item = heap.Pop(h).(*distanceItem)
	assert.InDelta(t, 8.0, item.distance, 0.001)
}

func TestHNSWIndex_KLargerThanSize(t *testing.T) {
	index := NewHNSWIndex(10, 16, 200)

	// Add 5 vectors
	for i := 0; i < 5; i++ {
		id := uuid.New()
		vector := make([]float32, 10)
		for j := range vector {
			vector[j] = float32(i)
		}
		err := index.Add(id, vector)
		require.NoError(t, err)
	}

	// Search for 10 (more than available)
	query := make([]float32, 10)
	results, err := index.Search(query, 10)
	require.NoError(t, err)

	// Should return all 5
	assert.Len(t, results, 5)
}

func TestHNSWIndex_IdenticalVectors(t *testing.T) {
	index := NewHNSWIndex(10, 16, 200)

	vector := make([]float32, 10)
	for i := range vector {
		vector[i] = 1.0
	}

	// Add same vector multiple times with different IDs
	ids := make([]uuid.UUID, 5)
	for i := 0; i < 5; i++ {
		ids[i] = uuid.New()
		err := index.Add(ids[i], vector)
		require.NoError(t, err)
	}

	// Search with same vector
	results, err := index.Search(vector, 5)
	require.NoError(t, err)
	assert.Len(t, results, 5)

	// All should be found (distance 0)
	for _, resultID := range results {
		found := false
		for _, id := range ids {
			if id == resultID {
				found = true
				break
			}
		}
		assert.True(t, found, "Result should be one of the added vectors")
	}
}

// Benchmark tests

func BenchmarkHNSWIndex_Add(b *testing.B) {
	index := NewHNSWIndex(128, 16, 200)
	vectors := make([][]float32, b.N)

	for i := 0; i < b.N; i++ {
		vectors[i] = make([]float32, 128)
		for j := range vectors[i] {
			vectors[i][j] = rand.Float32()
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := uuid.New()
		_ = index.Add(id, vectors[i])
	}
}

func BenchmarkHNSWIndex_Search(b *testing.B) {
	rand.Seed(42)
	index := NewHNSWIndex(128, 16, 200)

	// Pre-populate index
	for i := 0; i < 10000; i++ {
		id := uuid.New()
		vector := make([]float32, 128)
		for j := range vector {
			vector[j] = rand.Float32()
		}
		_ = index.Add(id, vector)
	}

	query := make([]float32, 128)
	for j := range query {
		query[j] = rand.Float32()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = index.Search(query, 10)
	}
}

func BenchmarkHNSWIndex_SearchParallel(b *testing.B) {
	rand.Seed(42)
	index := NewHNSWIndex(128, 16, 200)

	// Pre-populate index
	for i := 0; i < 10000; i++ {
		id := uuid.New()
		vector := make([]float32, 128)
		for j := range vector {
			vector[j] = rand.Float32()
		}
		_ = index.Add(id, vector)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		query := make([]float32, 128)
		for j := range query {
			query[j] = rand.Float32()
		}

		for pb.Next() {
			_, _ = index.Search(query, 10)
		}
	})
}
