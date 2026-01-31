package embedding

import (
	"fmt"
	"math"
)

// LSAReducer implements Latent Semantic Analysis for dimensionality reduction
type LSAReducer struct {
	components    [][]float64 // Principal components (eigenvectors)
	targetDim     int         // Target dimensionality
	sourceDim     int         // Source dimensionality
	mean          []float64   // Mean vector for centering
}

// NewLSAReducer creates a new LSA reducer
func NewLSAReducer(targetDim int) *LSAReducer {
	return &LSAReducer{
		targetDim: targetDim,
	}
}

// Fit learns the principal components from a matrix of vectors
func (l *LSAReducer) Fit(vectors [][]float64) error {
	if len(vectors) == 0 {
		return fmt.Errorf("no vectors provided")
	}

	l.sourceDim = len(vectors[0])
	numVectors := len(vectors)

	// Compute mean vector for centering
	l.mean = make([]float64, l.sourceDim)
	for _, vec := range vectors {
		for i, val := range vec {
			l.mean[i] += val
		}
	}
	for i := range l.mean {
		l.mean[i] /= float64(numVectors)
	}

	// Center the data
	centeredVectors := make([][]float64, numVectors)
	for i, vec := range vectors {
		centeredVectors[i] = make([]float64, l.sourceDim)
		for j := range vec {
			centeredVectors[i][j] = vec[j] - l.mean[j]
		}
	}

	// Compute covariance matrix (simplified: just use the vectors as-is for PCA-like reduction)
	// For true SVD, we'd need more complex matrix operations
	// Here we use a simplified approach: find principal components using power iteration

	l.components = make([][]float64, l.targetDim)

	// Extract top k components using power iteration
	for k := 0; k < l.targetDim && k < l.sourceDim; k++ {
		component := l.extractComponent(centeredVectors, k)
		l.components[k] = component

		// Deflate: remove this component's contribution from the vectors
		for i := range centeredVectors {
			projection := dotProduct(centeredVectors[i], component)
			for j := range centeredVectors[i] {
				centeredVectors[i][j] -= projection * component[j]
			}
		}
	}

	return nil
}

// extractComponent extracts a principal component using power iteration
func (l *LSAReducer) extractComponent(vectors [][]float64, componentIdx int) []float64 {
	if len(vectors) == 0 || l.sourceDim == 0 {
		return make([]float64, l.sourceDim)
	}

	// Initialize with random vector
	component := make([]float64, l.sourceDim)
	for i := range component {
		component[i] = 1.0 / math.Sqrt(float64(l.sourceDim))
	}

	// Power iteration
	maxIterations := 100
	for iter := 0; iter < maxIterations; iter++ {
		// Multiply by matrix transpose
		newComponent := make([]float64, l.sourceDim)
		for _, vec := range vectors {
			proj := dotProduct(vec, component)
			for j := range newComponent {
				newComponent[j] += proj * vec[j]
			}
		}

		// Normalize
		norm := vectorNorm(newComponent)
		if norm > 0 {
			for j := range newComponent {
				newComponent[j] /= norm
			}
		}

		// Check convergence
		diff := 0.0
		for j := range component {
			diff += math.Abs(newComponent[j] - component[j])
		}
		component = newComponent

		if diff < 1e-6 {
			break
		}
	}

	return component
}

// Transform reduces the dimensionality of a vector
func (l *LSAReducer) Transform(vector []float64) ([]float64, error) {
	if len(vector) != l.sourceDim {
		return nil, fmt.Errorf("vector dimension mismatch: expected %d, got %d", l.sourceDim, len(vector))
	}

	// Center the vector
	centered := make([]float64, len(vector))
	for i := range vector {
		centered[i] = vector[i] - l.mean[i]
	}

	// Project onto principal components
	reduced := make([]float64, l.targetDim)
	for i, component := range l.components {
		if i < l.targetDim {
			reduced[i] = dotProduct(centered, component)
		}
	}

	return reduced, nil
}

// TransformBatch reduces dimensionality for multiple vectors
func (l *LSAReducer) TransformBatch(vectors [][]float64) ([][]float64, error) {
	reduced := make([][]float64, len(vectors))
	for i, vec := range vectors {
		r, err := l.Transform(vec)
		if err != nil {
			return nil, err
		}
		reduced[i] = r
	}
	return reduced, nil
}

// FitTransform fits the reducer and transforms vectors in one step
func (l *LSAReducer) FitTransform(vectors [][]float64) ([][]float64, error) {
	if err := l.Fit(vectors); err != nil {
		return nil, err
	}
	return l.TransformBatch(vectors)
}

// dotProduct computes the dot product of two vectors
func dotProduct(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0.0
	}
	sum := 0.0
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}

// vectorNorm computes the L2 norm of a vector
func vectorNorm(v []float64) float64 {
	sum := 0.0
	for _, val := range v {
		sum += val * val
	}
	return math.Sqrt(sum)
}

// normalizeVector normalizes a vector to unit length
func normalizeVector(v []float64) []float64 {
	norm := vectorNorm(v)
	if norm == 0 {
		return v
	}
	normalized := make([]float64, len(v))
	for i, val := range v {
		normalized[i] = val / norm
	}
	return normalized
}
