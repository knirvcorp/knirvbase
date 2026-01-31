// DEPRECATED: Use github.com/knirvcorp/knirvbase/go/internal/indexing instead
// This file exists for compatibility purposes only and will be removed in future versions
package indexing

import (
	"container/heap"
	"math"
	"math/rand"
	"sync"

	"github.com/google/uuid"
)

// HNSWNode represents a node in the HNSW graph
type HNSWNode struct {
	ID          uuid.UUID
	Vector      []float32
	Connections map[int][]uuid.UUID // layer -> list of connected node IDs
	Level       int                 // highest layer this node appears in
}

// HNSWIndex implements Hierarchical Navigable Small World graph for vector similarity search
type HNSWIndex struct {
	dimension      int
	M              int                     // max number of connections per layer
	Mmax           int                     // max number of connections for layer 0
	Mmax0          int                     // max number of connections for higher layers
	efConstruction int                     // size of dynamic candidate list during construction
	ef             int                     // size of dynamic candidate list during search
	ml             float64                 // level generation multiplier
	nodes          map[uuid.UUID]*HNSWNode // all nodes in the index
	entryPoint     *HNSWNode               // entry point for search (highest layer node)
	mu             sync.RWMutex
}

// NewHNSWIndex creates a new HNSW index
func NewHNSWIndex(dimension, m, efConstruction int) *HNSWIndex {
	return &HNSWIndex{
		dimension:      dimension,
		M:              m,
		Mmax:           m,
		Mmax0:          m * 2, // layer 0 can have more connections
		efConstruction: efConstruction,
		ef:             efConstruction,
		ml:             1.0 / math.Log(2.0),
		nodes:          make(map[uuid.UUID]*HNSWNode),
		entryPoint:     nil,
	}
}

// SetEf sets the search beam width
func (h *HNSWIndex) SetEf(ef int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ef = ef
}

// Add inserts a new vector into the HNSW index
func (h *HNSWIndex) Add(id uuid.UUID, vector []float32) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(vector) != h.dimension {
		return ErrDimensionMismatch
	}

	// Determine the level for this node
	level := h.randomLevel()

	// Create the new node
	node := &HNSWNode{
		ID:          id,
		Vector:      vector,
		Connections: make(map[int][]uuid.UUID),
		Level:       level,
	}

	// Initialize connection lists for all layers
	for l := 0; l <= level; l++ {
		node.Connections[l] = make([]uuid.UUID, 0)
	}

	// If this is the first node, set it as entry point
	if h.entryPoint == nil {
		h.entryPoint = node
		h.nodes[id] = node
		return nil
	}

	// Search for nearest neighbors at all layers
	ep := []uuid.UUID{h.entryPoint.ID}

	// Traverse from top layer to target layer
	for lc := h.entryPoint.Level; lc > level; lc-- {
		ep = h.searchLayer(vector, ep, 1, lc)
	}

	// Insert at all layers from level down to 0
	for lc := level; lc >= 0; lc-- {
		candidates := h.searchLayer(vector, ep, h.efConstruction, lc)

		// Select M nearest neighbors
		m := h.M
		if lc == 0 {
			m = h.Mmax0
		}

		neighbors := h.selectNeighbors(vector, candidates, m)

		// Add bidirectional connections
		for _, neighborID := range neighbors {
			h.connect(node.ID, neighborID, lc)
			h.connect(neighborID, node.ID, lc)

			// Prune connections if needed
			neighborNode := h.nodes[neighborID]
			if len(neighborNode.Connections[lc]) > m {
				h.pruneConnections(neighborID, lc, m)
			}
		}

		ep = candidates
	}

	// Update entry point if this node is higher
	if level > h.entryPoint.Level {
		h.entryPoint = node
	}

	h.nodes[id] = node
	return nil
}

