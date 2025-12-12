package badger

import (
	"context"
	"fmt"
	"sort"
	"sync/atomic"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/timshannon/badgerhold/v4"
)

// logSequence is a global counter to ensure unique log keys even within the same nanosecond
var logSequence uint64

// sortLogsAsc sorts logs in ascending order (oldest first)
// Uses Sequence field if available, falls back to FullTimestamp for backwards compatibility
func sortLogsAsc(logs []models.LogEntry) {
	sort.SliceStable(logs, func(i, j int) bool {
		// Both have Sequence - compare by Sequence
		if logs[i].Sequence != "" && logs[j].Sequence != "" {
			return logs[i].Sequence < logs[j].Sequence
		}
		// One or both missing Sequence - fall back to FullTimestamp
		return logs[i].FullTimestamp < logs[j].FullTimestamp
	})
}

// sortLogsDesc sorts logs in descending order (newest first)
// Uses Sequence field if available, falls back to FullTimestamp for backwards compatibility
func sortLogsDesc(logs []models.LogEntry) {
	sort.SliceStable(logs, func(i, j int) bool {
		// Both have Sequence - compare by Sequence
		if logs[i].Sequence != "" && logs[j].Sequence != "" {
			return logs[i].Sequence > logs[j].Sequence
		}
		// One or both missing Sequence - fall back to FullTimestamp
		return logs[i].FullTimestamp > logs[j].FullTimestamp
	})
}

// LogStorage implements the LogStorage interface for Badger
type LogStorage struct {
	db     *BadgerDB
	logger arbor.ILogger
}

// NewLogStorage creates a new LogStorage instance
func NewLogStorage(db *BadgerDB, logger arbor.ILogger) interfaces.LogStorage {
	return &LogStorage{
		db:     db,
		logger: logger,
	}
}

func (s *LogStorage) AppendLog(ctx context.Context, jobID string, entry models.LogEntry) error {
	// Set JobIDField directly (primary indexed field)
	entry.JobIDField = jobID

	// Generate unique key using timestamp + atomic sequence counter
	// This ensures uniqueness even when multiple logs are written within the same nanosecond
	seq := atomic.AddUint64(&logSequence, 1)
	now := time.Now().UnixNano()
	key := fmt.Sprintf("%s_%d_%d", jobID, now, seq)

	// Set Sequence field for stable sorting - combines timestamp and sequence
	// Format: 19-digit nanosecond timestamp + underscore + 10-digit zero-padded sequence
	// This ensures lexicographic sorting matches chronological order
	entry.Sequence = fmt.Sprintf("%019d_%010d", now, seq)

	if err := s.db.Store().Insert(key, &entry); err != nil {
		return fmt.Errorf("failed to append log: %w", err)
	}
	return nil
}

func (s *LogStorage) AppendLogs(ctx context.Context, jobID string, entries []models.LogEntry) error {
	for _, entry := range entries {
		if err := s.AppendLog(ctx, jobID, entry); err != nil {
			return err
		}
	}
	return nil
}

func (s *LogStorage) GetLogs(ctx context.Context, jobID string, limit int) ([]models.LogEntry, error) {
	var logs []models.LogEntry
	// Query using the indexed JobIDField
	query := badgerhold.Where("JobIDField").Eq(jobID)

	if err := s.db.Store().Find(&logs, query); err != nil {
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}

	// Sort in-memory to handle logs with/without Sequence field
	sortLogsAsc(logs)

	// Apply limit after sorting
	if limit > 0 && len(logs) > limit {
		logs = logs[:limit]
	}
	return logs, nil
}

func (s *LogStorage) GetLogsByLevel(ctx context.Context, jobID string, level string, limit int) ([]models.LogEntry, error) {
	var logs []models.LogEntry
	query := badgerhold.Where("JobIDField").Eq(jobID).And("Level").Eq(level)

	if err := s.db.Store().Find(&logs, query); err != nil {
		return nil, fmt.Errorf("failed to get logs by level: %w", err)
	}

	// Sort in-memory to handle logs with/without Sequence field (newest first)
	sortLogsDesc(logs)

	// Apply limit after sorting
	if limit > 0 && len(logs) > limit {
		logs = logs[:limit]
	}
	return logs, nil
}

