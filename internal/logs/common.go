package logs

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"

	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// CursorKey represents a cursor position in the sorted log stream
// Format: base64(full_timestamp|job_id|seq) where seq is a per-log tie-breaker
type CursorKey struct {
	FullTimestamp string
	JobID         string
	Seq           int
}

// logIterator represents an iterator for fetching logs from a single job
type logIterator struct {
	jobID     string
	level     string
	order     string
	cursor    *CursorKey
	logs      []models.LogEntry
	nextIdx   int
	ctx       context.Context
	storage   interfaces.LogStorage
	batchSize int
	offset    int // Number of logs already fetched from this job (for pagination)
	fetched   bool
	seq       int  // Per-job sequence number for the next log to emit
	done      bool // Iterator is exhausted
}

// heapItem represents an item for k-way merge
type heapItem struct {
	log       models.LogEntry
	iterator  *logIterator
	seqAtPush int // Per-job sequence number for stable tie-breaking
}

// minHeap implements heap.Interface for ascending order
type minHeap []heapItem

func (h minHeap) Len() int { return len(h) }
func (h minHeap) Less(i, j int) bool {
	// Compare by Sequence field first (combines timestamp + sequence counter)
	// Sequence format: "timestamp_sequence" which sorts lexicographically correctly
	if h[i].log.Sequence != h[j].log.Sequence {
		return h[i].log.Sequence < h[j].log.Sequence
	}
	// Fallback to FullTimestamp for logs without Sequence (backwards compatibility)
	if h[i].log.FullTimestamp != h[j].log.FullTimestamp {
		return h[i].log.FullTimestamp < h[j].log.FullTimestamp
	}
	// Tie-break by job ID
	if h[i].log.JobID() != h[j].log.JobID() {
		return h[i].log.JobID() < h[j].log.JobID()
	}
	return h[i].seqAtPush < h[j].seqAtPush // Use seqAtPush for stable ordering
}
func (h minHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *minHeap) Push(x interface{}) { *h = append(*h, x.(heapItem)) }
func (h *minHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// maxHeap implements heap.Interface for descending order
type maxHeap []heapItem

func (h maxHeap) Len() int { return len(h) }
func (h maxHeap) Less(i, j int) bool {
	// Compare by Sequence field first (combines timestamp + sequence counter)
	// Sequence format: "timestamp_sequence" which sorts lexicographically correctly
	if h[i].log.Sequence != h[j].log.Sequence {
		return h[i].log.Sequence > h[j].log.Sequence
	}
	// Fallback to FullTimestamp for logs without Sequence (backwards compatibility)
	if h[i].log.FullTimestamp != h[j].log.FullTimestamp {
		return h[i].log.FullTimestamp > h[j].log.FullTimestamp
	}
	// Tie-break by job ID
	if h[i].log.JobID() != h[j].log.JobID() {
		return h[i].log.JobID() > h[j].log.JobID()
	}
	return h[i].seqAtPush > h[j].seqAtPush // Use seqAtPush for stable ordering
}
func (h maxHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *maxHeap) Push(x interface{}) { *h = append(*h, x.(heapItem)) }
func (h *maxHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// decodeCursor decodes a base64-encoded cursor string into a CursorKey
func decodeCursor(cursorStr string) (*CursorKey, error) {
	if cursorStr == "" {
		return nil, nil
	}

	// Decode base64
	data, err := base64.StdEncoding.DecodeString(cursorStr)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor encoding: %w", err)
	}

	// Parse format: full_timestamp|job_id|seq
	parts := strings.Split(string(data), "|")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid cursor format")
	}

	seq := 0
	if parts[2] != "" {
		fmt.Sscanf(parts[2], "%d", &seq)
	}

	return &CursorKey{
		FullTimestamp: parts[0],
		JobID:         parts[1],
		Seq:           seq,
	}, nil
}

// encodeCursor encodes a CursorKey into a base64-encoded string
func encodeCursor(key *CursorKey) string {
	if key == nil {
		return ""
	}

	data := fmt.Sprintf("%s|%s|%d", key.FullTimestamp, key.JobID, key.Seq)
	return base64.StdEncoding.EncodeToString([]byte(data))
}

// newLogIterator creates a new log iterator for a job
func newLogIterator(ctx context.Context, jobID, level, order string, cursor *CursorKey, storage interfaces.LogStorage, batchSize int) *logIterator {
	return &logIterator{
		jobID:     jobID,
		level:     level,
		order:     order,
		cursor:    cursor,
		logs:      nil,
		nextIdx:   0,
		ctx:       ctx,
		storage:   storage,
		batchSize: batchSize,
		offset:    0,
		seq:       0,
		fetched:   false,
		done:      false,
	}
}

