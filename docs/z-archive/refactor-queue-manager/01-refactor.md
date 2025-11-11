# Quaero Queue Management Redesign - Complete Implementation Guide

**Date:** 2025-11-03  
**Version:** 4.0  
**Status:** Ready for Implementation

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Problem Analysis](#problem-analysis)
3. [Solution Architecture](#solution-architecture)
4. [Database Schema](#database-schema)
5. [Implementation Files](#implementation-files)
6. [UI Implementation](#ui-implementation)
7. [Migration Steps](#migration-steps)
8. [Testing & Validation](#testing--validation)
9. [Rollback Plan](#rollback-plan)

---

## Executive Summary

### What Failed

Previous attempts (redesign-job-queue-3, redesign-job-queue-post, refactor-queue-management) created custom queue management solutions that:
- Replaced goqite with custom logic (violating DRY and increasing complexity)
- Mixed queue management with job execution logic
- Created overly complex UI with chevrons and excessive detail
- Hardcoded test URLs in production code
- Failed to properly implement parent/child job hierarchy

### The Fix

This redesign:
1. **Uses goqite as the ONLY queue manager** (no custom queue logic)
2. **Separates concerns**: goqite = queue, jobs table = metadata, executors = business logic
3. **Simplifies UI**: Simple table → click for details → modal with children
4. **Implements proper parent/child structure** with pre/core/post phases
5. **Removes all test URLs** from production code

### Success Criteria

- ✅ Jobs enqueue, process, and complete successfully
- ✅ UI shows parent jobs by default
- ✅ Child jobs visible only when parent is clicked
- ✅ Progress tracking works correctly
- ✅ Logs display in real-time
- ✅ No test URLs in production code
- ✅ goqite handles ALL queue operations

---

## Problem Analysis

### Root Causes of Failure

1. **Abstraction Violation**
   - Custom queue logic built on top of goqite
   - goqite's features (persistence, retries, visibility timeout) duplicated or ignored
   - Multiple sources of truth for queue state

2. **Tight Coupling**
   - Job execution logic mixed with queue management
   - UI directly coupled to queue internals
   - Hard to test individual components

3. **UI Complexity**
   - Chevron-based expansion in tables
   - Too much information shown at once
   - Poor UX for drilling into job details

4. **Configuration Issues**
   - Test URLs hardcoded in production paths
   - No clear separation of environments
   - Configuration scattered across codebase

5. **Missing Job Hierarchy**
   - No clear parent/child relationship
   - Pre/core/post phases not properly modeled
   - Child job spawning during execution not supported

---

## Solution Architecture

### Design Principles

```
PRINCIPLE 1: goqite is the ONLY queue manager
PRINCIPLE 2: Jobs table stores metadata, NOT queue state
PRINCIPLE 3: UI shows parent jobs by default, children on demand
PRINCIPLE 4: Job phases (pre/core/post) are explicit and separate
PRINCIPLE 5: Zero test URLs in production code
```

### Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                         UI Layer                             │
│  • Parent jobs table (simple, no chevrons)                  │
│  • Click parent → modal with children                        │
│  • Logs view                                                 │
└─────────────────────────────────────────────────────────────┘
                              ↕
┌─────────────────────────────────────────────────────────────┐
│                      API/Handler Layer                       │
│  GET  /api/jobs                  → List parent jobs         │
│  GET  /api/jobs/{id}/children    → List child jobs          │
│  GET  /api/jobs/{id}/logs        → Get job logs             │
│  POST /api/jobs                  → Create new job           │
└─────────────────────────────────────────────────────────────┘
                              ↕
┌─────────────────────────────────────────────────────────────┐
│                      Job Manager Layer                       │
│  • CreateParentJob()                                         │
│  • CreateChildJob()                                          │
│  • ListParentJobs()                                          │
│  • ListChildJobs()                                           │
│  • UpdateJobStatus()                                         │
│  • UpdateJobProgress()                                       │
│  • AddJobLog()                                               │
└─────────────────────────────────────────────────────────────┘
                              ↕
┌─────────────────────────────────────────────────────────────┐
│                   Queue Manager (goqite)                     │
│  • Enqueue(message)        ← ONLY entry point               │
│  • Receive()               ← ONLY retrieval                  │
│  • Delete(message)         ← ONLY completion                 │
│  • Extend(message)         ← Keep-alive for long jobs        │
└─────────────────────────────────────────────────────────────┘
                              ↕
┌─────────────────────────────────────────────────────────────┐
│                      Worker Pool Layer                       │
│  • Pulls messages from goqite                                │
│  • Routes to appropriate executor                            │
│  • Updates job status                                        │
│  • Adds logs                                                 │
│  • Deletes message on completion                             │
└─────────────────────────────────────────────────────────────┘
                              ↕
┌─────────────────────────────────────────────────────────────┐
│                    Job Executor Layer                        │
│  • CrawlerExecutor                                           │
│  • SummarizerExecutor                                        │
│  • CleanupExecutor                                           │
│  • Each executor creates child jobs as needed                │
└─────────────────────────────────────────────────────────────┘
                              ↕
┌─────────────────────────────────────────────────────────────┐
│                     SQLite Database                          │
│  • goqite_queue (managed by goqite - DO NOT TOUCH)          │
│  • jobs (our metadata)                                       │
│  • job_logs (our logs)                                       │
└─────────────────────────────────────────────────────────────┘
```

### Data Flow Example: Crawler Job

```
1. User creates crawler job via UI
   ↓
2. API handler calls JobManager.CreateParentJob("crawler", payload)
   ↓
3. JobManager:
   a. Creates parent job record in jobs table (status=pending)
   b. Calls QueueManager.Enqueue(message)
   ↓
4. goqite stores message in queue
   ↓
5. Worker pool calls QueueManager.Receive()
   ↓
6. goqite returns message
   ↓
7. Worker:
   a. Updates job status to "running"
   b. Calls CrawlerExecutor.Execute(jobID, payload)
   ↓
8. CrawlerExecutor:
   a. Creates child jobs for each URL (phase="core")
   b. Each child job enqueued via QueueManager.Enqueue()
   c. Updates progress as children complete
   ↓
9. Worker marks job as completed
   ↓
10. Worker calls deleteFn to remove message from goqite
```

---

## Database Schema

### Migration File: `migrations/008_redesign_job_queue.sql`

```sql
-- Drop old tables if they exist from previous attempts
DROP TABLE IF EXISTS queue_messages;
DROP TABLE IF EXISTS queue_state;
DROP TABLE IF EXISTS job_queue;

-- Jobs metadata table
-- This is NOT the queue - goqite manages the queue
-- This table stores job metadata and relationships
CREATE TABLE IF NOT EXISTS jobs (
    id TEXT PRIMARY KEY,
    parent_id TEXT REFERENCES jobs(id) ON DELETE CASCADE,
    job_type TEXT NOT NULL,
    phase TEXT NOT NULL CHECK (phase IN ('pre', 'core', 'post')),
    status TEXT NOT NULL CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    payload TEXT,  -- JSON payload
    result TEXT,   -- JSON result
    error TEXT,
    progress_current INTEGER DEFAULT 0,
    progress_total INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_jobs_parent ON jobs(parent_id);
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_created ON jobs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_jobs_type ON jobs(job_type);

-- Job logs table
CREATE TABLE IF NOT EXISTS job_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    level TEXT NOT NULL CHECK (level IN ('debug', 'info', 'warn', 'error')),
    message TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_job_logs_job ON job_logs(job_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_job_logs_timestamp ON job_logs(timestamp DESC);

-- goqite will create its own table (goqite_jobs) - we don't touch it
```

### Schema Notes

1. **Jobs Table**
   - `parent_id = NULL` indicates a parent job
   - `parent_id != NULL` indicates a child job
   - `phase` indicates job lifecycle phase (pre/core/post)
   - `status` tracks execution state
   - `payload` stores job-specific input data (JSON)
   - `result` stores job output (JSON)
   - `progress_current/progress_total` for UI progress bars

2. **Job Logs Table**
   - Separate from jobs for performance
   - Timestamped for chronological viewing
   - Level-based filtering (debug/info/warn/error)

3. **goqite Table**
   - Created and managed by goqite library
   - **DO NOT** modify or query directly
   - Use QueueManager wrapper only

---

## Implementation Files

### 1. Queue Types

**File:** `internal/queue/types.go`

```go
package queue

import "encoding/json"

// Message is the ONLY structure that goes into goqite
// Keep it simple - just enough to route the job
type Message struct {
    JobID   string          `json:"job_id"`   // References jobs.id
    Type    string          `json:"type"`     // Job type for executor routing
    Payload json.RawMessage `json:"payload"`  // Job-specific data (passed through)
}
```

### 2. Queue Manager (goqite Wrapper)

**File:** `internal/queue/manager.go`

```go
package queue

import (
    "context"
    "database/sql"
    "encoding/json"
    "time"

    "github.com/maragudk/goqite"
)

// Manager is a thin wrapper around goqite
// It provides ONLY queue operations, no business logic
type Manager struct {
    q *goqite.Queue
}

// NewManager creates a new queue manager
func NewManager(db *sql.DB, queueName string) (*Manager, error) {
    q := goqite.New(goqite.NewOpts{
        DB:   db,
        Name: queueName,
    })

    // Setup creates the goqite table if it doesn't exist
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := q.Setup(ctx); err != nil {
        return nil, err
    }

    return &Manager{q: q}, nil
}

// Enqueue adds a message to the queue
// This is the ONLY way to add jobs to the queue
func (m *Manager) Enqueue(ctx context.Context, msg Message) error {
    data, err := json.Marshal(msg)
    if err != nil {
        return err
    }

    return m.q.Send(ctx, goqite.Message{
        Body: data,
    })
}

// Receive pulls the next message from the queue
// Returns the message and a delete function to call after processing
func (m *Manager) Receive(ctx context.Context) (*Message, func() error, error) {
    gMsg, err := m.q.Receive(ctx)
    if err != nil {
        return nil, nil, err
    }

    var msg Message
    if err := json.Unmarshal(gMsg.Body, &msg); err != nil {
        return nil, nil, err
    }

    // Return delete function for worker to call after successful processing
    deleteFn := func() error {
        return m.q.Delete(ctx, gMsg.ID)
    }

    return &msg, deleteFn, nil
}

// Extend extends the visibility timeout for a long-running job
// Call this periodically during job execution to prevent re-delivery
func (m *Manager) Extend(ctx context.Context, messageID goqite.ID, duration time.Duration) error {
    return m.q.Extend(ctx, messageID, duration)
}

// Close closes the queue manager
func (m *Manager) Close() error {
    // goqite doesn't require explicit close, but we provide it for consistency
    return nil
}
```

### 3. Job Manager

**File:** `internal/jobs/manager.go`

```go
package jobs

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/ternarybob/quaero/internal/queue"
)

// Manager handles job metadata and lifecycle
// It does NOT manage the queue - that's goqite's job
type Manager struct {
    db    *sql.DB
    queue *queue.Manager
}

func NewManager(db *sql.DB, queue *queue.Manager) *Manager {
    return &Manager{
        db:    db,
        queue: queue,
    }
}

// Job represents job metadata
type Job struct {
    ID              string     `json:"id"`
    ParentID        *string    `json:"parent_id,omitempty"`
    Type            string     `json:"job_type"`
    Phase           string     `json:"phase"`
    Status          string     `json:"status"`
    CreatedAt       time.Time  `json:"created_at"`
    StartedAt       *time.Time `json:"started_at,omitempty"`
    CompletedAt     *time.Time `json:"completed_at,omitempty"`
    Payload         string     `json:"payload,omitempty"`
    Result          string     `json:"result,omitempty"`
    Error           *string    `json:"error,omitempty"`
    ProgressCurrent int        `json:"progress_current"`
    ProgressTotal   int        `json:"progress_total"`
}

// CreateParentJob creates a new parent job and enqueues it
func (m *Manager) CreateParentJob(ctx context.Context, jobType string, payload interface{}) (string, error) {
    jobID := uuid.New().String()

    payloadJSON, err := json.Marshal(payload)
    if err != nil {
        return "", fmt.Errorf("marshal payload: %w", err)
    }

    // Create job record
    _, err = m.db.ExecContext(ctx, `
        INSERT INTO jobs (id, parent_id, job_type, phase, status, created_at, payload)
        VALUES (?, NULL, ?, 'core', 'pending', ?, ?)
    `, jobID, jobType, time.Now(), string(payloadJSON))

    if err != nil {
        return "", fmt.Errorf("create job record: %w", err)
    }

    // Enqueue the job
    if err := m.queue.Enqueue(ctx, queue.Message{
        JobID:   jobID,
        Type:    jobType,
        Payload: payloadJSON,
    }); err != nil {
        return "", fmt.Errorf("enqueue job: %w", err)
    }

    return jobID, nil
}

// CreateChildJob creates a child job and enqueues it
func (m *Manager) CreateChildJob(ctx context.Context, parentID, jobType, phase string, payload interface{}) (string, error) {
    jobID := uuid.New().String()

    payloadJSON, err := json.Marshal(payload)
    if err != nil {
        return "", fmt.Errorf("marshal payload: %w", err)
    }

    // Create job record
    _, err = m.db.ExecContext(ctx, `
        INSERT INTO jobs (id, parent_id, job_type, phase, status, created_at, payload)
        VALUES (?, ?, ?, ?, 'pending', ?, ?)
    `, jobID, parentID, jobType, phase, time.Now(), string(payloadJSON))

    if err != nil {
        return "", fmt.Errorf("create job record: %w", err)
    }

    // Enqueue the job
    if err := m.queue.Enqueue(ctx, queue.Message{
        JobID:   jobID,
        Type:    jobType,
        Payload: payloadJSON,
    }); err != nil {
        return "", fmt.Errorf("enqueue job: %w", err)
    }

    return jobID, nil
}

// GetJob retrieves a job by ID
func (m *Manager) GetJob(ctx context.Context, jobID string) (*Job, error) {
    var job Job
    var parentID sql.NullString
    var startedAt, completedAt sql.NullTime
    var err, result sql.NullString

    row := m.db.QueryRowContext(ctx, `
        SELECT id, parent_id, job_type, phase, status, created_at, started_at, 
               completed_at, payload, result, error, progress_current, progress_total
        FROM jobs
        WHERE id = ?
    `, jobID)

    if err := row.Scan(
        &job.ID, &parentID, &job.Type, &job.Phase, &job.Status,
        &job.CreatedAt, &startedAt, &completedAt,
        &job.Payload, &result, &err,
        &job.ProgressCurrent, &job.ProgressTotal,
    ); err != nil {
        return nil, err
    }

    if parentID.Valid {
        job.ParentID = &parentID.String
    }
    if startedAt.Valid {
        job.StartedAt = &startedAt.Time
    }
    if completedAt.Valid {
        job.CompletedAt = &completedAt.Time
    }
    if result.Valid {
        job.Result = result.String
    }
    if err.Valid {
        job.Error = &err.String
    }

    return &job, nil
}

// ListParentJobs returns all parent jobs (parent_id IS NULL)
func (m *Manager) ListParentJobs(ctx context.Context, limit, offset int) ([]Job, error) {
    rows, err := m.db.QueryContext(ctx, `
        SELECT id, job_type, phase, status, created_at, started_at, completed_at,
               progress_current, progress_total, error
        FROM jobs
        WHERE parent_id IS NULL
        ORDER BY created_at DESC
        LIMIT ? OFFSET ?
    `, limit, offset)

    if err != nil {
        return nil, err
    }
    defer rows.Close()

    return m.scanJobs(rows)
}

// ListChildJobs returns all child jobs for a parent
func (m *Manager) ListChildJobs(ctx context.Context, parentID string) ([]Job, error) {
    rows, err := m.db.QueryContext(ctx, `
        SELECT id, parent_id, job_type, phase, status, created_at, started_at,
               completed_at, progress_current, progress_total, error
        FROM jobs
        WHERE parent_id = ?
        ORDER BY created_at ASC
    `, parentID)

    if err != nil {
        return nil, err
    }
    defer rows.Close()

    return m.scanJobs(rows)
}

// UpdateJobStatus updates the job status
func (m *Manager) UpdateJobStatus(ctx context.Context, jobID, status string) error {
    now := time.Now()

    query := "UPDATE jobs SET status = ?"
    args := []interface{}{status}

    if status == "running" {
        query += ", started_at = ?"
        args = append(args, now)
    } else if status == "completed" || status == "failed" || status == "cancelled" {
        query += ", completed_at = ?"
        args = append(args, now)
    }

    query += " WHERE id = ?"
    args = append(args, jobID)

    _, err := m.db.ExecContext(ctx, query, args...)
    return err
}

// UpdateJobProgress updates job progress
func (m *Manager) UpdateJobProgress(ctx context.Context, jobID string, current, total int) error {
    _, err := m.db.ExecContext(ctx, `
        UPDATE jobs SET progress_current = ?, progress_total = ?
        WHERE id = ?
    `, current, total, jobID)
    return err
}

// SetJobError sets job error message and marks as failed
func (m *Manager) SetJobError(ctx context.Context, jobID string, errorMsg string) error {
    _, err := m.db.ExecContext(ctx, `
        UPDATE jobs SET status = 'failed', error = ?, completed_at = ?
        WHERE id = ?
    `, errorMsg, time.Now(), jobID)
    return err
}

// SetJobResult sets job result data
func (m *Manager) SetJobResult(ctx context.Context, jobID string, result interface{}) error {
    resultJSON, err := json.Marshal(result)
    if err != nil {
        return fmt.Errorf("marshal result: %w", err)
    }

    _, err = m.db.ExecContext(ctx, `
        UPDATE jobs SET result = ? WHERE id = ?
    `, string(resultJSON), jobID)
    return err
}

// AddJobLog adds a log entry for a job
func (m *Manager) AddJobLog(ctx context.Context, jobID, level, message string) error {
    _, err := m.db.ExecContext(ctx, `
        INSERT INTO job_logs (job_id, timestamp, level, message)
        VALUES (?, ?, ?, ?)
    `, jobID, time.Now(), level, message)
    return err
}

// GetJobLogs retrieves logs for a job
func (m *Manager) GetJobLogs(ctx context.Context, jobID string, limit int) ([]JobLog, error) {
    rows, err := m.db.QueryContext(ctx, `
        SELECT id, job_id, timestamp, level, message
        FROM job_logs
        WHERE job_id = ?
        ORDER BY timestamp DESC
        LIMIT ?
    `, jobID, limit)

    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var logs []JobLog
    for rows.Next() {
        var log JobLog
        if err := rows.Scan(&log.ID, &log.JobID, &log.Timestamp, &log.Level, &log.Message); err != nil {
            return nil, err
        }
        logs = append(logs, log)
    }

    return logs, rows.Err()
}

// JobLog represents a job log entry
type JobLog struct {
    ID        int       `json:"id"`
    JobID     string    `json:"job_id"`
    Timestamp time.Time `json:"timestamp"`
    Level     string    `json:"level"`
    Message   string    `json:"message"`
}

// scanJobs helper function to scan job rows
func (m *Manager) scanJobs(rows *sql.Rows) ([]Job, error) {
    var jobs []Job

    for rows.Next() {
        var job Job
        var parentID sql.NullString
        var startedAt, completedAt sql.NullTime
        var errorMsg sql.NullString

        if err := rows.Scan(
            &job.ID, &job.Type, &job.Phase, &job.Status,
            &job.CreatedAt, &startedAt, &completedAt,
            &job.ProgressCurrent, &job.ProgressTotal, &errorMsg,
        ); err != nil {
            // Try with parent_id column if it exists
            if err := rows.Scan(
                &job.ID, &parentID, &job.Type, &job.Phase, &job.Status,
                &job.CreatedAt, &startedAt, &completedAt,
                &job.ProgressCurrent, &job.ProgressTotal, &errorMsg,
            ); err != nil {
                return nil, err
            }
        }

        if parentID.Valid {
            job.ParentID = &parentID.String
        }
        if startedAt.Valid {
            job.StartedAt = &startedAt.Time
        }
        if completedAt.Valid {
            job.CompletedAt = &completedAt.Time
        }
        if errorMsg.Valid {
            job.Error = &errorMsg.String
        }

        jobs = append(jobs, job)
    }

    return jobs, rows.Err()
}
```

### 4. Worker Pool

**File:** `internal/queue/worker.go`

```go
package queue

import (
    "context"
    "fmt"
    "log"
    "sync"

    "github.com/ternarybob/quaero/internal/jobs"
)

// Executor interface for job execution
type Executor interface {
    Execute(ctx context.Context, jobID string, payload []byte) error
}

// WorkerPool manages a pool of workers that process jobs
type WorkerPool struct {
    queueMgr   *Manager
    jobMgr     *jobs.Manager
    executors  map[string]Executor
    numWorkers int
    wg         sync.WaitGroup
    ctx        context.Context
    cancel     context.CancelFunc
}

func NewWorkerPool(queueMgr *Manager, jobMgr *jobs.Manager, numWorkers int) *WorkerPool {
    ctx, cancel := context.WithCancel(context.Background())

    return &WorkerPool{
        queueMgr:   queueMgr,
        jobMgr:     jobMgr,
        executors:  make(map[string]Executor),
        numWorkers: numWorkers,
        ctx:        ctx,
        cancel:     cancel,
    }
}

// RegisterExecutor registers an executor for a job type
func (wp *WorkerPool) RegisterExecutor(jobType string, executor Executor) {
    wp.executors[jobType] = executor
}

// Start starts the worker pool
func (wp *WorkerPool) Start() {
    log.Printf("Starting worker pool with %d workers", wp.numWorkers)

    for i := 0; i < wp.numWorkers; i++ {
        wp.wg.Add(1)
        go wp.worker(i)
    }
}

// Stop stops the worker pool gracefully
func (wp *WorkerPool) Stop() {
    log.Println("Stopping worker pool...")
    wp.cancel()
    wp.wg.Wait()
    log.Println("Worker pool stopped")
}

// worker is the main worker loop
func (wp *WorkerPool) worker(id int) {
    defer wp.wg.Done()

    log.Printf("Worker %d started", id)

    for {
        select {
        case <-wp.ctx.Done():
            log.Printf("Worker %d stopping", id)
            return
        default:
            wp.processNextJob(id)
        }
    }
}

// processNextJob processes the next job from the queue
func (wp *WorkerPool) processNextJob(workerID int) {
    // Receive next message from queue
    msg, deleteFn, err := wp.queueMgr.Receive(wp.ctx)
    if err != nil {
        // No message available or context cancelled
        return
    }

    log.Printf("Worker %d processing job %s (type: %s)", workerID, msg.JobID, msg.Type)

    // Update job status to running
    if err := wp.jobMgr.UpdateJobStatus(wp.ctx, msg.JobID, "running"); err != nil {
        log.Printf("Worker %d: Failed to update job status: %v", workerID, err)
    }

    if err := wp.jobMgr.AddJobLog(wp.ctx, msg.JobID, "info", fmt.Sprintf("Job started on worker %d", workerID)); err != nil {
        log.Printf("Worker %d: Failed to add job log: %v", workerID, err)
    }

    // Get executor for job type
    executor, ok := wp.executors[msg.Type]
    if !ok {
        errMsg := fmt.Sprintf("No executor registered for job type: %s", msg.Type)
        log.Printf("Worker %d: %s", workerID, errMsg)
        wp.jobMgr.SetJobError(wp.ctx, msg.JobID, errMsg)
        wp.jobMgr.AddJobLog(wp.ctx, msg.JobID, "error", errMsg)
        deleteFn() // Remove from queue
        return
    }

    // Execute the job
    err = executor.Execute(wp.ctx, msg.JobID, msg.Payload)

    if err != nil {
        // Job failed
        log.Printf("Worker %d: Job %s failed: %v", workerID, msg.JobID, err)
        wp.jobMgr.SetJobError(wp.ctx, msg.JobID, err.Error())
        wp.jobMgr.AddJobLog(wp.ctx, msg.JobID, "error", err.Error())
    } else {
        // Job succeeded
        log.Printf("Worker %d: Job %s completed successfully", workerID, msg.JobID)
        wp.jobMgr.UpdateJobStatus(wp.ctx, msg.JobID, "completed")
        wp.jobMgr.AddJobLog(wp.ctx, msg.JobID, "info", "Job completed successfully")
    }

    // Remove message from queue
    if err := deleteFn(); err != nil {
        log.Printf("Worker %d: Failed to delete message: %v", workerID, err)
    }
}
```

### 5. Example Executor: Crawler

**File:** `internal/services/crawler/executor.go`

```go
package crawler

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/ternarybob/quaero/internal/jobs"
)

// Executor executes crawler jobs
type Executor struct {
    jobMgr  *jobs.Manager
    service *Service // Your existing crawler service
}

func NewExecutor(jobMgr *jobs.Manager, service *Service) *Executor {
    return &Executor{
        jobMgr:  jobMgr,
        service: service,
    }
}

// CrawlerPayload defines the crawler job payload
type CrawlerPayload struct {
    URL   string `json:"url"`
    Depth int    `json:"depth"`
}

// Execute implements the Executor interface
func (e *Executor) Execute(ctx context.Context, jobID string, payload []byte) error {
    var p CrawlerPayload
    if err := json.Unmarshal(payload, &p); err != nil {
        return fmt.Errorf("unmarshal payload: %w", err)
    }

    e.jobMgr.AddJobLog(ctx, jobID, "info", fmt.Sprintf("Starting crawler for URL: %s", p.URL))

    // Crawl the URL using your existing service
    doc, childURLs, err := e.service.Crawl(ctx, p.URL)
    if err != nil {
        return fmt.Errorf("crawl failed: %w", err)
    }

    e.jobMgr.AddJobLog(ctx, jobID, "info", fmt.Sprintf("Crawled URL, found %d child URLs", len(childURLs)))

    // Store the document
    if err := e.service.StoreDocument(ctx, doc); err != nil {
        return fmt.Errorf("store document: %w", err)
    }

    // Create child jobs for discovered URLs (if depth allows)
    if p.Depth > 0 {
        for _, childURL := range childURLs {
            childPayload := CrawlerPayload{
                URL:   childURL,
                Depth: p.Depth - 1,
            }

            childJobID, err := e.jobMgr.CreateChildJob(ctx, jobID, "crawler", "core", childPayload)
            if err != nil {
                e.jobMgr.AddJobLog(ctx, jobID, "warn", fmt.Sprintf("Failed to create child job for %s: %v", childURL, err))
                continue
            }

            e.jobMgr.AddJobLog(ctx, jobID, "info", fmt.Sprintf("Created child job %s for %s", childJobID, childURL))
        }
    }

    // Update progress
    e.jobMgr.UpdateJobProgress(ctx, jobID, 1, 1)

    return nil
}
```

### 6. API Handler

**File:** `internal/handlers/job_handler.go`

```go
package handlers

import (
    "encoding/json"
    "net/http"
    "strconv"

    "github.com/gorilla/mux"
    "github.com/ternarybob/quaero/internal/jobs"
)

type JobHandler struct {
    jobMgr *jobs.Manager
}

func NewJobHandler(jobMgr *jobs.Manager) *JobHandler {
    return &JobHandler{jobMgr: jobMgr}
}

// ListJobs handles GET /api/jobs
// Returns parent jobs only (no parent_id)
func (h *JobHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
    limit := 100
    offset := 0

    if l := r.URL.Query().Get("limit"); l != "" {
        if val, err := strconv.Atoi(l); err == nil {
            limit = val
        }
    }

    if o := r.URL.Query().Get("offset"); o != "" {
        if val, err := strconv.Atoi(o); err == nil {
            offset = val
        }
    }

    jobs, err := h.jobMgr.ListParentJobs(r.Context(), limit, offset)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(jobs)
}

// GetJobChildren handles GET /api/jobs/{id}/children
func (h *JobHandler) GetJobChildren(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    jobID := vars["id"]

    children, err := h.jobMgr.ListChildJobs(r.Context(), jobID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(children)
}

// GetJobLogs handles GET /api/jobs/{id}/logs
func (h *JobHandler) GetJobLogs(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    jobID := vars["id"]

    limit := 1000
    if l := r.URL.Query().Get("limit"); l != "" {
        if val, err := strconv.Atoi(l); err == nil {
            limit = val
        }
    }

    logs, err := h.jobMgr.GetJobLogs(r.Context(), jobID, limit)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(logs)
}

// GetJob handles GET /api/jobs/{id}
func (h *JobHandler) GetJob(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    jobID := vars["id"]

    job, err := h.jobMgr.GetJob(r.Context(), jobID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(job)
}

// CreateJob handles POST /api/jobs
func (h *JobHandler) CreateJob(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Type    string      `json:"type"`
        Payload interface{} `json:"payload"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    jobID, err := h.jobMgr.CreateParentJob(r.Context(), req.Type, req.Payload)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"job_id": jobID})
}
```

### 7. Router Registration

**File:** `internal/server/router.go` (update)

```go
// Add these routes to your router setup
func (s *Server) setupRoutes() {
    // ... existing routes ...

    // Job routes
    s.router.HandleFunc("/api/jobs", s.jobHandler.ListJobs).Methods("GET")
    s.router.HandleFunc("/api/jobs", s.jobHandler.CreateJob).Methods("POST")
    s.router.HandleFunc("/api/jobs/{id}", s.jobHandler.GetJob).Methods("GET")
    s.router.HandleFunc("/api/jobs/{id}/children", s.jobHandler.GetJobChildren).Methods("GET")
    s.router.HandleFunc("/api/jobs/{id}/logs", s.jobHandler.GetJobLogs).Methods("GET")

    // Job UI page
    s.router.HandleFunc("/jobs", s.uiHandler.JobsPage).Methods("GET")
}
```

---

## UI Implementation

### Jobs Page Template

**File:** `pages/jobs.html`

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Jobs - Quaero</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bulma@0.9.4/css/bulma.min.css">
    <script defer src="https://unpkg.com/alpinejs@3.x.x/dist/cdn.min.js"></script>
    <style>
        .log-entry {
            font-family: 'Courier New', monospace;
            font-size: 0.875rem;
            padding: 0.25rem 0;
        }
        .log-debug { color: #7f8c8d; }
        .log-info { color: #3498db; }
        .log-warn { color: #f39c12; }
        .log-error { color: #e74c3c; }
    </style>
</head>
<body>
    <section class="section" x-data="jobsApp()">
        <div class="container">
            <!-- Header -->
            <div class="level">
                <div class="level-left">
                    <div class="level-item">
                        <h1 class="title">Jobs</h1>
                    </div>
                </div>
                <div class="level-right">
                    <div class="level-item">
                        <button class="button is-primary" @click="refresh()">
                            Refresh
                        </button>
                    </div>
                </div>
            </div>

            <!-- Loading State -->
            <div x-show="loading" class="has-text-centered">
                <progress class="progress is-small is-primary" max="100">Loading</progress>
            </div>

            <!-- Parent Jobs Table -->
            <div x-show="!loading">
                <table class="table is-fullwidth is-striped is-hoverable">
                    <thead>
                        <tr>
                            <th>Job ID</th>
                            <th>Type</th>
                            <th>Status</th>
                            <th>Progress</th>
                            <th>Created</th>
                            <th>Duration</th>
                            <th>Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        <template x-for="job in jobs" :key="job.id">
                            <tr>
                                <td>
                                    <code x-text="job.id.substring(0, 8) + '...'"></code>
                                </td>
                                <td>
                                    <span class="tag is-light" x-text="job.job_type"></span>
                                </td>
                                <td>
                                    <span class="tag" 
                                          :class="{
                                              'is-info': job.status === 'pending',
                                              'is-warning': job.status === 'running',
                                              'is-success': job.status === 'completed',
                                              'is-danger': job.status === 'failed'
                                          }"
                                          x-text="job.status"></span>
                                </td>
                                <td>
                                    <template x-if="job.progress_total > 0">
                                        <div>
                                            <progress class="progress is-small" 
                                                      :class="{
                                                          'is-info': job.status === 'pending',
                                                          'is-warning': job.status === 'running',
                                                          'is-success': job.status === 'completed',
                                                          'is-danger': job.status === 'failed'
                                                      }"
                                                      :value="job.progress_current" 
                                                      :max="job.progress_total"></progress>
                                            <span class="is-size-7" 
                                                  x-text="`${job.progress_current} / ${job.progress_total}`"></span>
                                        </div>
                                    </template>
                                    <template x-if="job.progress_total === 0">
                                        <span class="has-text-grey-light">—</span>
                                    </template>
                                </td>
                                <td>
                                    <span class="is-size-7" x-text="formatDate(job.created_at)"></span>
                                </td>
                                <td>
                                    <span class="is-size-7" x-text="getDuration(job)"></span>
                                </td>
                                <td>
                                    <button class="button is-small is-info" 
                                            @click="viewDetails(job.id)">
                                        View Details
                                    </button>
                                </td>
                            </tr>
                        </template>
                    </tbody>
                </table>

                <!-- Empty State -->
                <div x-show="jobs.length === 0" class="has-text-centered has-text-grey-light">
                    <p class="is-size-4">No jobs found</p>
                    <p class="is-size-6">Jobs will appear here once created</p>
                </div>
            </div>

            <!-- Job Details Modal -->
            <div class="modal" :class="{'is-active': selectedJobId}">
                <div class="modal-background" @click="closeDetails()"></div>
                <div class="modal-card" style="width: 90%; max-width: 1200px;">
                    <header class="modal-card-head">
                        <p class="modal-card-title">
                            Job Details: <code x-text="selectedJobId"></code>
                        </p>
                        <button class="delete" @click="closeDetails()"></button>
                    </header>
                    <section class="modal-card-body">
                        <!-- Child Jobs -->
                        <div class="mb-5">
                            <h2 class="subtitle">Child Jobs</h2>
                            <template x-if="childJobs.length > 0">
                                <table class="table is-fullwidth is-striped">
                                    <thead>
                                        <tr>
                                            <th>Job ID</th>
                                            <th>Type</th>
                                            <th>Phase</th>
                                            <th>Status</th>
                                            <th>Progress</th>
                                            <th>Created</th>
                                        </tr>
                                    </thead>
                                    <tbody>
                                        <template x-for="child in childJobs" :key="child.id">
                                            <tr>
                                                <td>
                                                    <code x-text="child.id.substring(0, 8) + '...'"></code>
                                                </td>
                                                <td>
                                                    <span class="tag is-light" x-text="child.job_type"></span>
                                                </td>
                                                <td>
                                                    <span class="tag" 
                                                          :class="{
                                                              'is-info': child.phase === 'pre',
                                                              'is-primary': child.phase === 'core',
                                                              'is-success': child.phase === 'post'
                                                          }"
                                                          x-text="child.phase"></span>
                                                </td>
                                                <td>
                                                    <span class="tag" 
                                                          :class="{
                                                              'is-info': child.status === 'pending',
                                                              'is-warning': child.status === 'running',
                                                              'is-success': child.status === 'completed',
                                                              'is-danger': child.status === 'failed'
                                                          }"
                                                          x-text="child.status"></span>
                                                </td>
                                                <td>
                                                    <template x-if="child.progress_total > 0">
                                                        <span class="is-size-7" 
                                                              x-text="`${child.progress_current} / ${child.progress_total}`"></span>
                                                    </template>
                                                    <template x-if="child.progress_total === 0">
                                                        <span class="has-text-grey-light">—</span>
                                                    </template>
                                                </td>
                                                <td>
                                                    <span class="is-size-7" x-text="formatDate(child.created_at)"></span>
                                                </td>
                                            </tr>
                                        </template>
                                    </tbody>
                                </table>
                            </template>
                            <template x-if="childJobs.length === 0">
                                <p class="has-text-grey-light">No child jobs</p>
                            </template>
                        </div>

                        <!-- Job Logs -->
                        <div>
                            <h2 class="subtitle">Logs</h2>
                            <div class="box" style="max-height: 400px; overflow-y: auto; background-color: #1e1e1e; color: #d4d4d4;">
                                <template x-if="logs.length > 0">
                                    <div>
                                        <template x-for="log in logs" :key="log.id">
                                            <div class="log-entry" :class="`log-${log.level}`">
                                                <span x-text="formatTime(log.timestamp)"></span>
                                                <span x-text="`[${log.level.toUpperCase()}]`"></span>
                                                <span x-text="log.message"></span>
                                            </div>
                                        </template>
                                    </div>
                                </template>
                                <template x-if="logs.length === 0">
                                    <p class="has-text-grey-light">No logs available</p>
                                </template>
                            </div>
                        </div>
                    </section>
                </div>
            </div>
        </div>
    </section>

    <script>
        function jobsApp() {
            return {
                jobs: [],
                childJobs: [],
                logs: [],
                selectedJobId: null,
                loading: true,
                refreshInterval: null,

                init() {
                    this.loadJobs();
                    // Auto-refresh every 3 seconds
                    this.refreshInterval = setInterval(() => {
                        this.loadJobs();
                        if (this.selectedJobId) {
                            this.loadDetails(this.selectedJobId);
                        }
                    }, 3000);
                },

                async loadJobs() {
                    try {
                        const response = await fetch('/api/jobs');
                        if (!response.ok) throw new Error('Failed to load jobs');
                        this.jobs = await response.json();
                    } catch (error) {
                        console.error('Error loading jobs:', error);
                    } finally {
                        this.loading = false;
                    }
                },

                async viewDetails(jobId) {
                    this.selectedJobId = jobId;
                    await this.loadDetails(jobId);
                },

                async loadDetails(jobId) {
                    try {
                        // Load child jobs
                        const childResponse = await fetch(`/api/jobs/${jobId}/children`);
                        if (childResponse.ok) {
                            this.childJobs = await childResponse.json();
                        }

                        // Load logs
                        const logsResponse = await fetch(`/api/jobs/${jobId}/logs`);
                        if (logsResponse.ok) {
                            this.logs = await logsResponse.json();
                            this.logs.reverse(); // Show oldest first
                        }
                    } catch (error) {
                        console.error('Error loading job details:', error);
                    }
                },

                closeDetails() {
                    this.selectedJobId = null;
                    this.childJobs = [];
                    this.logs = [];
                },

                refresh() {
                    this.loading = true;
                    this.loadJobs();
                },

                formatDate(dateString) {
                    const date = new Date(dateString);
                    return date.toLocaleString();
                },

                formatTime(dateString) {
                    const date = new Date(dateString);
                    return date.toLocaleTimeString();
                },

                getDuration(job) {
                    if (!job.started_at) return '—';
                    
                    const start = new Date(job.started_at);
                    const end = job.completed_at ? new Date(job.completed_at) : new Date();
                    const duration = Math.floor((end - start) / 1000); // seconds

                    if (duration < 60) return `${duration}s`;
                    if (duration < 3600) return `${Math.floor(duration / 60)}m ${duration % 60}s`;
                    return `${Math.floor(duration / 3600)}h ${Math.floor((duration % 3600) / 60)}m`;
                },

                destroy() {
                    if (this.refreshInterval) {
                        clearInterval(this.refreshInterval);
                    }
                }
            };
        }
    </script>
</body>
</html>
```

### UI Handler

**File:** `internal/handlers/ui.go` (add method)

```go
// JobsPage renders the jobs page
func (h *UIHandler) JobsPage(w http.ResponseWriter, r *http.Request) {
    h.renderTemplate(w, "jobs.html", nil)
}
```

---

## Migration Steps

### Step 1: Backup Current System

```bash
# Backup database
cp quaero.db quaero.db.backup.$(date +%Y%m%d_%H%M%S)

# Backup code
git commit -am "Backup before queue redesign"
git tag backup-before-queue-redesign-v4
```

### Step 2: Create New Files

```bash
# Create new directories if needed
mkdir -p internal/queue
mkdir -p docs/migrations

# Create migration file
touch docs/migrations/008_redesign_job_queue.sql

# Create implementation files
touch internal/queue/types.go
touch internal/queue/manager.go
touch internal/queue/worker.go
touch internal/jobs/manager.go
touch internal/services/crawler/executor.go
touch internal/handlers/job_handler.go
touch pages/jobs.html
```

### Step 3: Run Database Migration

```sql
-- Run this in SQLite
.read docs/migrations/008_redesign_job_queue.sql
```

### Step 4: Implement Files in Order

1. **Queue Types** (`internal/queue/types.go`)
2. **Queue Manager** (`internal/queue/manager.go`)
3. **Job Manager** (`internal/jobs/manager.go`)
4. **Worker Pool** (`internal/queue/worker.go`)
5. **Executors** (start with crawler)
6. **API Handler** (`internal/handlers/job_handler.go`)
7. **UI Template** (`pages/jobs.html`)
8. **Router Updates** (`internal/server/router.go`)

### Step 5: Update Application Initialization

**File:** `internal/app/app.go` (update)

```go
func (a *App) Initialize() error {
    // ... existing initialization ...

    // Create queue manager
    queueMgr, err := queue.NewManager(a.db, "quaero_jobs")
    if err != nil {
        return fmt.Errorf("create queue manager: %w", err)
    }
    a.queueMgr = queueMgr

    // Create job manager
    jobMgr := jobs.NewManager(a.db, queueMgr)
    a.jobMgr = jobMgr

    // Create worker pool
    workerPool := queue.NewWorkerPool(queueMgr, jobMgr, 5) // 5 workers

    // Register executors
    crawlerExecutor := crawler.NewExecutor(jobMgr, a.crawlerService)
    workerPool.RegisterExecutor("crawler", crawlerExecutor)

    // Add more executors as needed
    // summarizerExecutor := summarizer.NewExecutor(jobMgr, a.summarizerService)
    // workerPool.RegisterExecutor("summarizer", summarizerExecutor)

    a.workerPool = workerPool

    // Start worker pool
    go workerPool.Start()

    // ... rest of initialization ...
}
```

### Step 6: Remove Old Code

**Files to Delete:**
- Any custom queue implementation files
- Old job_queue tables/code
- Chevron UI components

**Code to Remove:**
```bash
# Search for old queue code
grep -r "queue_messages" internal/
grep -r "queue_state" internal/
grep -r "chevron" pages/

# Remove after verification
```

### Step 7: Update Tests

Create new tests for:
- Queue manager enqueue/receive
- Job manager CRUD operations
- Worker pool processing
- API endpoints

### Step 8: Configuration

**File:** `quaero.toml` (add section)

```toml
[queue]
num_workers = 5
```

### Step 9: Build and Test

```powershell
# Build
.\scripts\build.ps1 -Clean

# Run tests
cd test
go test -v ./...

# Start application
.\scripts\build.ps1 -Run
```

### Step 10: Smoke Test

```bash
# Check queue table exists
sqlite3 quaero.db "SELECT name FROM sqlite_master WHERE type='table' AND name='goqite_jobs';"

# Check jobs table
sqlite3 quaero.db "SELECT COUNT(*) FROM jobs;"

# Test API
curl http://localhost:8085/api/jobs

# Open UI
# Navigate to http://localhost:8085/jobs
```

---

## Testing & Validation

### Unit Tests

**File:** `internal/queue/manager_test.go`

```go
package queue_test

import (
    "context"
    "database/sql"
    "testing"

    _ "github.com/mattn/go-sqlite3"
    "github.com/ternarybob/quaero/internal/queue"
)

func TestQueueManager(t *testing.T) {
    // Setup in-memory database
    db, err := sql.Open("sqlite3", ":memory:")
    if err != nil {
        t.Fatal(err)
    }
    defer db.Close()

    // Create queue manager
    qm, err := queue.NewManager(db, "test_queue")
    if err != nil {
        t.Fatal(err)
    }

    ctx := context.Background()

    // Test enqueue
    msg := queue.Message{
        JobID:   "test-job-1",
        Type:    "test",
        Payload: []byte(`{"test": true}`),
    }

    if err := qm.Enqueue(ctx, msg); err != nil {
        t.Errorf("Enqueue failed: %v", err)
    }

    // Test receive
    receivedMsg, deleteFn, err := qm.Receive(ctx)
    if err != nil {
        t.Errorf("Receive failed: %v", err)
    }

    if receivedMsg.JobID != msg.JobID {
        t.Errorf("Expected JobID %s, got %s", msg.JobID, receivedMsg.JobID)
    }

    // Test delete
    if err := deleteFn(); err != nil {
        t.Errorf("Delete failed: %v", err)
    }
}
```

### Integration Tests

**File:** `test/jobs_test.go`

```go
package test

import (
    "context"
    "net/http"
    "testing"
    "time"
)

func TestJobsAPI(t *testing.T) {
    // Start test server
    server := startTestServer(t)
    defer server.Close()

    ctx := context.Background()

    // Create a test job
    jobID, err := server.JobManager.CreateParentJob(ctx, "test", map[string]string{
        "test": "data",
    })
    if err != nil {
        t.Fatalf("Failed to create job: %v", err)
    }

    // Wait for processing
    time.Sleep(2 * time.Second)

    // Test GET /api/jobs
    resp, err := http.Get(server.URL + "/api/jobs")
    if err != nil {
        t.Fatalf("Failed to get jobs: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        t.Errorf("Expected status 200, got %d", resp.StatusCode)
    }

    // Test GET /api/jobs/{id}
    resp, err = http.Get(server.URL + "/api/jobs/" + jobID)
    if err != nil {
        t.Fatalf("Failed to get job: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        t.Errorf("Expected status 200, got %d", resp.StatusCode)
    }
}
```

### Manual Testing Checklist

```
[ ] Database migration runs successfully
[ ] goqite table created automatically
[ ] Jobs table created with correct schema
[ ] Job logs table created with correct schema
[ ] Queue manager can enqueue messages
[ ] Queue manager can receive messages
[ ] Worker pool starts with configured workers
[ ] Executors are registered correctly
[ ] Parent jobs created successfully
[ ] Child jobs created successfully
[ ] Job status updates correctly (pending → running → completed)
[ ] Job logs are recorded
[ ] Progress tracking works
[ ] UI loads at /jobs
[ ] UI shows parent jobs in table
[ ] Clicking "View Details" opens modal
[ ] Modal shows child jobs
[ ] Modal shows logs
[ ] Auto-refresh works (3 second interval)
[ ] No test URLs in production code
[ ] No chevrons in UI
```

---

## Rollback Plan

### If Migration Fails

```bash
# Restore database backup
cp quaero.db.backup.YYYYMMDD_HHMMSS quaero.db

# Revert code
git reset --hard backup-before-queue-redesign-v4

# Rebuild
.\scripts\build.ps1 -Clean
.\scripts\build.ps1 -Run
```

### If Runtime Issues Occur

1. **Stop the application**
2. **Check logs** for specific errors
3. **Verify database state**:
   ```sql
   SELECT * FROM jobs LIMIT 10;
   SELECT * FROM job_logs LIMIT 10;
   SELECT COUNT(*) FROM goqite_jobs;
   ```
4. **If queue is stuck**:
   ```sql
   -- Clear queue (careful!)
   DELETE FROM goqite_jobs;
   ```
5. **Restart with single worker** for debugging:
   ```toml
   [queue]
   num_workers = 1
   ```

---

## Success Metrics

After implementation, verify:

1. ✅ **Queue is managed solely by goqite**
   - No custom queue tables
   - No custom queue logic
   - All queue operations through goqite

2. ✅ **Job hierarchy works**
   - Parent jobs visible in UI
   - Child jobs accessible via parent
   - Pre/core/post phases distinct

3. ✅ **UI is simple**
   - No chevrons
   - Simple table for parents
   - Modal for details
   - Clean, readable design

4. ✅ **Jobs execute successfully**
   - Jobs move through states correctly
   - Progress tracking accurate
   - Logs captured properly
   - Errors handled gracefully

5. ✅ **No environment pollution**
   - No test URLs in production code
   - Configuration-driven
   - Clean separation of concerns

---

## Appendix: Common Issues and Solutions

### Issue: "goqite table not found"

**Solution:** Queue manager wasn't initialized. Check `Setup()` was called.

### Issue: "No executor registered for job type"

**Solution:** Register executor in app initialization:
```go
workerPool.RegisterExecutor("your_type", yourExecutor)
```

### Issue: Jobs stuck in "running" state

**Solution:** Worker crashed without updating status. Add recovery:
```go
defer func() {
    if r := recover(); r != nil {
        jobMgr.SetJobError(ctx, jobID, fmt.Sprintf("panic: %v", r))
    }
}()
```

### Issue: UI not showing child jobs

**Solution:** Check parent_id is set correctly in database:
```sql
SELECT id, parent_id, job_type FROM jobs WHERE parent_id = 'your-parent-id';
```

### Issue: Logs not appearing

**Solution:** Verify AddJobLog is called:
```go
jobMgr.AddJobLog(ctx, jobID, "info", "Your message")
```

---

## Next Steps After Implementation

1. **Monitor Production**
   - Watch for errors in logs
   - Monitor queue depth
   - Track job completion rates

2. **Add More Executors**
   - Summarizer
   - Cleanup
   - Custom job types

3. **Enhance UI**
   - Add filtering by status
   - Add search by job ID
   - Add job cancellation

4. **Performance Tuning**
   - Adjust worker count
   - Add queue metrics
   - Optimize database queries

5. **Documentation**
   - Update README
   - Create operator guide
   - Document troubleshooting

---

**END OF IMPLEMENTATION GUIDE**