func (s *LogStorage) DeleteLogs(ctx context.Context, jobID string) error {
	if err := s.db.Store().DeleteMatching(&models.LogEntry{}, badgerhold.Where("JobIDField").Eq(jobID)); err != nil {
		return fmt.Errorf("failed to delete logs: %w", err)
	}
	return nil
}

func (s *LogStorage) CountLogs(ctx context.Context, jobID string) (int, error) {
	count, err := s.db.Store().Count(&models.LogEntry{}, badgerhold.Where("JobIDField").Eq(jobID))
	if err != nil {
		return 0, fmt.Errorf("failed to count logs: %w", err)
	}
	return int(count), nil
}

func (s *LogStorage) GetLogsWithOffset(ctx context.Context, jobID string, limit int, offset int) ([]models.LogEntry, error) {
	var logs []models.LogEntry
	query := badgerhold.Where("JobIDField").Eq(jobID)

	if err := s.db.Store().Find(&logs, query); err != nil {
		return nil, fmt.Errorf("failed to get logs with offset: %w", err)
	}

	// Sort in-memory to handle logs with/without Sequence field (newest first)
	sortLogsDesc(logs)

	// Apply offset and limit after sorting
	if offset > 0 {
		if offset >= len(logs) {
			return []models.LogEntry{}, nil
		}
		logs = logs[offset:]
	}
	if limit > 0 && len(logs) > limit {
		logs = logs[:limit]
	}
	return logs, nil
}

func (s *LogStorage) GetLogsByLevelWithOffset(ctx context.Context, jobID string, level string, limit int, offset int) ([]models.LogEntry, error) {
	var logs []models.LogEntry
	query := badgerhold.Where("JobIDField").Eq(jobID).And("Level").Eq(level)

	if err := s.db.Store().Find(&logs, query); err != nil {
		return nil, fmt.Errorf("failed to get logs by level with offset: %w", err)
	}

	// Sort in-memory to handle logs with/without Sequence field (newest first)
	sortLogsDesc(logs)

	// Apply offset and limit after sorting
	if offset > 0 {
		if offset >= len(logs) {
			return []models.LogEntry{}, nil
		}
		logs = logs[offset:]
	}
	if limit > 0 && len(logs) > limit {
		logs = logs[:limit]
	}
	return logs, nil
}

// GetLogsByManagerID retrieves logs for all jobs under a manager
// Note: This fetches all logs and filters in-memory since badgerhold cannot query into map fields
// This is less efficient but rarely used in practice
func (s *LogStorage) GetLogsByManagerID(ctx context.Context, managerID string, limit int) ([]models.LogEntry, error) {
	var allLogs []models.LogEntry
	// Fetch all logs - badgerhold doesn't support querying into map fields
	if err := s.db.Store().Find(&allLogs, badgerhold.Where("JobIDField").Ne("")); err != nil {
		return nil, fmt.Errorf("failed to get all logs: %w", err)
	}

	// Filter in-memory
	var filtered []models.LogEntry
	for _, log := range allLogs {
		if log.GetContext(models.LogCtxManagerID) == managerID {
			filtered = append(filtered, log)
		}
	}

	// Sort in-memory to handle logs with/without Sequence field (newest first)
	sortLogsDesc(filtered)

	// Apply limit after sorting
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, nil
}

// GetLogsByStepID retrieves logs for all jobs under a step
// Note: This fetches all logs and filters in-memory since badgerhold cannot query into map fields
// This is less efficient but rarely used in practice
func (s *LogStorage) GetLogsByStepID(ctx context.Context, stepID string, limit int) ([]models.LogEntry, error) {
	var allLogs []models.LogEntry
	// Fetch all logs - badgerhold doesn't support querying into map fields
	if err := s.db.Store().Find(&allLogs, badgerhold.Where("JobIDField").Ne("")); err != nil {
		return nil, fmt.Errorf("failed to get all logs: %w", err)
	}

	// Filter in-memory
	var filtered []models.LogEntry
	for _, log := range allLogs {
		if log.GetContext(models.LogCtxStepID) == stepID {
			filtered = append(filtered, log)
		}
	}

	// Sort in-memory to handle logs with/without Sequence field (newest first)
	sortLogsDesc(filtered)

	// Apply limit after sorting
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, nil
}
