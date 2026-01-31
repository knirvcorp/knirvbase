package indexing

import (
	"math/rand"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestDebugClusterSearch(t *testing.T) {
	testCases := []struct {
		name              string
		dimension         int
		M                 int
		efConstruction    int
		cluster1Vectors   int
		cluster2Vectors   int
		query             []float32
		expectedCluster1  int
	}{
		{
			name:              "2 clusters, query near cluster 1",
			dimension:         10,
			M:                 16,
			efConstruction:    200,
			cluster1Vectors:   20,
			cluster2Vectors:   20,
			query:             []float32{0.015, 0.015, 0.015, 0.015, 0.015, 0.015, 0.015, 0.015, 0.015, 0.015},
			expectedCluster1:  4, // Should find most from cluster 1
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create index with parameters
			index := NewHNSWIndex(tc.dimension, tc.M, tc.efConstruction)

			cluster1IDs := make([]uuid.UUID, 0)
			cluster2IDs := make([]uuid.UUID, 0)

			// Add cluster 1 vectors
			for i := 0; i < tc.cluster1Vectors; i++ {
				id := uuid.New()
				vector := make([]float32, tc.dimension)
				for j := range vector {
					vector[j] = 0.01 + rand.Float32()*0.01 // 0.01 to 0.02
				}
				index.Add(id, vector)
				cluster1IDs = append(cluster1IDs, id)
			}

			// Add cluster 2 vectors
			for i := 0; i < tc.cluster2Vectors; i++ {
				id := uuid.New()
				vector := make([]float32, tc.dimension)
				for j := range vector {
					vector[j] = 10.0 + rand.Float32() // 10.0 to 11.0
				}
				index.Add(id, vector)
				cluster2IDs = append(cluster2IDs, id)
			}

			// Search and count results from each cluster
			results, err := index.Search(tc.query, 5)
			assert.NoError(t, err)
			assert.Len(t, results, 5)

			cluster1Count := 0
			for _, id := range results {
				for _, c1ID := range cluster1IDs {
					if id == c1ID {
						cluster1Count++
						break
					}
				}
			}

			assert.GreaterOrEqual(t, cluster1Count, tc.expectedCluster1,
				"Expected at least %d cluster 1 results, got %d", tc.expectedCluster1, cluster1Count)
		})
	}
}
