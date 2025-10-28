package interfaces

import (
	"context"
)

// SummaryService generates and maintains summary documents about the corpus
type SummaryService interface {
	// GenerateSummaryDocument creates/updates a special summary document
	// containing metadata about the document corpus
	GenerateSummaryDocument(ctx context.Context) error
}
