package memory

import "time"

type Document struct {
	ID         string    `json:"id"`
	Path       string    `json:"path"`
	Title      string    `json:"title"`
	SourceKind string    `json:"source_kind"`
	SourceRef  string    `json:"source_ref"`
	SHA256     string    `json:"sha256"`
	CreatedAt  time.Time `json:"created_at"`
}

type Chunk struct {
	ID            string        `json:"id"`
	DocumentID    string        `json:"document_id"`
	Ordinal       int           `json:"ordinal"`
	HeadingPath   string        `json:"heading_path"`
	Content       string        `json:"content"`
	TokenCount    int           `json:"token_count"`
	PageFrom      *int          `json:"page_from,omitempty"`
	PageTo        *int          `json:"page_to,omitempty"`
	EmbeddingRefs EmbeddingRefs `json:"embedding_refs"`
	CreatedAt     time.Time     `json:"created_at"`
}

type ChunkSearchRequest struct {
	Text       string `json:"text"`
	DocumentID string `json:"document_id"`
	Limit      int    `json:"limit"`
}

type ChunkSearchResult struct {
	Chunk    Chunk    `json:"chunk"`
	Document Document `json:"document"`
	Score    float64  `json:"score"`
}
