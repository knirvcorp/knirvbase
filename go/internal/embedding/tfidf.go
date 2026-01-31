package embedding

import (
	"fmt"
	"math"
	"strings"
	"unicode"
)

// TFIDFVectorizer implements TF-IDF vectorization
type TFIDFVectorizer struct {
	vocabulary    map[string]int      // word -> index
	idf           map[string]float64  // word -> inverse document frequency
	docCount      int                 // total number of documents
	wordDocCounts map[string]int      // word -> number of documents containing it
}

// NewTFIDFVectorizer creates a new TF-IDF vectorizer
func NewTFIDFVectorizer() *TFIDFVectorizer {
	return &TFIDFVectorizer{
		vocabulary:    make(map[string]int),
		idf:           make(map[string]float64),
		wordDocCounts: make(map[string]int),
		docCount:      0,
	}
}

// tokenize splits text into words
func tokenize(text string) []string {
	// Convert to lowercase and split on non-alphanumeric characters
	text = strings.ToLower(text)

	var tokens []string
	var currentToken strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			currentToken.WriteRune(r)
		} else {
			if currentToken.Len() > 0 {
				token := currentToken.String()
				if len(token) > 1 { // Filter out single-character tokens
					tokens = append(tokens, token)
				}
				currentToken.Reset()
			}
		}
	}

	// Add last token
	if currentToken.Len() > 0 {
		token := currentToken.String()
		if len(token) > 1 {
			tokens = append(tokens, token)
		}
	}

	return tokens
}

// Fit learns the vocabulary and IDF from documents
func (v *TFIDFVectorizer) Fit(documents []string) error {
	if len(documents) == 0 {
		return fmt.Errorf("no documents provided")
	}

	// Reset state
	v.vocabulary = make(map[string]int)
	v.idf = make(map[string]float64)
	v.wordDocCounts = make(map[string]int)
	v.docCount = len(documents)

	// Build vocabulary and count document frequencies
	for _, doc := range documents {
		tokens := tokenize(doc)
		uniqueWords := make(map[string]bool)

		for _, token := range tokens {
			uniqueWords[token] = true
		}

		for word := range uniqueWords {
			v.wordDocCounts[word]++
		}
	}

	// Assign indices to words and compute IDF
	idx := 0
	for word, docFreq := range v.wordDocCounts {
		v.vocabulary[word] = idx
		// IDF = log(N / df) where N is total docs, df is doc frequency
		v.idf[word] = math.Log(float64(v.docCount) / float64(docFreq))
		idx++
	}

	return nil
}

// FitIncremental updates the vocabulary with a new document
func (v *TFIDFVectorizer) FitIncremental(document string) {
	tokens := tokenize(document)
	uniqueWords := make(map[string]bool)

	for _, token := range tokens {
		uniqueWords[token] = true
	}

	v.docCount++

	for word := range uniqueWords {
		// Update document count
		v.wordDocCounts[word]++

		// Add to vocabulary if new
		if _, exists := v.vocabulary[word]; !exists {
			v.vocabulary[word] = len(v.vocabulary)
		}

		// Recompute IDF
		v.idf[word] = math.Log(float64(v.docCount) / float64(v.wordDocCounts[word]))
	}

	// Update IDF for existing words that aren't in this document
	for word := range v.vocabulary {
		if !uniqueWords[word] {
			v.idf[word] = math.Log(float64(v.docCount) / float64(v.wordDocCounts[word]))
		}
	}
}

// Transform converts a document to a TF-IDF vector
func (v *TFIDFVectorizer) Transform(document string) ([]float64, error) {
	if len(v.vocabulary) == 0 {
		return nil, fmt.Errorf("vectorizer not fitted")
	}

	tokens := tokenize(document)

	// Compute term frequencies
	tf := make(map[string]float64)
	for _, token := range tokens {
		tf[token]++
	}

	// Normalize by document length
	docLength := float64(len(tokens))
	if docLength > 0 {
		for word := range tf {
			tf[word] /= docLength
		}
	}

	// Build TF-IDF vector
	vector := make([]float64, len(v.vocabulary))
	for word, freq := range tf {
		if idx, exists := v.vocabulary[word]; exists {
			vector[idx] = freq * v.idf[word]
		}
	}

	return vector, nil
}

// FitTransform fits the vectorizer and transforms documents in one step
func (v *TFIDFVectorizer) FitTransform(documents []string) ([][]float64, error) {
	if err := v.Fit(documents); err != nil {
		return nil, err
	}

	vectors := make([][]float64, len(documents))
	for i, doc := range documents {
		vec, err := v.Transform(doc)
		if err != nil {
			return nil, err
		}
		vectors[i] = vec
	}

	return vectors, nil
}

// VocabularySize returns the number of unique words
func (v *TFIDFVectorizer) VocabularySize() int {
	return len(v.vocabulary)
}

// GetVocabulary returns a copy of the vocabulary
func (v *TFIDFVectorizer) GetVocabulary() map[string]int {
	vocab := make(map[string]int)
	for word, idx := range v.vocabulary {
		vocab[word] = idx
	}
	return vocab
}