// fetch fetches the next batch of logs from storage
// Storage returns logs in DESC order (newest first). For ASC order, we reverse the batch.
// For cursor-based pagination, we use offset to skip already-consumed logs.
func (it *logIterator) fetch() error {
	// Don't refetch if we already have logs in the buffer
	if it.fetched && it.nextIdx < len(it.logs) {
		return nil
	}

	// If iterator is done, nothing to fetch
	if it.done {
		return nil
	}

	var rawLogs []models.LogEntry
	var err error

	// Use offset-based pagination for multi-batch fetching
	if it.level != "" && it.level != "all" {
		rawLogs, err = it.storage.GetLogsByLevelWithOffset(it.ctx, it.jobID, it.level, it.batchSize, it.offset)
	} else {
		rawLogs, err = it.storage.GetLogsWithOffset(it.ctx, it.jobID, it.batchSize, it.offset)
	}

	if err != nil {
		return err
	}

	// Associate logs with job ID (set field if missing - shouldn't be needed but ensures consistency)
	for i := range rawLogs {
		if rawLogs[i].JobIDField == "" {
			rawLogs[i].JobIDField = it.jobID
		}
	}

	logs := rawLogs

	// Storage returns DESC order (newest first).
	// If ASC order requested, reverse the batch once (not for every fetch).
	if it.order == "asc" && len(logs) > 0 {
		// Reverse in-place
		for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
			logs[i], logs[j] = logs[j], logs[i]
		}
	}

	// Filter logs based on cursor - only for the first batch when offset is 0
	// Cursor filtering is done AFTER ordering reversal to ensure correct comparison
	if it.cursor != nil && it.offset == 0 {
		filtered := make([]models.LogEntry, 0, len(logs))
		for idx, log := range logs {
			// Filter based on cursor position
			skip := false
			if it.order == "asc" {
				// For ASC: skip logs <= cursor (we want logs AFTER the cursor)
				if log.FullTimestamp < it.cursor.FullTimestamp {
					skip = true
				} else if log.FullTimestamp == it.cursor.FullTimestamp {
					// Same timestamp, check per-entry sequence for tie-breaking
					candidateSeq := it.seq + idx
					if it.cursor.JobID == it.jobID && candidateSeq <= it.cursor.Seq {
						skip = true
					}
				}
			} else { // desc
				// For DESC: skip logs >= cursor (we want logs BEFORE the cursor)
				if log.FullTimestamp > it.cursor.FullTimestamp {
					skip = true
				} else if log.FullTimestamp == it.cursor.FullTimestamp {
					// Same timestamp, check per-entry sequence for tie-breaking
					candidateSeq := it.seq + idx
					if it.cursor.JobID == it.jobID && candidateSeq <= it.cursor.Seq {
						skip = true
					}
				}
			}

			if !skip {
				filtered = append(filtered, log)
			}
		}
		logs = filtered
	}

	it.logs = logs
	it.nextIdx = 0
	it.fetched = true

	// Update offset using raw count (before filtering)
	it.offset += len(rawLogs)

	// If we got fewer logs than requested, we're done
	if len(rawLogs) < it.batchSize {
		it.done = true
	}

	return nil
}

// next returns the next log from the iterator
func (it *logIterator) next() (*models.LogEntry, error) {
	// Fetch more logs if buffer is empty
	if it.nextIdx >= len(it.logs) {
		if err := it.fetch(); err != nil {
			return nil, err
		}
		// If still no logs after fetch, iterator is exhausted
		if it.nextIdx >= len(it.logs) {
			return nil, nil
		}
	}

	// Return next log and advance
	log := it.logs[it.nextIdx]
	it.nextIdx++

	// Increment per-job sequence for cursor encoding
	it.seq++

	return &log, nil
}

// sortLogsByTimestamp sorts logs chronologically (oldest-first)
// Uses Sequence field (timestamp_counter) for accurate sorting
func sortLogsByTimestamp(logs []models.LogEntry) {
	if len(logs) <= 1 {
		return
	}

	// Use stable sort for O(n log n) performance and to preserve order for equal timestamps
	sort.SliceStable(logs, func(i, j int) bool {
		// Sort by Sequence field (combines timestamp + sequence counter)
		// Falls back to FullTimestamp for backwards compatibility
		if logs[i].Sequence != logs[j].Sequence {
			return logs[i].Sequence < logs[j].Sequence
		}
		return logs[i].FullTimestamp < logs[j].FullTimestamp
	})
}
