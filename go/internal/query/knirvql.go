package query

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	coll "github.com/knirvcorp/knirvbase/go/internal/collection"
	db "github.com/knirvcorp/knirvbase/go/internal/database"
	stor "github.com/knirvcorp/knirvbase/go/internal/storage"
	typ "github.com/knirvcorp/knirvbase/go/internal/types"
)

// KNIRVQLParser parses KNIRVQL queries
type KNIRVQLParser struct{}

// Parse parses a KNIRVQL query
func (p *KNIRVQLParser) Parse(query string) (*Query, error) {
	query = strings.TrimSpace(query)
	parts := strings.Fields(query)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty query")
	}

	cmd := strings.ToUpper(parts[0])
	switch cmd {
	case "GET":
		return p.parseGet(parts[1:])
	case "SET":
		return p.parseSet(parts[1:])
	case "DELETE":
		return p.parseDelete(parts[1:])
	case "CREATE":
		return p.parseCreate(parts[1:])
	case "DROP":
		return p.parseDrop(parts[1:])
	default:
		return nil, fmt.Errorf("unknown command: %s", cmd)
	}
}

func (p *KNIRVQLParser) parseGet(parts []string) (*Query, error) {
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid GET query")
	}

	entryType := strings.ToUpper(parts[0])
	var collection string
	var filters []Filter
	var similarTo []float64
	var limit int

	i := 1
	if parts[i] == "WHERE" {
		i++
		for i < len(parts) {
			if parts[i] == "SIMILAR" && i+1 < len(parts) && parts[i+1] == "TO" {
				i += 2
				if i < len(parts) {
					vecStr := parts[i]
					vecStr = strings.Trim(vecStr, "[]")
					vecParts := strings.Split(vecStr, ",")
					for _, vp := range vecParts {
						if v, err := strconv.ParseFloat(strings.TrimSpace(vp), 64); err == nil {
							similarTo = append(similarTo, v)
						}
					}
				}
				break
			} else if parts[i] == "LIMIT" {
				i++
				if i < len(parts) {
					if l, err := strconv.Atoi(parts[i]); err == nil {
						limit = l
					}
				}
				break
			} else {
				// Parse filter: key operator value
				if i+2 < len(parts) {
					key := parts[i]
					operator := parts[i+1]
					valueStr := strings.Trim(parts[i+2], "\"")
					// Parse value
					var value interface{}
					if f, err := strconv.ParseFloat(valueStr, 64); err == nil {
						value = f
					} else if i, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
						value = i
					} else {
						value = valueStr
					}
					filters = append(filters, Filter{Key: key, Operator: operator, Value: value})
					i += 3
				} else {
					break
				}
			}
		}
	}

	return &Query{
		Type:       QueryGet,
		EntryType:  typ.EntryType(entryType),
		Collection: collection,
		Filters:    filters,
		SimilarTo:  similarTo,
		Limit:      limit,
	}, nil
}

func (p *KNIRVQLParser) parseSet(parts []string) (*Query, error) {
	if len(parts) < 3 || parts[1] != "=" {
		return nil, fmt.Errorf("invalid SET query")
	}

	key := parts[0]
	value := strings.Join(parts[2:], " ")
	value = strings.Trim(value, "\"")

	return &Query{
		Type:      QuerySet,
		Key:       key,
		Value:     value,
		EntryType: typ.EntryTypeAuth,
	}, nil
}

func (p *KNIRVQLParser) parseDelete(parts []string) (*Query, error) {
	if len(parts) < 2 || parts[0] != "WHERE" || parts[1] != "id" || parts[2] != "=" {
		return nil, fmt.Errorf("invalid DELETE query")
	}

	id := parts[3]

	return &Query{
		Type: QueryDelete,
		ID:   id,
	}, nil
}

func (p *KNIRVQLParser) parseCreate(parts []string) (*Query, error) {
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid CREATE command")
	}

	subCmd := strings.ToUpper(parts[0])
	switch subCmd {
	case "INDEX":
		return p.parseCreateIndex(parts[1:])
	case "COLLECTION":
		return p.parseCreateCollection(parts[1:])
	default:
		return nil, fmt.Errorf("unknown CREATE command: %s", subCmd)
	}
}

