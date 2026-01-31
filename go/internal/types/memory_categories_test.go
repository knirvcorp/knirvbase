package types

import (
	"testing"

	"github.com/google/uuid"
)

// TestMemoryCategories verifies that all memory categories are properly defined and accessible
func TestMemoryCategories(t *testing.T) {
	// Test existing categories
	categories := []MemoryCategory{
		MemoryCategoryError,
		MemoryCategoryContext,
		MemoryCategoryIdea,
		MemoryCategorySolution,
		MemoryCategorySkill,
		MemoryCategoryGeneric,
		// New categories
		MemoryCategoryEvent,
		MemoryCategoryPreference,
		MemoryCategoryTrait,
	}

	// Verify each category has a unique string representation
	seen := make(map[string]bool)
	for _, cat := range categories {
		catStr := string(cat)
		if seen[catStr] {
			t.Errorf("Duplicate memory category: %s", catStr)
		}
		seen[catStr] = true
	}

	// Test that categories can be used in map operations (as needed for storage)
	categoryMap := make(map[MemoryCategory][]uuid.UUID)

	for _, cat := range categories {
		id := uuid.New()
		categoryMap[cat] = append(categoryMap[cat], id)
	}

	// Verify map operations work correctly
	for _, cat := range categories {
		if len(categoryMap[cat]) == 0 {
			t.Errorf("Category %s not found in map", cat)
		}
	}

	// Test that new categories are properly stringified
	if string(MemoryCategoryEvent) != "EVENT" {
		t.Errorf("MemoryCategoryEvent string representation: got %s, want EVENT", MemoryCategoryEvent)
	}
	if string(MemoryCategoryPreference) != "PREFERENCE" {
		t.Errorf("MemoryCategoryPreference string representation: got %s, want PREFERENCE", MemoryCategoryPreference)
	}
	if string(MemoryCategoryTrait) != "TRAIT" {
		t.Errorf("MemoryCategoryTrait string representation: got %s, want TRAIT", MemoryCategoryTrait)
	}
}

// TestBlock implements the Block interface for testing
type TestBlock struct {
	ID       uuid.UUID
	Time     int64
	Category MemoryCategory
	Vector   []float32
}

func (tb TestBlock) GetBlockID() uuid.UUID        { return tb.ID }
func (tb TestBlock) GetTimestamp() int64          { return tb.Time }
func (tb TestBlock) GetCategory() MemoryCategory  { return tb.Category }
func (tb TestBlock) GetSemanticVector() []float32 { return tb.Vector }

// TestNewMemoryCategoriesInIndexing verifies new categories work with indexing system
func TestNewMemoryCategoriesInIndexing(t *testing.T) {
	// This demonstrates that new memory categories can be used with the indexing system
	blocks := []TestBlock{
		{
			ID:       uuid.New(),
			Time:     1000,
			Category: MemoryCategoryEvent,
			Vector:   []float32{0.1, 0.2, 0.3},
		},
		{
			ID:       uuid.New(),
			Time:     2000,
			Category: MemoryCategoryPreference,
			Vector:   []float32{0.4, 0.5, 0.6},
		},
		{
			ID:       uuid.New(),
			Time:     3000,
			Category: MemoryCategoryTrait,
			Vector:   []float32{0.7, 0.8, 0.9},
		},
	}

	// Verify categories are accessible and can be used in storage-like operations
	categoryGrouping := make(map[MemoryCategory][]TestBlock)

	for _, block := range blocks {
		categoryGrouping[block.GetCategory()] = append(categoryGrouping[block.GetCategory()], block)
	}

	// Verify grouping worked correctly
	expectedCategories := []MemoryCategory{MemoryCategoryEvent, MemoryCategoryPreference, MemoryCategoryTrait}
	for _, expectedCat := range expectedCategories {
		blocks, found := categoryGrouping[expectedCat]
		if !found {
			t.Errorf("Category %s not found in grouping", expectedCat)
			continue
		}
		if len(blocks) != 1 {
			t.Errorf("Expected 1 block for category %s, got %d", expectedCat, len(blocks))
		}
	}
}