// Search finds the k nearest neighbors to the query vector
func (h *HNSWIndex) Search(query []float32, k int) ([]uuid.UUID, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(query) != h.dimension {
		return nil, ErrDimensionMismatch
	}

	if h.entryPoint == nil {
		return []uuid.UUID{}, nil
	}

	// For very small graphs, use all nodes as entry points
	var ep []uuid.UUID
	if len(h.nodes) <= k*2 {
		// Small graph - collect all nodes
		for id := range h.nodes {
			ep = append(ep, id)
		}
	} else {
		ep = []uuid.UUID{h.entryPoint.ID}

		// Traverse from top layer to layer 1
		for lc := h.entryPoint.Level; lc > 0; lc-- {
			ep = h.searchLayer(query, ep, 1, lc)
		}
	}

	// Search at layer 0 with larger beam width
	ep = h.searchLayer(query, ep, max(h.ef, k), 0)

	// Sort results by distance to query vector before returning
	type distanceEntry struct {
		id       uuid.UUID
		distance float64
	}

	var entries []distanceEntry
	for _, id := range ep {
		if node, ok := h.nodes[id]; ok {
			dist := euclideanDistance(query, node.Vector)
			entries = append(entries, distanceEntry{
				id:       id,
				distance: dist,
			})
		}
	}

	// Sort by distance
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].distance > entries[j].distance {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// Return top k results
	var results []uuid.UUID
	for i := 0; i < k && i < len(entries); i++ {
		results = append(results, entries[i].id)
	}

	return results, nil
}

// searchLayer performs greedy search at a specific layer
func (h *HNSWIndex) searchLayer(query []float32, entryPoints []uuid.UUID, ef int, layer int) []uuid.UUID {
	// Priority queue for candidates (min-heap by distance)
	visited := make(map[uuid.UUID]bool)
	candidates := &distanceHeap{}
	results := &distanceHeap{}

	heap.Init(candidates)
	heap.Init(results)

	// Initialize with entry points
	for _, ep := range entryPoints {
		if node, ok := h.nodes[ep]; ok {
			dist := euclideanDistance(query, node.Vector)
			heap.Push(candidates, &distanceItem{id: ep, distance: dist})
			heap.Push(results, &distanceItem{id: ep, distance: -dist}) // max-heap for results
			visited[ep] = true
		}
	}

	for candidates.Len() > 0 {
		// Get closest candidate
		current := heap.Pop(candidates).(*distanceItem)

		// If this is farther than the worst result, we're done
		if results.Len() >= ef && current.distance > -results.Top().distance {
			break
		}

		// Examine neighbors
		if node, ok := h.nodes[current.id]; ok {
			if connections, exists := node.Connections[layer]; exists {
				for _, neighborID := range connections {
					if !visited[neighborID] {
						visited[neighborID] = true

						if neighbor, ok := h.nodes[neighborID]; ok {
							dist := euclideanDistance(query, neighbor.Vector)

							// Add to candidates if it's closer than the worst result
							if results.Len() < ef || dist < -results.Top().distance {
								heap.Push(candidates, &distanceItem{id: neighborID, distance: dist})
								heap.Push(results, &distanceItem{id: neighborID, distance: -dist})

								// Keep only ef results
								if results.Len() > ef {
									heap.Pop(results)
								}
							}
						}
					}
				}
			}
		}
	}

	// Extract IDs from results (sorted by distance)
	ids := make([]uuid.UUID, 0, results.Len())
	items := make([]*distanceItem, 0, results.Len())
	for results.Len() > 0 {
		items = append(items, heap.Pop(results).(*distanceItem))
	}

	// Reverse to get closest first
	for i := len(items) - 1; i >= 0; i-- {
		ids = append(ids, items[i].id)
	}

	return ids
}

// selectNeighbors selects m nearest neighbors using heuristic
func (h *HNSWIndex) selectNeighbors(query []float32, candidates []uuid.UUID, m int) []uuid.UUID {
	if len(candidates) <= m {
		return candidates
	}

	// Calculate distances and sort
	cands := make([]candidate, 0, len(candidates))
	for _, id := range candidates {
		if node, ok := h.nodes[id]; ok {
			dist := euclideanDistance(query, node.Vector)
			cands = append(cands, candidate{id: id, dist: dist})
		}
	}

	// Simple selection: take m nearest
	// (could be enhanced with diversity heuristic)
	sortCandidates(cands)

	selected := make([]uuid.UUID, 0, m)
	for i := 0; i < m && i < len(cands); i++ {
		selected = append(selected, cands[i].id)
	}

	return selected
}

// connect adds a connection between two nodes at a specific layer
func (h *HNSWIndex) connect(from, to uuid.UUID, layer int) {
	if node, ok := h.nodes[from]; ok {
		// Check if connection already exists
		for _, conn := range node.Connections[layer] {
			if conn == to {
				return
			}
		}
		node.Connections[layer] = append(node.Connections[layer], to)
	}
}

