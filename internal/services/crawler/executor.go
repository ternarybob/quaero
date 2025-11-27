package crawler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/queue"
)

// Executor executes crawler jobs
type Executor struct {
	jobMgr  *queue.Manager
	logger  arbor.ILogger
	service *Service // Your existing crawler service
}

func NewExecutor(jobMgr *queue.Manager, logger arbor.ILogger, service *Service) *Executor {
	return &Executor{
		jobMgr:  jobMgr,
		logger:  logger,
		service: service,
	}
}

// CrawlerPayload defines the crawler job payload
type CrawlerPayload struct {
	URL        string            `json:"url"`
	Depth      int               `json:"depth"`
	MaxPages   int               `json:"max_pages"`
	Exclusions []string          `json:"exclusions,omitempty"`
	SourceType string            `json:"source_type"`
	SourceID   string            `json:"source_id"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// Execute implements the Executor interface
func (e *Executor) Execute(ctx context.Context, jobID string, payload []byte) error {
	var p CrawlerPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	e.logger.Debug().
		Str("job_id", jobID).
		Str("url", p.URL).
		Int("depth", p.Depth).
		Msg("Starting crawler job")

	e.jobMgr.AddJobLog(ctx, jobID, "info", fmt.Sprintf("Crawling URL: %s (depth: %d)", p.URL, p.Depth))

	// TODO: Implement actual crawling logic using the existing Service
	// This is a placeholder that will be replaced with real crawler integration
	// For now, just mark as completed with a note

	e.jobMgr.AddJobLog(ctx, jobID, "info", "Crawler execution - integration with existing service pending")

	// Update progress
	e.jobMgr.UpdateJobProgress(ctx, jobID, 1, 1)

	return nil
}
