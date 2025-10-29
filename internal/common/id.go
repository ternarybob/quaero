package common

import (
	"github.com/google/uuid"
)

// NewDocumentID generates a unique document ID with the "doc_" prefix
// Format: doc_<uuid>
func NewDocumentID() string {
	return "doc_" + uuid.New().String()
}