// pruneConnections reduces connections to m nearest neighbors
func (h *HNSWIndex) pruneConnections(nodeID uuid.UUID, layer, m int) {
	node, ok := h.nodes[nodeID]
	if !ok {
		return
	}

	connections := node.Connections[layer]
	if len(connections) <= m {
		return
	}

	// Calculate distances to all connections
	type connDist struct {
		id   uuid.UUID
		dist float64
	}

	distances := make([]connDist, 0, len(connections))
	for _, connID := range connections {
		if connNode, ok := h.nodes[connID]; ok {
			dist := euclideanDistance(node.Vector, connNode.Vector)
			distances = append(distances, connDist{id: connID, dist: dist})
		}
	}

	// Sort by distance
	for i := 0; i < len(distances)-1; i++ {
		for j := i + 1; j < len(distances); j++ {
			if distances[i].dist > distances[j].dist {
				distances[i], distances[j] = distances[j], distances[i]
			}
		}
	}

	// Keep only m nearest
	newConnections := make([]uuid.UUID, 0, m)
	for i := 0; i < m && i < len(distances); i++ {
		newConnections = append(newConnections, distances[i].id)
	}

	node.Connections[layer] = newConnections
}

// randomLevel generates a random level for a new node
func (h *HNSWIndex) randomLevel() int {
	level := 0
	for rand.Float64() < 0.5 && level < 16 { // cap at 16 layers
		level++
	}
	return level
}

// Remove deletes a node from the index
func (h *HNSWIndex) Remove(id uuid.UUID) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	node, ok := h.nodes[id]
	if !ok {
		return ErrNodeNotFound
	}

	// Remove connections to this node from all neighbors
	for layer := 0; layer <= node.Level; layer++ {
		for _, neighborID := range node.Connections[layer] {
			if neighbor, ok := h.nodes[neighborID]; ok {
				newConns := make([]uuid.UUID, 0)
				for _, conn := range neighbor.Connections[layer] {
					if conn != id {
						newConns = append(newConns, conn)
					}
				}
				neighbor.Connections[layer] = newConns
			}
		}
	}

	// Delete the node first
	delete(h.nodes, id)

	// Update entry point if needed (after deletion)
	if h.entryPoint != nil && h.entryPoint.ID == id {
		h.entryPoint = h.findNewEntryPoint()
	}

	return nil
}

// findNewEntryPoint finds a new entry point (highest level node)
func (h *HNSWIndex) findNewEntryPoint() *HNSWNode {
	var maxLevel int = -1
	var entryPoint *HNSWNode = nil

	for _, node := range h.nodes {
		if node.Level > maxLevel {
			maxLevel = node.Level
			entryPoint = node
		}
	}

	return entryPoint
}

// Size returns the number of nodes in the index
func (h *HNSWIndex) Size() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.nodes)
}

// EntryPoint returns the entry point node for search
func (h *HNSWIndex) EntryPoint() *HNSWNode {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.entryPoint
}

// GetAllNodes returns all node IDs in the index
func (h *HNSWIndex) GetAllNodes() []uuid.UUID {
	h.mu.RLock()
	defer h.mu.RUnlock()

	ids := make([]uuid.UUID, 0, len(h.nodes))
	for id := range h.nodes {
		ids = append(ids, id)
	}
	return ids
}

// GetNode returns a node by its ID
func (h *HNSWIndex) GetNode(id uuid.UUID) *HNSWNode {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.nodes[id]
}

// Helper functions

func euclideanDistance(v1, v2 []float32) float64 {
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

type candidate struct {
	id   uuid.UUID
	dist float64
}

func sortCandidates(cands []candidate) {
	// Simple bubble sort (could use sort.Slice for production)
	for i := 0; i < len(cands)-1; i++ {
		for j := i + 1; j < len(cands); j++ {
			if cands[i].dist > cands[j].dist {
				cands[i], cands[j] = cands[j], cands[i]
			}
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Priority queue implementation for distance-based search

type distanceItem struct {
	id       uuid.UUID
	distance float64
}

type distanceHeap []*distanceItem

func (h distanceHeap) Len() int           { return len(h) }
func (h distanceHeap) Less(i, j int) bool { return h[i].distance < h[j].distance }
func (h distanceHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *distanceHeap) Push(x interface{}) {
	*h = append(*h, x.(*distanceItem))
}

func (h *distanceHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}

func (h *distanceHeap) Top() *distanceItem {
	if len(*h) == 0 {
		return nil
	}
	return (*h)[0]
}

// Errors

var (
	ErrDimensionMismatch = &HNSWError{"dimension mismatch"}
	ErrNodeNotFound      = &HNSWError{"node not found"}
)

type HNSWError struct {
	msg string
}

func (e *HNSWError) Error() string {
	return e.msg
}
