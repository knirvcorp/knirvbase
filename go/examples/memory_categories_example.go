// Example usage of new memory categories in storage and indexing operations
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/knirvcorp/knirvbase/go/internal/indexing"
	"github.com/knirvcorp/knirvbase/go/internal/storage"
	"github.com/knirvcorp/knirvbase/go/internal/types"
)

// MemoryBlock implements the indexing.Block interface for storage operations
type MemoryBlock struct {
	ID       uuid.UUID
	Time     int64
	Category types.MemoryCategory
	Vector   []float32
	Data     map[string]interface{}
}

func (mb MemoryBlock) GetBlockID() uuid.UUID             { return mb.ID }
func (mb MemoryBlock) GetTimestamp() int64               { return mb.Time }
func (mb MemoryBlock) GetCategory() types.MemoryCategory { return mb.Category }
func (mb MemoryBlock) GetSemanticVector() []float32      { return mb.Vector }

func main() {
	// Example: Using new memory categories with storage
	fmt.Println("KNIRVBASE Memory Categories Example")
	fmt.Println("==================================")

	// Create file storage
	fileStorage := storage.NewFileStorage("/tmp/knirvbase_example")
	defer fileStorage.Close()

	// Example documents with new memory categories
	memories := []MemoryBlock{
		{
			ID:       uuid.New(),
			Time:     1640995200, // Jan 1, 2022
			Category: types.MemoryCategoryEvent,
			Vector:   []float32{0.1, 0.2, 0.3},
			Data: map[string]interface{}{
				"event_type": "user_login",
				"user_id":    "user123",
				"location":   "New York",
			},
		},
		{
			ID:       uuid.New(),
			Time:     1640995260,
			Category: types.MemoryCategoryPreference,
			Vector:   []float32{0.4, 0.5, 0.6},
			Data: map[string]interface{}{
				"preference_type": "ui_theme",
				"value":           "dark_mode",
				"user_id":         "user123",
			},
		},
		{
			ID:       uuid.New(),
			Time:     1640995320,
			Category: types.MemoryCategoryTrait,
			Vector:   []float32{0.7, 0.8, 0.9},
			Data: map[string]interface{}{
				"trait_type": "learning_style",
				"value":      "visual",
				"confidence": 0.85,
			},
		},
	}

	// Store memories in file storage
	fmt.Println("\nStoring memories with new categories:")
	for _, memory := range memories {
		doc := map[string]interface{}{
			"id":        memory.ID.String(),
			"entryType": types.EntryTypeMemory,
			"category":  string(memory.Category),
			"timestamp": memory.Time,
			"vector":    memory.Vector,
			"payload":   memory.Data,
		}

		err := fileStorage.Insert("memories", doc)
		if err != nil {
			log.Printf("Error storing memory %s: %v", memory.ID, err)
			continue
		}
		fmt.Printf("✓ Stored %s memory: %s\n", memory.Category, memory.ID.String()[:8])
	}

	// Example: Using new memory categories with indexing
	fmt.Println("\nIndexing memories by category:")
	categoryIndex := indexing.NewCategoryIndex()

	ctx := context.Background()
	for _, memory := range memories {
		err := categoryIndex.Add(ctx, memory)
		if err != nil {
			log.Printf("Error indexing memory %s: %v", memory.ID, err)
			continue
		}
		fmt.Printf("✓ Indexed %s memory\n", memory.Category)
	}

	// Example: Querying by new memory categories
	fmt.Println("\nQuerying memories by new categories:")
	categories := []types.MemoryCategory{
		types.MemoryCategoryEvent,
		types.MemoryCategoryPreference,
		types.MemoryCategoryTrait,
	}

	for _, category := range categories {
		ids, err := categoryIndex.Search(ctx, category)
		if err != nil {
			log.Printf("Error searching category %s: %v", category, err)
			continue
		}
		fmt.Printf("✓ Found %d memories in category %s\n", len(ids), category)
	}

	// Example: Retrieving stored memories
	fmt.Println("\nRetrieving stored memories:")
	allDocs, err := fileStorage.FindAll("memories")
	if err != nil {
		log.Printf("Error retrieving memories: %v", err)
		return
	}

	for _, doc := range allDocs {
		if category, ok := doc["category"].(string); ok {
			fmt.Printf("✓ Retrieved memory: %s (Category: %s)\n",
				doc["id"].(string)[:8], category)
		}
	}

	fmt.Printf("\nSuccessfully demonstrated new memory categories: %s, %s, %s\n",
		types.MemoryCategoryEvent, types.MemoryCategoryPreference, types.MemoryCategoryTrait)
}
