package models

import "time"

// Document represents a piece of content from any source
type Document struct {
	ID        string                 `json:"id"`
	Source    string                 `json:"source"`
	Title     string                 `json:"title"`
	ContentMD string                 `json:"content_md"`
	Chunks    []Chunk                `json:"chunks"`
	Images    []Image                `json:"images"`
	Metadata  map[string]interface{} `json:"metadata"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// Chunk represents a chunk of text with its vector embedding
type Chunk struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	Embedding []float64              `json:"embedding,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Image represents an image in a document
type Image struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url,omitempty"`
}
