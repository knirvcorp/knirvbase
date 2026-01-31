package indexing

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/google/uuid"
)

func DebugHNSWIndex() {
	rand.Seed(42)

	index := NewHNSWIndex(10, 16, 200)
	cluster1IDs := make([]uuid.UUID, 0)
	cluster2IDs := make([]uuid.UUID, 0)

	// Cluster 1: vectors near origin (0.01-0.02)
	fmt.Println("Adding cluster 1 vectors (20 vectors)...")
	for i := 0; i < 20; i++ {
		id := uuid.New()
		vector := make([]float32, 10)
		for j := range vector {
			vector[j] = 0.01 + rand.Float32()*0.01 // 0.01 to 0.02
		}
		err := index.Add(id, vector)
		if err != nil {
			fmt.Printf("Error adding cluster 1 vector %d: %v\n", i, err)
			continue
		}
		cluster1IDs = append(cluster1IDs, id)
	}
	fmt.Printf("Cluster 1 count: %d\n", len(cluster1IDs))

	// Cluster 2: vectors far from origin (10.0-11.0)
	fmt.Println("\nAdding cluster 2 vectors (20 vectors)...")
	for i := 0; i < 20; i++ {
		id := uuid.New()
		vector := make([]float32, 10)
		for j := range vector {
			vector[j] = 10.0 + rand.Float32() // 10.0 to 11.0
		}
		err := index.Add(id, vector)
		if err != nil {
			fmt.Printf("Error adding cluster 2 vector %d: %v\n", i, err)
			continue
		}
		cluster2IDs = append(cluster2IDs, id)
	}
	fmt.Printf("Cluster 2 count: %d\n", len(cluster2IDs))

	// Find entry point
	fmt.Printf("\nEntry point ID: %s\n", index.EntryPoint().ID)

	// Check which cluster the entry point is in
	isCluster1 := false
	for _, c1ID := range cluster1IDs {
		if index.EntryPoint().ID == c1ID {
			isCluster1 = true
			break
		}
	}

	if isCluster1 {
		fmt.Println("Entry point is in Cluster 1")
	} else {
		fmt.Println("Entry point is in Cluster 2")
	}

	// Check levels of all nodes
	fmt.Printf("\nTotal nodes: %d\n", index.Size())
	cluster1LevelCounts := make(map[int]int)
	cluster2LevelCounts := make(map[int]int)

	allNodes := index.GetAllNodes()

	for _, nodeID := range allNodes {
		node := index.GetNode(nodeID)
		if node == nil {
			continue
		}

		isC1 := false
		for _, c1ID := range cluster1IDs {
			if nodeID == c1ID {
				isC1 = true
				cluster1LevelCounts[node.Level]++
				break
			}
		}
		if !isC1 {
			for _, c2ID := range cluster2IDs {
				if nodeID == c2ID {
					cluster2LevelCounts[node.Level]++
					break
				}
			}
		}
	}

	fmt.Println("\nLevel distribution (Cluster 1):")
	for level, count := range cluster1LevelCounts {
		fmt.Printf("Level %d: %d nodes\n", level, count)
	}

	fmt.Println("\nLevel distribution (Cluster 2):")
	for level, count := range cluster2LevelCounts {
		fmt.Printf("Level %d: %d nodes\n", level, count)
	}

	// Test search
	query := make([]float32, 10)
	for i := range query {
		query[i] = 0.015 // Center of cluster 1
	}
	fmt.Printf("\nSearching for query vector: %v\n", query)
	results, err := index.Search(query, 5)
	if err != nil {
		fmt.Printf("Search error: %v\n", err)
		return
	}

	fmt.Printf("Search results (top 5):\n")
	for i, id := range results {
		var cluster string
		isC1 := false
		for _, c1ID := range cluster1IDs {
			if id == c1ID {
				isC1 = true
				cluster = "Cluster 1"
				break
			}
		}
		if !isC1 {
			cluster = "Cluster 2"
		}
		node := index.GetNode(id)
		if node != nil {
			fmt.Printf("Result %d: ID=%s, Level=%d, Cluster=%s, Distance=%.3f\n",
				i, id, node.Level, cluster, EuclideanDistance(query, node.Vector))
		}
	}

	// Count cluster1 results
	cluster1Count := 0
	for _, id := range results {
		for _, c1ID := range cluster1IDs {
			if id == c1ID {
				cluster1Count++
			}
		}
	}
	fmt.Printf("\nCluster 1 results count: %d (expected >= 3)\n", cluster1Count)
}

// EuclideanDistance calculates the Euclidean distance between two vectors
func EuclideanDistance(v1, v2 []float32) float64 {
	if len(v1) != len(v2) {
		return math.MaxFloat64
	}

	var sum float64
	for i := range v1 {
		diff := float64(v1[i] - v2[i])
		sum += diff * diff
	}

	return math.Sqrt(sum)
}
