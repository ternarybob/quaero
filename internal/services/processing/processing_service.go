package processing

import (
	"context"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// Service handles document extraction from source tables and vectorization
type Service struct {
	documentService   interfaces.DocumentService
	jiraStorage       interfaces.JiraStorage
	confluenceStorage interfaces.ConfluenceStorage
	logger            arbor.ILogger
	isRunning         bool
	lastRunTime       *time.Time
	lastRunError      error
}

// NewService creates a new processing service
func NewService(
	documentService interfaces.DocumentService,
	jiraStorage interfaces.JiraStorage,
	confluenceStorage interfaces.ConfluenceStorage,
	logger arbor.ILogger,
) *Service {
	return &Service{
		documentService:   documentService,
		jiraStorage:       jiraStorage,
		confluenceStorage: confluenceStorage,
		logger:            logger,
	}
}

// ProcessingStats represents statistics from a processing run
type ProcessingStats struct {
	StartTime        time.Time
	EndTime          time.Time
	Duration         time.Duration
	JiraProcessed    int
	JiraErrors       int
	ConfProcessed    int
	ConfErrors       int
	TotalProcessed   int
	TotalErrors      int
	NewDocuments     int
	UpdatedDocuments int
	Errors           []string
}

// ProcessingStatus represents the current state of the processing engine
type ProcessingStatus struct {
	IsRunning        bool       `json:"is_running"`
	LastRunTime      *time.Time `json:"last_run_time"`
	LastRunError     string     `json:"last_run_error,omitempty"`
	EngineStatus     string     `json:"engine_status"`
	TotalDocuments   int        `json:"total_documents"`
	ProcessedCount   int        `json:"processed_count"`
	PendingCount     int        `json:"pending_count"`
	FailedCount      int        `json:"failed_count"`
	LastRunTimestamp string     `json:"last_run_timestamp,omitempty"`
}

// GetStatus returns the current processing engine status
func (s *Service) GetStatus(ctx context.Context) (*ProcessingStatus, error) {
	status := &ProcessingStatus{
		IsRunning: s.isRunning,
	}

	if s.isRunning {
		status.EngineStatus = "RUNNING"
	} else {
		status.EngineStatus = "IDLE"
	}

	if s.lastRunTime != nil {
		status.LastRunTime = s.lastRunTime
		status.LastRunTimestamp = s.lastRunTime.Format(time.RFC3339)
	}

	if s.lastRunError != nil {
		status.LastRunError = s.lastRunError.Error()
	}

	stats, err := s.documentService.GetStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get document stats: %w", err)
	}

	status.TotalDocuments = stats.TotalDocuments
	// NOTE: Phase 3 - Embedding removal: track total documents, not vectorization
	status.ProcessedCount = stats.TotalDocuments
	status.PendingCount = 0
	status.FailedCount = 0

	return status, nil
}

// ProcessAll extracts and vectorizes documents from all sources
func (s *Service) ProcessAll(ctx context.Context) (*ProcessingStats, error) {
	s.isRunning = true
	defer func() {
		s.isRunning = false
	}()

	stats := &ProcessingStats{
		StartTime: time.Now(),
		Errors:    make([]string, 0),
	}

	s.logger.Info().Msg("Starting document processing from all sources")

	// Process Jira
	jiraStats, err := s.ProcessJira(ctx)
	if err != nil {
		stats.Errors = append(stats.Errors, fmt.Sprintf("Jira: %v", err))
	}
	stats.JiraProcessed = jiraStats.Processed
	stats.JiraErrors = jiraStats.Errors

	// Process Confluence
	confStats, err := s.ProcessConfluence(ctx)
	if err != nil {
		stats.Errors = append(stats.Errors, fmt.Sprintf("Confluence: %v", err))
	}
	stats.ConfProcessed = confStats.Processed
	stats.ConfErrors = confStats.Errors

	// Totals
	stats.TotalProcessed = stats.JiraProcessed + stats.ConfProcessed
	stats.TotalErrors = stats.JiraErrors + stats.ConfErrors
	stats.NewDocuments = jiraStats.New + confStats.New
	stats.UpdatedDocuments = jiraStats.Updated + confStats.Updated

	stats.EndTime = time.Now()
	stats.Duration = stats.EndTime.Sub(stats.StartTime)

	now := time.Now()
	s.lastRunTime = &now

	if len(stats.Errors) > 0 {
		s.lastRunError = fmt.Errorf("processing completed with %d errors", stats.TotalErrors)
	} else {
		s.lastRunError = nil
	}

	s.logger.Info().
		Int("total_processed", stats.TotalProcessed).
		Int("new", stats.NewDocuments).
		Int("updated", stats.UpdatedDocuments).
		Int("errors", stats.TotalErrors).
		Dur("duration", stats.Duration).
		Msg("Document processing completed")

	return stats, nil
}