func (p *KNIRVQLParser) parseCreateIndex(parts []string) (*Query, error) {
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid CREATE INDEX command")
	}

	// Parse collection:index format
	indexRef := parts[0]
	indexParts := strings.Split(indexRef, ":")
	if len(indexParts) != 2 {
		return nil, fmt.Errorf("invalid index reference, expected collection:index")
	}
	collection := indexParts[0]
	indexName := indexParts[1]

	if parts[1] != "ON" {
		return nil, fmt.Errorf("expected ON after index name")
	}

	if parts[2] != collection {
		return nil, fmt.Errorf("collection mismatch in index definition")
	}

	fields := []string{}

	i := 3
	if i < len(parts) && strings.HasPrefix(parts[i], "(") {
		// Handle (field1,field2) format
		fieldStr := strings.Trim(parts[i], "()")
		if fieldStr != "" {
			fieldParts := strings.Split(fieldStr, ",")
			for _, f := range fieldParts {
				f = strings.TrimSpace(f)
				if f != "" {
					fields = append(fields, f)
				}
			}
		}
		i++
	}

	unique := false
	if i < len(parts) && strings.ToUpper(parts[i]) == "UNIQUE" {
		unique = true
		i++
	}

	return &Query{
		Type:       QueryCreateIndex,
		IndexName:  indexName,
		Collection: collection,
		Fields:     fields,
		Unique:     unique,
	}, nil
}

func (p *KNIRVQLParser) parseCreateCollection(parts []string) (*Query, error) {
	if len(parts) < 1 {
		return nil, fmt.Errorf("invalid CREATE COLLECTION command")
	}

	collectionName := parts[0]

	return &Query{
		Type:       QueryCreateCollection,
		Collection: collectionName,
	}, nil
}

func (p *KNIRVQLParser) parseDrop(parts []string) (*Query, error) {
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid DROP command")
	}

	subCmd := strings.ToUpper(parts[0])
	switch subCmd {
	case "INDEX":
		return p.parseDropIndex(parts[1:])
	case "COLLECTION":
		return p.parseDropCollection(parts[1:])
	default:
		return nil, fmt.Errorf("unknown DROP command: %s", subCmd)
	}
}

func (p *KNIRVQLParser) parseDropIndex(parts []string) (*Query, error) {
	if len(parts) < 1 {
		return nil, fmt.Errorf("invalid DROP INDEX command")
	}

	// Parse collection:index format
	indexRef := parts[0]
	indexParts := strings.Split(indexRef, ":")
	if len(indexParts) != 2 {
		return nil, fmt.Errorf("invalid index reference, expected collection:index")
	}

	return &Query{
		Type:       QueryDropIndex,
		Collection: indexParts[0],
		IndexName:  indexParts[1],
	}, nil
}

func (p *KNIRVQLParser) parseDropCollection(parts []string) (*Query, error) {
	if len(parts) < 1 {
		return nil, fmt.Errorf("invalid DROP COLLECTION command")
	}

	collectionName := parts[0]

	return &Query{
		Type:       QueryDropCollection,
		Collection: collectionName,
	}, nil
}

// Query represents a parsed KNIRVQL query
type Query struct {
	Type       QueryType
	EntryType  typ.EntryType
	Collection string
	ID         string
	Key        string
	Value      string
	Filters    []Filter
	SimilarTo  []float64
	Limit      int

	// Index management
	IndexName string
	Fields    []string
	Unique    bool
}

// QueryType enum
type QueryType int

const (
	QueryGet QueryType = iota
	QuerySet
	QueryDelete
	QueryCreateIndex
	QueryCreateCollection
	QueryDropIndex
	QueryDropCollection
)

// Filter for WHERE clauses
type Filter struct {
	Key      string
	Operator string
	Value    interface{}
}

// Execute executes the query on the database
func (q *Query) Execute(db *db.DistributedDatabase, collection *coll.DistributedCollection) (interface{}, error) {
	switch q.Type {
	case QueryGet:
		return q.executeGet(db, collection)
	case QuerySet:
		doc := map[string]interface{}{
			"id":        q.Key,
			"entryType": typ.EntryTypeAuth,
			"payload": map[string]interface{}{
				"key":   q.Key,
				"value": q.Value,
			},
		}
		_, err := collection.Insert(context.TODO(), doc)
		return nil, err
	case QueryDelete:
		_, err := collection.Delete(q.ID)
		return nil, err
	case QueryCreateIndex:
		// Default to B-Tree index
		indexType := stor.IndexTypeBTree
		if q.IndexName == "vector" {
			indexType = stor.IndexTypeHNSW
		}
		return nil, db.CreateIndex(q.Collection, q.IndexName, indexType, q.Fields, q.Unique, "", nil)
	case QueryCreateCollection:
		// Collections are created implicitly when accessed
		return nil, nil
	case QueryDropIndex:
		return nil, db.DropIndex(q.Collection, q.IndexName)
	case QueryDropCollection:
		// For now, just return success - actual drop would need more implementation
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported query type")
	}
}

