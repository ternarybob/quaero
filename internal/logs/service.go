package logs

import (
	"container/heap"
	"context"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// ErrJobNotFound is a sentinel error returned when a job is not found
var ErrJobNotFound = fmt.Errorf("job not found")

// Service implements LogService for storage operations only
type Service struct {
	storage    interfaces.LogStorage
	jobStorage interfaces.QueueStorage
	logger     arbor.ILogger
}

// NewService creates a new LogService for log storage operations
func NewService(storage interfaces.LogStorage, jobStorage interfaces.QueueStorage, logger arbor.ILogger) interfaces.LogService {
	return &Service{
		storage:    storage,
		jobStorage: jobStorage,
		logger:     logger,
	}
}

// AppendLog appends a single log entry (delegates to storage)
func (s *Service) AppendLog(ctx context.Context, jobID string, entry models.LogEntry) error {
	return s.storage.AppendLog(ctx, jobID, entry)
}

// AppendLogs appends multiple log entries (delegates to storage)
func (s *Service) AppendLogs(ctx context.Context, jobID string, entries []models.LogEntry) error {
	return s.storage.AppendLogs(ctx, jobID, entries)
}

// GetLogs retrieves log entries for a job (delegates to storage)
func (s *Service) GetLogs(ctx context.Context, jobID string, limit int) ([]models.LogEntry, error) {
	return s.storage.GetLogs(ctx, jobID, limit)
}

// GetLogsByLevel retrieves log entries filtered by level (delegates to storage)
func (s *Service) GetLogsByLevel(ctx context.Context, jobID string, level string, limit int) ([]models.LogEntry, error) {
	return s.storage.GetLogsByLevel(ctx, jobID, level, limit)
}

// DeleteLogs deletes all log entries for a job (delegates to storage)
func (s *Service) DeleteLogs(ctx context.Context, jobID string) error {
	return s.storage.DeleteLogs(ctx, jobID)
}

// CountLogs returns the number of log entries for a job (delegates to storage)
func (s *Service) CountLogs(ctx context.Context, jobID string) (int, error) {
	return s.storage.CountLogs(ctx, jobID)
}

func (s *Service) CountLogsByLevel(ctx context.Context, jobID string, level string) (int, error) {
	return s.storage.CountLogsByLevel(ctx, jobID, level)
}

// GetLogsWithOffset fetches logs with offset-based pagination (delegates to storage)
// Returns logs in DESC order (newest first) after skipping 'offset' newest logs
func (s *Service) GetLogsWithOffset(ctx context.Context, jobID string, limit int, offset int) ([]models.LogEntry, error) {
	return s.storage.GetLogsWithOffset(ctx, jobID, limit, offset)
}

// GetAggregatedLogs fetches logs for parent job and optionally all child jobs
// Implements k-way merge with cursor-based pagination
// Returns logs slice, metadata map, and next_cursor for pagination
func (s *Service) GetAggregatedLogs(ctx context.Context, parentJobID string, includeChildren bool, level string, limit int, cursor string, order string) ([]models.LogEntry, map[string]*interfaces.AggregatedJobMeta, string, error) {
	metadata := make(map[string]*interfaces.AggregatedJobMeta)
	var allLogs []models.LogEntry

	// Decode cursor if provided
	cursorKey, err := decodeCursor(cursor)
	if err != nil {
		return nil, nil, "", fmt.Errorf("invalid cursor: %w", err)
	}

	// Check if parent job exists (required - return 404 if not found)
	parentJob, err := s.jobStorage.GetJob(ctx, parentJobID)
	if err != nil {
		return nil, nil, "", fmt.Errorf("%w: %v", ErrJobNotFound, err)
	}

	// Extract metadata from parent job (best-effort - don't fail if extraction fails)
	if jobState, ok := parentJob.(*models.QueueJobState); ok {
		jobMeta := s.extractJobMetadata(jobState.ToQueueJob())
		metadata[parentJobID] = jobMeta
	} else {
		// Log warning but continue - metadata enrichment is optional, job existence is not
		s.logger.Warn().Str("parent_job_id", parentJobID).Msg("Could not extract job metadata, continuing with logs-only response")
	}

	// Collect all job IDs for iterators
	jobIDs := []string{parentJobID}

	// Step 1: Fetch all descendant jobs recursively if requested
	// This ensures logs from grandchildren (worker jobs under step jobs) are included
	if includeChildren {
		allDescendants := s.collectAllDescendants(ctx, parentJobID, metadata)
		jobIDs = append(jobIDs, allDescendants...)
	}

	// Step 2: Create iterators for each job
	numJobs := len(jobIDs)
	batchSize := (limit + numJobs - 1) / numJobs // Ceiling division to distribute load
	if batchSize < 10 {
		batchSize = 10 // Minimum batch size
	}

	iterators := make([]*logIterator, 0, numJobs)
	for _, jobID := range jobIDs {
		iter := newLogIterator(ctx, jobID, level, order, cursorKey, s.storage, batchSize)
		iterators = append(iterators, iter)
	}

	// Step 3: Perform k-way merge using heap
	var h heap.Interface
	if order == "asc" {
		h = &minHeap{}
	} else {
		h = &maxHeap{}
	}

	// Initialize heap with first log from each iterator
	for _, iter := range iterators {
		log, err := iter.next()
		if err != nil {
			s.logger.Warn().Err(err).Msg("Error fetching initial log from iterator")
			continue
		}
		if log != nil {
			// Compute seqAtPush for this log
			seqAtPush := iter.seq - 1
			heap.Push(h, heapItem{log: *log, iterator: iter, seqAtPush: seqAtPush})
		}
	}

	// Extract logs up to limit
	allLogs = make([]models.LogEntry, 0, limit)
	var lastItem *heapItem = nil

	for len(allLogs) < limit && h.Len() > 0 {
		// Pop next log from heap
		item := heap.Pop(h).(heapItem)

		// Add log to results
		log := item.log
		allLogs = append(allLogs, log)

		// Track the last emitted item
		lastItem = &item

		// Get next log from same iterator
		nextLog, err := item.iterator.next()
		if err != nil {
			s.logger.Warn().Err(err).Str("job_id", item.iterator.jobID).Msg("Error fetching next log from iterator")
			continue
		}
		if nextLog != nil {
			// Compute seqAtPush for the next log
			seqAtPush := item.iterator.seq - 1
			// Add back to heap
			heap.Push(h, heapItem{log: *nextLog, iterator: item.iterator, seqAtPush: seqAtPush})
		}
	}

	// Step 4: Generate next_cursor from last result only if more results remain
	var nextCursor string
	if len(allLogs) > 0 && lastItem != nil {
		// Check if more results remain: either heap has items or any iterator can still yield
		hasMore := h.Len() > 0
		if !hasMore {
			// Check all iterators to see if they can still produce data
			for _, iter := range iterators {
				if !iter.done || iter.nextIdx < len(iter.logs) {
					hasMore = true
					break
				}
			}
		}

		// Only emit next_cursor if more results remain
		if hasMore {
			lastLog := allLogs[len(allLogs)-1]
			nextCursorKey := &CursorKey{
				FullTimestamp: lastLog.FullTimestamp,
				JobID:         lastLog.JobID(),
				Seq:           lastItem.seqAtPush,
			}
			nextCursor = encodeCursor(nextCursorKey)
		}
	}

	return allLogs, metadata, nextCursor, nil
}

// extractJobMetadata extracts relevant metadata from a QueueJob for UI display
func (s *Service) extractJobMetadata(job *models.QueueJob) *interfaces.AggregatedJobMeta {
	meta := &interfaces.AggregatedJobMeta{}

	// Job name
	if job.Name != "" {
		meta.JobName = job.Name
	} else {
		meta.JobName = fmt.Sprintf("Job %s", job.ID)
	}

	// Job URL - extract from Config["seed_urls"]
	if seedURLs, ok := job.Config["seed_urls"].([]interface{}); ok && len(seedURLs) > 0 {
		if url, ok := seedURLs[0].(string); ok {
			meta.JobURL = url
		}
	}

	// Job depth - extract from Config["max_depth"]
	if maxDepth, ok := job.Config["max_depth"].(float64); ok {
		meta.JobDepth = int(maxDepth)
	} else if maxDepth, ok := job.Config["max_depth"].(int); ok {
		meta.JobDepth = maxDepth
	}

	// Job type
	meta.JobType = job.Type

	// Parent ID
	if job.ParentID != nil {
		meta.ParentID = *job.ParentID
	}

	return meta
}

// collectAllDescendants recursively collects all descendant job IDs from a parent job.
// This includes children, grandchildren, etc. - all jobs in the hierarchy.
// It also populates the metadata map for each discovered job.
// Limited to maxDescendants jobs to prevent excessive memory/query usage.
func (s *Service) collectAllDescendants(ctx context.Context, parentJobID string, metadata map[string]*interfaces.AggregatedJobMeta) []string {
	const maxDescendants = 1000 // Safety limit for very large job trees

	var result []string

	// Use a queue for breadth-first traversal to avoid deep recursion
	queue := []string{parentJobID}

	for len(queue) > 0 && len(result) < maxDescendants {
		// Pop from front
		currentID := queue[0]
		queue = queue[1:]

		// Get direct children of current job
		childJobs, err := s.jobStorage.GetChildJobs(ctx, currentID)
		if err != nil {
			s.logger.Debug().Err(err).Str("parent_id", currentID).Msg("Failed to fetch child jobs")
			continue
		}

		// Process each child
		for _, childJob := range childJobs {
			if len(result) >= maxDescendants {
				s.logger.Info().Int("limit", maxDescendants).Str("parent_id", parentJobID).Msg("Reached max descendants limit for log aggregation")
				break
			}

			// Add to result
			result = append(result, childJob.ID)

			// Extract and store metadata
			jobMeta := s.extractJobMetadata(childJob)
			metadata[childJob.ID] = jobMeta

			// Add to queue for further traversal (to get grandchildren)
			queue = append(queue, childJob.ID)
		}
	}

	return result
}
