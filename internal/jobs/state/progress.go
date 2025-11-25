package state

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ternarybob/quaero/internal/models"
)

// UpdateJobProgress updates job progress
func (m *Manager) UpdateJobProgress(ctx context.Context, jobID string, current, total int) error {
	progress := &models.JobProgress{
		CompletedURLs: current,
		TotalURLs:     total,
	}

	progressJSON, err := json.Marshal(progress)
	if err != nil {
		return fmt.Errorf("marshal progress: %w", err)
	}

	return m.jobStorage.UpdateJobProgress(ctx, jobID, string(progressJSON))
}

// IncrementDocumentCount increments the document_count in job metadata
// This is used to track the number of documents saved by child jobs for a parent job
func (m *Manager) IncrementDocumentCount(ctx context.Context, jobID string) error {
	// Get job
	jobEntityInterface, err := m.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	jobState := jobEntityInterface.(*models.QueueJobState)

	if jobState.Metadata == nil {
		jobState.Metadata = make(map[string]interface{})
	}

	// Increment document_count
	currentCount := 0
	if count, ok := jobState.Metadata["document_count"].(float64); ok {
		currentCount = int(count)
	} else if count, ok := jobState.Metadata["document_count"].(int); ok {
		currentCount = count
	}
	jobState.Metadata["document_count"] = currentCount + 1

	return m.jobStorage.UpdateJob(ctx, jobState)
}

// GetDocumentCount retrieves the document_count from job metadata
// Returns 0 if document_count is not present in metadata
func (m *Manager) GetDocumentCount(ctx context.Context, jobID string) (int, error) {
	jobEntityInterface, err := m.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return 0, err
	}
	jobState := jobEntityInterface.(*models.QueueJobState)

	if jobState.Metadata == nil {
		return 0, nil
	}

	if count, ok := jobState.Metadata["document_count"].(float64); ok {
		return int(count), nil
	} else if count, ok := jobState.Metadata["document_count"].(int); ok {
		return count, nil
	}

	return 0, nil
}