// executeGet executes a GET query using the query optimizer
func (q *Query) executeGet(db *db.DistributedDatabase, collection *coll.DistributedCollection) (interface{}, error) {
	// Get collection name from query or default
	collectionName := q.Collection
	if collectionName == "" {
		if q.EntryType == typ.EntryTypeAuth {
			collectionName = "auth"
		} else {
			collectionName = "memory"
		}
	}

	// Get indexes for the collection
	indexes := db.GetIndexesForCollection(collectionName)

	// Create optimizer
	optimizer := NewQueryOptimizer(collectionName, indexes, nil)

	// Generate execution plan
	plan, err := optimizer.Optimize(q)
	if err != nil {
		return nil, fmt.Errorf("failed to optimize query: %w", err)
	}

	// Execute the plan
	return q.executePlan(plan, db, collection)
}

// executePlan executes a query plan
func (q *Query) executePlan(plan *QueryPlan, db *db.DistributedDatabase, collection *coll.DistributedCollection) (interface{}, error) {
	switch plan.ScanType {
	case FullScan:
		return q.executeFullScan(plan, collection)
	case IndexScan:
		return q.executeIndexScan(plan, db, collection)
	case IndexOnlyScan:
		return q.executeIndexOnlyScan(plan, db)
	default:
		return q.executeFullScan(plan, collection)
	}
}

// executeFullScan performs a full collection scan
func (q *Query) executeFullScan(plan *QueryPlan, collection *coll.DistributedCollection) (interface{}, error) {
	docs, err := collection.FindAll()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for _, doc := range docs {
		if q.matchesFiltersWithPlan(doc, plan.PostFilters) {
			results = append(results, doc)
		}
	}

	if plan.Limit > 0 && len(results) > plan.Limit {
		results = results[:plan.Limit]
	}

	return results, nil
}

// executeIndexScan performs an index scan followed by post-filtering
func (q *Query) executeIndexScan(plan *QueryPlan, db *db.DistributedDatabase, collection *coll.DistributedCollection) (interface{}, error) {
	// Query the index to get candidate document IDs
	docIDs, err := db.QueryIndex(plan.IndexName, plan.IndexName, map[string]interface{}{
		"value": plan.IndexFilters[0].Value, // Simplified - assumes single filter
	})
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for _, docID := range docIDs {
		doc, err := collection.Find(docID)
		if err != nil {
			continue
		}
		if doc != nil && q.matchesFiltersWithPlan(doc, plan.PostFilters) {
			results = append(results, doc)
		}
	}

	if plan.Limit > 0 && len(results) > plan.Limit {
		results = results[:plan.Limit]
	}

	return results, nil
}

// executeIndexOnlyScan performs an index-only scan (no document access needed)
func (q *Query) executeIndexOnlyScan(plan *QueryPlan, db *db.DistributedDatabase) (interface{}, error) {
	// For index-only scans, we can return document IDs directly
	docIDs, err := db.QueryIndex(plan.IndexName, plan.IndexName, map[string]interface{}{
		"value": plan.IndexFilters[0].Value, // Simplified - assumes single filter
	})
	if err != nil {
		return nil, err
	}

	if plan.Limit > 0 && len(docIDs) > plan.Limit {
		docIDs = docIDs[:plan.Limit]
	}

	return docIDs, nil
}

func (q *Query) matchesFiltersWithPlan(doc map[string]interface{}, filters []Filter) bool {
	payload, ok := doc["payload"].(map[string]interface{})
	if !ok {
		return false
	}
	for _, f := range filters {
		if !q.matchesFilter(payload, f) {
			return false
		}
	}
	return true
}

func (q *Query) matchesFilter(payload map[string]interface{}, filter Filter) bool {
	val, ok := payload[filter.Key]
	if !ok {
		return false
	}
	switch filter.Operator {
	case "=":
		return fmt.Sprintf("%v", val) == fmt.Sprintf("%v", filter.Value)
	case "!=":
		return fmt.Sprintf("%v", val) != fmt.Sprintf("%v", filter.Value)
	case ">":
		return compareValues(val, filter.Value) > 0
	case "<":
		return compareValues(val, filter.Value) < 0
	case ">=":
		return compareValues(val, filter.Value) >= 0
	case "<=":
		return compareValues(val, filter.Value) <= 0
	case "CONTAINS":
		valStr := fmt.Sprintf("%v", val)
		filterStr := fmt.Sprintf("%v", filter.Value)
		return strings.Contains(valStr, filterStr)
	case "STARTS_WITH":
		valStr := fmt.Sprintf("%v", val)
		filterStr := fmt.Sprintf("%v", filter.Value)
		return strings.HasPrefix(valStr, filterStr)
	default:
		return false
	}
}

func compareValues(a, b interface{}) int {
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	if aStr < bStr {
		return -1
	} else if aStr > bStr {
		return 1
	}
	return 0
}
