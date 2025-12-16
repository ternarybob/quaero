package badger

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/timshannon/badgerhold/v4"
)

// logSequence is a global counter to ensure unique log keys even within the same nanosecond
var logSequence uint64

// stepLineCounters tracks per-step line number counters
// Key: step_id (or jobID as fallback), Value: pointer to uint64 counter
// All workers logging to the same step share one counter for sequential line numbers
var stepLineCounters sync.Map

// sortLogsAsc sorts logs in ascending order (oldest first)
// For logs from the same step, sorts by LineNumber (per-step sequential ordering)
// For logs from different steps, sorts by Sequence (global timestamp ordering)
func sortLogsAsc(logs []models.LogEntry) {
	sort.SliceStable(logs, func(i, j int) bool {
		// If both logs are from the same step, use LineNumber for proper ordering
		stepI := logs[i].GetContext(models.LogCtxStepID)
		stepJ := logs[j].GetContext(models.LogCtxStepID)
		if stepI != "" && stepI == stepJ && logs[i].LineNumber > 0 && logs[j].LineNumber > 0 {
			return logs[i].LineNumber < logs[j].LineNumber
		}
		// For cross-step logs, use Sequence (timestamp-based global ordering)
		if logs[i].Sequence != "" && logs[j].Sequence != "" {
			return logs[i].Sequence < logs[j].Sequence
		}
		// Final fallback to FullTimestamp
		return logs[i].FullTimestamp < logs[j].FullTimestamp
	})
}

