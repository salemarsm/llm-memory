package memory

import "context"

// EmbeddingAdapter is the optional semantic retrieval extension point.
// No adapter is required; the system runs fully on lexical retrieval without one.
// Implement this interface and call Store.SetEmbeddingAdapter to enable
// SemanticScore in SearchRanked and BuildContext results.
type EmbeddingAdapter interface {
	// Embed returns one embedding vector per input text, in the same order.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// CosineSimilarity returns the cosine similarity between two vectors.
	CosineSimilarity(a, b []float32) float64
	// Dimensions returns the vector dimensionality of this adapter.
	Dimensions() int
}
