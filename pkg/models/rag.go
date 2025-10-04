package models

import "context"

// RAG defines the interface for Retrieval-Augmented Generation
type RAG interface {
	// Query answers a natural language question
	Query(ctx context.Context, question string) (*Answer, error)
}

// Answer represents the response to a query
type Answer struct {
	Text    string      `json:"text"`
	Sources []*Document `json:"sources"`
}