// sortLogsDesc sorts logs in descending order (newest first)
// For logs from the same step, sorts by LineNumber (per-step sequential ordering)
// For logs from different steps, sorts by Sequence (global timestamp ordering)
func sortLogsDesc(logs []models.LogEntry) {
	sort.SliceStable(logs, func(i, j int) bool {
		// If both logs are from the same step, use LineNumber for proper ordering
		stepI := logs[i].GetContext(models.LogCtxStepID)
		stepJ := logs[j].GetContext(models.LogCtxStepID)
		if stepI != "" && stepI == stepJ && logs[i].LineNumber > 0 && logs[j].LineNumber > 0 {
			return logs[i].LineNumber > logs[j].LineNumber
		}
		// For cross-step logs, use Sequence (timestamp-based global ordering)
		if logs[i].Sequence != "" && logs[j].Sequence != "" {
			return logs[i].Sequence > logs[j].Sequence
		}
		// Final fallback to FullTimestamp
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

// getNextLineNumber returns the next line number for a step (1-based, atomically incremented)
// Uses step_id as counter key so all workers in the same step share sequential line numbers.
// Falls back to jobID if no step_id is available.
// On first call for a step, it queries the DB to find the current max LineNumber.
func (s *LogStorage) getNextLineNumber(ctx context.Context, counterKey string) int {
	// Try to get existing counter
	if counterPtr, ok := stepLineCounters.Load(counterKey); ok {
		return int(atomic.AddUint64(counterPtr.(*uint64), 1))
	}

	// First access for this step - need to initialize from DB
	// Query all logs and filter by step_id in context (badgerhold can't query map fields)
	var allLogs []models.LogEntry
	if err := s.db.Store().Find(&allLogs, badgerhold.Where("JobIDField").Ne("")); err != nil {
		// On error, start from 1
		var counter uint64 = 1
		stepLineCounters.Store(counterKey, &counter)
		return 1
	}

	// Find max LineNumber for logs with matching step_id
	var maxLineNumber uint64 = 0
	for _, log := range allLogs {
		logStepID := log.GetContext(models.LogCtxStepID)
		if logStepID == counterKey || (logStepID == "" && log.JobIDField == counterKey) {
			if uint64(log.LineNumber) > maxLineNumber {
				maxLineNumber = uint64(log.LineNumber)
			}
		}
	}

	// Initialize counter at maxLineNumber (next call will increment to max+1)
	// Use LoadOrStore to handle race condition where another goroutine initialized first
	newCounter := maxLineNumber
	actual, loaded := stepLineCounters.LoadOrStore(counterKey, &newCounter)
	if loaded {
		// Another goroutine initialized first, use their counter
		return int(atomic.AddUint64(actual.(*uint64), 1))
	}

	// We initialized, increment and return
	return int(atomic.AddUint64(&newCounter, 1))
}

func (s *LogStorage) AppendLog(ctx context.Context, jobID string, entry models.LogEntry) (int, error) {
	// Set JobIDField directly (primary indexed field)
	entry.JobIDField = jobID

	// Normalize level to 3-letter format for consistent storage/query
	// API uses: "info", "warn", "error", "debug"
	// Storage uses: "INF", "WRN", "ERR", "DBG"
	entry.Level = normalizeLevel(entry.Level)

	// Determine counter key: use step_id for per-step sequential line numbers
	// All workers in the same step share one counter for proper interleaved ordering
	// Falls back to jobID if no step context is available
	counterKey := entry.GetContext(models.LogCtxStepID)
	if counterKey == "" {
		counterKey = jobID
	}

	// Get next per-step line number (1-based, contiguous within each step)
	entry.LineNumber = s.getNextLineNumber(ctx, counterKey)

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
		return 0, fmt.Errorf("failed to append log: %w", err)
	}
	return entry.LineNumber, nil
}

func (s *LogStorage) AppendLogs(ctx context.Context, jobID string, entries []models.LogEntry) error {
	for _, entry := range entries {
		if _, err := s.AppendLog(ctx, jobID, entry); err != nil {
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

	// Sort in-memory (newest first) to handle logs with/without Sequence field
	// All log retrieval methods return DESC order (newest first) for consistency
	sortLogsDesc(logs)

	// Apply limit after sorting - returns newest N logs
	if limit > 0 && len(logs) > limit {
		logs = logs[:limit]
	}
	return logs, nil
}

// normalizeLevel converts API level names to the 3-letter codes used in storage
// API uses: "info", "warn", "error", "debug"
// Storage uses: "INF", "WRN", "ERR", "DBG"
func normalizeLevel(level string) string {
	switch strings.ToLower(level) {
	case "info", "inf":
		return "INF"
	case "warn", "warning", "wrn":
		return "WRN"
	case "error", "err":
		return "ERR"
	case "debug", "dbg":
		return "DBG"
	default:
		// Return as-is if already in correct format or unknown
		return strings.ToUpper(level)
	}
}

func (s *LogStorage) GetLogsByLevel(ctx context.Context, jobID string, level string, limit int) ([]models.LogEntry, error) {
	var logs []models.LogEntry
	// Normalize level to 3-letter format used in storage
	normalizedLevel := normalizeLevel(level)

	// Get levels to include based on hierarchical filtering:
	// - "all" or "DBG" -> all levels (DBG, INF, WRN, ERR)
	// - "INF" -> INF, WRN, ERR
	// - "WRN" -> WRN, ERR
	// - "ERR" -> ERR only
	includedLevels := getLevelsAtOrAbove(normalizedLevel)

	// Query all logs for this job and filter by level in-memory
	var allLogs []models.LogEntry
	query := badgerhold.Where("JobIDField").Eq(jobID)
	if err := s.db.Store().Find(&allLogs, query); err != nil {
		return nil, fmt.Errorf("failed to get logs by level: %w", err)
	}

	// Filter by level
	for _, log := range allLogs {
		if _, ok := includedLevels[log.Level]; ok {
			logs = append(logs, log)
		}
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
	// Clear line number counters that might be associated with this job
	// Note: Counter is keyed by step_id, but we also try jobID as fallback
	stepLineCounters.Delete(jobID)
	return nil
}

// ClearStepLineCounter clears the line number counter for a step.
// Call this when a step is deleted or reset to ensure fresh line numbering.
func (s *LogStorage) ClearStepLineCounter(stepID string) {
	stepLineCounters.Delete(stepID)
}

func (s *LogStorage) CountLogs(ctx context.Context, jobID string) (int, error) {
	count, err := s.db.Store().Count(&models.LogEntry{}, badgerhold.Where("JobIDField").Eq(jobID))
	if err != nil {
		return 0, fmt.Errorf("failed to count logs: %w", err)
	}
	return int(count), nil
}

func (s *LogStorage) CountLogsByLevel(ctx context.Context, jobID string, level string) (int, error) {
	normalizedLevel := normalizeLevel(level)

	// Get levels to include based on hierarchical filtering
	includedLevels := getLevelsAtOrAbove(normalizedLevel)

	// Query all logs for this job and count by level in-memory
	var allLogs []models.LogEntry
	query := badgerhold.Where("JobIDField").Eq(jobID)
	if err := s.db.Store().Find(&allLogs, query); err != nil {
		return 0, fmt.Errorf("failed to count logs by level: %w", err)
	}

	count := 0
	for _, log := range allLogs {
		if _, ok := includedLevels[log.Level]; ok {
			count++
		}
	}
	return count, nil
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
	// Normalize level to 3-letter format used in storage
	normalizedLevel := normalizeLevel(level)

	// Get levels to include based on hierarchical filtering:
	// - "all" or "DBG" -> all levels (DBG, INF, WRN, ERR)
	// - "INF" -> INF, WRN, ERR
	// - "WRN" -> WRN, ERR
	// - "ERR" -> ERR only
	includedLevels := getLevelsAtOrAbove(normalizedLevel)

	// Query all logs for this job and filter by level in-memory
	// (badgerhold doesn't support IN queries easily)
	var allLogs []models.LogEntry
	query := badgerhold.Where("JobIDField").Eq(jobID)
	if err := s.db.Store().Find(&allLogs, query); err != nil {
		return nil, fmt.Errorf("failed to get logs by level with offset: %w", err)
	}

	// Filter by level
	for _, log := range allLogs {
		if _, ok := includedLevels[log.Level]; ok {
			logs = append(logs, log)
		}
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

// getLevelsAtOrAbove returns a set of levels at or above the given level
// Level hierarchy: DBG < INF < WRN < ERR
func getLevelsAtOrAbove(level string) map[string]bool {
	switch level {
	case "ERR":
		return map[string]bool{"ERR": true}
	case "WRN":
		return map[string]bool{"WRN": true, "ERR": true}
	case "INF":
		return map[string]bool{"INF": true, "WRN": true, "ERR": true}
	case "DBG":
		return map[string]bool{"DBG": true, "INF": true, "WRN": true, "ERR": true}
	default:
		// For unknown levels or "all", include everything
		return map[string]bool{"DBG": true, "INF": true, "WRN": true, "ERR": true}
	}
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
