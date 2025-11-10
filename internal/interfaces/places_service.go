package interfaces

import (
	"context"

	"github.com/ternarybob/quaero/internal/models"
)

// PlacesService defines the interface for Google Places API operations
type PlacesService interface {
	// SearchPlaces performs a Google Places API search and returns results.
	// Results are returned as structured data for the executor to store in job progress.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control
	//   - jobID: ID of the job triggering this search
	//   - req: Search request parameters
	//
	// Returns:
	//   - *models.PlacesSearchResult: Search results with places data
	//   - error: Error if search fails
	SearchPlaces(ctx context.Context, jobID string, req *models.PlacesSearchRequest) (*models.PlacesSearchResult, error)
}