// SourceStats represents processing statistics for a single source
type SourceStats struct {
	Processed int
	New       int
	Updated   int
	Errors    int
}

// ProcessJira extracts and vectorizes Jira issues
func (s *Service) ProcessJira(ctx context.Context) (*SourceStats, error) {
	stats := &SourceStats{}

	s.logger.Info().Msg("Processing Jira issues")

	// Get all issues from Jira storage
	issues, err := s.jiraStorage.GetAllProjects(ctx)
	if err != nil {
		return stats, fmt.Errorf("failed to get Jira projects: %w", err)
	}

	// For each project, get issues
	for _, project := range issues {
		projectIssues, err := s.jiraStorage.GetIssuesByProject(ctx, project.Key)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("project", project.Key).
				Msg("Failed to get issues")
			stats.Errors++
			continue
		}

		// Process each issue
		for _, issue := range projectIssues {
			// Check if document already exists
			existing, err := s.documentService.GetBySource(ctx, "jira", issue.Key)

			if err != nil || existing == nil {
				// New document - will be created by collector service
				stats.New++
			} else {
				// Existing document - could be updated
				stats.Updated++
			}

			stats.Processed++
		}
	}

	s.logger.Info().
		Int("processed", stats.Processed).
		Int("new", stats.New).
		Int("updated", stats.Updated).
		Int("errors", stats.Errors).
		Msg("Jira processing complete")

	return stats, nil
}

// ProcessConfluence extracts and vectorizes Confluence pages
func (s *Service) ProcessConfluence(ctx context.Context) (*SourceStats, error) {
	stats := &SourceStats{}

	s.logger.Info().Msg("Processing Confluence pages")

	// Get all spaces from Confluence storage
	spaces, err := s.confluenceStorage.GetAllSpaces(ctx)
	if err != nil {
		return stats, fmt.Errorf("failed to get Confluence spaces: %w", err)
	}

	// For each space, get pages
	for _, space := range spaces {
		pages, err := s.confluenceStorage.GetPagesBySpace(ctx, space.Key)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("space", space.Key).
				Msg("Failed to get pages")
			stats.Errors++
			continue
		}

		// Process each page
		for _, page := range pages {
			// Check if document already exists
			existing, err := s.documentService.GetBySource(ctx, "confluence", page.ID)

			if err != nil || existing == nil {
				// New document
				stats.New++
			} else {
				// Existing document - could be updated
				stats.Updated++
			}

			stats.Processed++
		}
	}

	s.logger.Info().
		Int("processed", stats.Processed).
		Int("new", stats.New).
		Int("updated", stats.Updated).
		Int("errors", stats.Errors).
		Msg("Confluence processing complete")

	return stats, nil
}

// VectorizeExisting vectorizes documents that don't have embeddings
func (s *Service) VectorizeExisting(ctx context.Context) error {
	s.logger.Info().Msg("Vectorizing documents without embeddings")

	// NOTE: Phase 5 - Embeddings removed, no longer processing vectorization
	// This method is now a no-op and can be removed in future cleanup

	total, err := s.documentService.Count(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to count documents: %w", err)
	}

	s.logger.Info().
		Int("total", total).
		Msg("Document processing status (embeddings removed in Phase 5)")

	return nil
}
